package qwacamole

import (
	"context"
	"crypto/x509"
	"fmt"
	"github.com/cztamas/qwacamole/lifecycle"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
	"go.uber.org/zap"
)

// One state owner spans overlapping Caddy configurations. Provisioning creates
// leases and validates candidates, but never starts workers or installs them.
var registry = struct {
	sync.Mutex
	entries    map[string]*owner
	generation uint64
}{entries: map[string]*owner{}}

type owner struct {
	// mu serializes this deployment only. registry protects references/map entries;
	// no signing, storage, worker shutdown or owner-lock wait holds registry.
	mu                                    sync.Mutex
	initErr                               error
	closing                               bool
	closed                                chan struct{}
	cfg                                   BindingConfig
	manager                               *store.Manager
	lock                                  *os.File
	sessions                              map[context.Context]*session
	refs                                  int
	customSign, customCommit, customWrite bool
	active                                uint64
	cancel                                context.CancelFunc
	done                                  chan struct{}
}
type session struct {
	cfg        BindingConfig
	material   *credentials.Credentials
	generation uint64
	refs       int
	storedTLS  *x509.Certificate
}

// Lease must be released at module cleanup, including rejected provisioning.
type Lease struct {
	owner   *owner
	session *session
	ctx     context.Context
	once    sync.Once
	logger  *zap.Logger
}

// Acquire loads material again for each Caddy configuration, including unchanged
// force reloads. Modules share the directory owner; Caddy may give the issuer
// and middleware different module contexts and therefore different sessions.
func Acquire(ctx caddy.Context, cfg BindingConfig, ops store.Operations) (*Lease, error) {
	cfg, e := cfg.normalized()
	if e != nil {
		return nil, e
	}
	mat, e := cfg.material()
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(cfg.Dir, 0700); e != nil {
		return nil, e
	}
	actual, e := filepath.EvalSymlinks(cfg.Dir)
	if e != nil {
		return nil, e
	}
	cfg.Dir = actual
	var o *owner
	var created bool
	for {
		registry.Lock()
		o = registry.entries[cfg.Dir]
		if o != nil && o.closing {
			done := o.closed
			registry.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		created = o == nil
		if created {
			o = &owner{cfg: cfg, sessions: map[context.Context]*session{}, closed: make(chan struct{}), customSign: ops.Sign != nil, customCommit: ops.CommitManifest != nil, customWrite: ops.WriteDocument != nil}
			o.mu.Lock() // reserve initialization before exposing the owner
			registry.entries[cfg.Dir] = o
		}
		o.refs++
		registry.Unlock()
		break
	}
	if !created {
		o.mu.Lock()
	} else {
		o.lock, e = lockState(filepath.Join(cfg.Dir, ".qwac-owner.lock"))
		if e == nil {
			o.manager, e = store.Recover(cfg.Config, mat, ops)
		}
		o.initErr = e
	}
	fail := func(err error) (*Lease, error) { o.mu.Unlock(); o.releaseReference(); return nil, err }
	if o.initErr != nil {
		return fail(fmt.Errorf("recover binding state: %w", o.initErr))
	}
	if !reflect.DeepEqual(o.cfg.Config, cfg.Config) || o.customSign != (ops.Sign != nil) || o.customCommit != (ops.CommitManifest != nil) || o.customWrite != (ops.WriteDocument != nil) {
		return fail(fmt.Errorf("conflicting binding state ownership/settings for %s", cfg.Dir))
	}
	s := o.sessions[ctx.Context]
	if s != nil {
		if !reflect.DeepEqual(s.cfg, cfg) || binding.ID(s.material.Chain[0]) != binding.ID(mat.Chain[0]) {
			return fail(fmt.Errorf("issuer/handler binding configuration or credentials mismatch"))
		}
	} else {
		if e = o.manager.PrepareReplacement(mat); e != nil {
			return fail(fmt.Errorf("prepare QWAC replacement: %w", e))
		}
		registry.Lock()
		registry.generation++
		generation := registry.generation
		registry.Unlock()
		s = &session{cfg: cfg, material: mat, generation: generation}
		o.sessions[ctx.Context] = s
	}
	s.refs++
	o.mu.Unlock()
	return &Lease{owner: o, session: s, ctx: ctx.Context, logger: ctx.Logger()}, nil
}

func (l *Lease) Manager() *store.Manager { return l.owner.manager }

// borrow keeps state ownership and refresh alive for an in-flight request or
// issuance even when Caddy cleans up the originating module during a reload.
func (l *Lease) borrow() (func(), error) {
	registry.Lock()
	defer registry.Unlock()
	if l.ctx.Err() != nil || l.owner.closing {
		return nil, fmt.Errorf("QWAC configuration has been unloaded")
	}
	l.owner.refs++
	var once sync.Once
	return func() {
		once.Do(func() { l.owner.releaseReference() })
	}, nil
}
func (l *Lease) SetStoredTLS(c *x509.Certificate) {
	l.owner.mu.Lock()
	defer l.owner.mu.Unlock()
	l.session.storedTLS = c
}

// Activate is called by live HTTP handling/issuance, after provisioning. Older
// configs continue using the shared current binding and cannot undo a newer leaf.
func (l *Lease) Activate() error {
	return l.activate(nil)
}

// ActivateForServing retains the previously activated credential after a
// replacement publication failure, only while its binding still covers every
// stored TLS leaf in the live configurations. Issuance never uses this fallback:
// Publish must commit new TLS coverage before returning a certificate.
func (l *Lease) ActivateForServing() error {
	if e := l.Activate(); e != nil {
		l.owner.mu.Lock()
		defer l.owner.mu.Unlock()
		o := l.owner
		state := o.manager.Snapshot()
		if l.ctx.Err() != nil || o.active == 0 || state.URL == "" || state.Expires <= time.Now().Unix() {
			return e
		}
		covered := map[string]bool{}
		for _, id := range state.IDs {
			covered[id] = true
		}
		for _, s := range o.sessions {
			if s.storedTLS != nil && !covered[binding.ID(s.storedTLS)] {
				return e
			}
		}
		if _, err := o.manager.CurrentDocument(); err != nil {
			return e
		}
		return nil
	}
	return nil
}

// Publish commits the new TLS leaf together with any pending configuration's
// credentials and stored TLS leaf before the ACME issuer may return it.
func (l *Lease) Publish(c *x509.Certificate) error {
	return l.activate(c)
}

func (l *Lease) activate(issued *x509.Certificate) error {
	l.owner.mu.Lock()
	defer l.owner.mu.Unlock()
	if l.ctx.Err() != nil {
		return fmt.Errorf("QWAC configuration has been unloaded")
	}
	o := l.owner
	if l.session.generation > o.active {
		// SetConfig discovers stored TLS on the issuer's lease. A handler's
		// context can differ, so activation must include every live session's
		// stored leaf, not just the activating module's. Overlapping/rejected
		// candidates can only add coverage, never roll back the chosen signer.
		stored := []*x509.Certificate{issued}
		for _, session := range o.sessions {
			stored = append(stored, session.storedTLS)
		}
		if e := o.manager.Activate(l.session.material, stored...); e != nil {
			l.logger.Error("QWAC replacement publication failed", zap.String("state", o.cfg.Dir), zap.Error(e))
			l.logStatus("QWAC replacement remains pending")
			return e
		}
		o.active = l.session.generation
		l.logger.Info("QWAC binding configuration activated", zap.String("hostname", o.cfg.Host), zap.String("signer", binding.ID(l.session.material.Chain[0])))
		l.logStatus("QWAC binding operational state")
	} else if issued != nil {
		if e := o.manager.Publish(issued); e != nil {
			return e
		}
	}
	if o.cancel == nil {
		child, cancel := context.WithCancel(context.Background())
		o.cancel = cancel
		o.done = make(chan struct{})
		go l.runWorker(child)
	}
	return nil
}

// The owner has one expiry-driven worker across overlapping configurations.
// Publication wakes it immediately; retry jitter is operational only and never
// extends signed validity or permits an expired document to be advertised.
func (l *Lease) runWorker(ctx context.Context) {
	o := l.owner
	defer close(o.done)
	lifecycle.Run(ctx, o.manager, func(e lifecycle.Event) {
		fields := []zap.Field{zap.String("state", o.cfg.Dir), zap.Any("binding", e.Status)}
		if e.Err != nil {
			l.logger.Error("QWAC maintenance "+e.Kind, append(fields, zap.Error(e.Err), zap.Duration("retry_after", e.RetryAfter))...)
		} else {
			l.logger.Info("QWAC maintenance "+e.Kind, append(fields, zap.Any("collection", e.Collection))...)
		}
	})
}

func retryDelay(failures int) time.Duration { return lifecycle.RetryDelay(failures) }

func (l *Lease) logStatus(message string) {
	s, e := l.owner.manager.Status()
	if e != nil {
		l.logger.Error("QWAC storage diagnostics failed", zap.Error(e))
		return
	}
	s.CredentialID = binding.ID(l.session.material.Chain[0])
	s.ReplacementPending = l.session.generation > l.owner.active
	l.logger.Info(message, zap.String("hostname", l.owner.cfg.Host), zap.Any("binding", s))
}
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		o := l.owner
		o.mu.Lock()
		l.session.refs--
		if l.session.refs == 0 {
			delete(o.sessions, l.ctx)
		}
		o.mu.Unlock()
		o.releaseReference()
	})
}

// Last-reference cleanup keeps a closing tombstone in registry. A new owner
// waits outside the global lock until the worker and process lock are released.
func (o *owner) releaseReference() {
	registry.Lock()
	o.refs--
	if o.refs != 0 {
		registry.Unlock()
		return
	}
	o.closing = true
	if o.closed == nil {
		o.closed = make(chan struct{})
	}
	registry.Unlock()
	o.mu.Lock()
	if o.cancel != nil {
		o.cancel()
		<-o.done
	}
	if o.lock != nil {
		o.lock.Close()
	}
	o.mu.Unlock()
	registry.Lock()
	if registry.entries[o.cfg.Dir] == o {
		delete(registry.entries, o.cfg.Dir)
	}
	close(o.closed)
	registry.Unlock()
}
