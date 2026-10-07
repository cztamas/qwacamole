package qwacamole

import (
	"context"
	"crypto/x509"
	"fmt"
	"math/rand/v2"
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
// force reloads. Issuer, middleware and development diagnostics share one lease
// session, ensuring consistent paths, material and operations within that config.
func Acquire(ctx caddy.Context, cfg BindingConfig, ops store.Operations) (*Lease, error) {
	cfg, e := cfg.normalized()
	if e != nil {
		return nil, e
	}
	mat, e := cfg.material()
	if e != nil {
		return nil, e
	}
	registry.Lock()
	defer registry.Unlock()
	o := registry.entries[cfg.Dir]
	if o == nil {
		if e = os.MkdirAll(cfg.Dir, 0700); e != nil {
			return nil, e
		}
		// Resolve symlinks so aliases cannot acquire independent in-process owners.
		actual, e := filepath.EvalSymlinks(cfg.Dir)
		if e != nil {
			return nil, e
		}
		cfg.Dir = actual
		o = registry.entries[actual]
	}
	if o == nil {
		lock, e := lockState(filepath.Join(cfg.Dir, ".qwac-owner.lock"))
		if e != nil {
			return nil, e
		}
		m, e := store.Recover(cfg.Config, mat, ops)
		if e != nil {
			lock.Close()
			return nil, fmt.Errorf("recover binding state: %w", e)
		}
		o = &owner{cfg: cfg, manager: m, lock: lock, sessions: map[context.Context]*session{}, customSign: ops.Sign != nil, customCommit: ops.CommitManifest != nil, customWrite: ops.WriteDocument != nil}
		registry.entries[cfg.Dir] = o
	}
	if !reflect.DeepEqual(o.cfg.Config, cfg.Config) || o.customSign != (ops.Sign != nil) || o.customCommit != (ops.CommitManifest != nil) || o.customWrite != (ops.WriteDocument != nil) {
		return nil, fmt.Errorf("conflicting binding state ownership/settings for %s", cfg.Dir)
	}
	s := o.sessions[ctx.Context]
	if s != nil {
		if !reflect.DeepEqual(s.cfg, cfg) || binding.ID(s.material.Chain[0]) != binding.ID(mat.Chain[0]) {
			return nil, fmt.Errorf("issuer/handler binding configuration or credentials mismatch")
		}
	} else {
		if e = o.manager.PrepareReplacement(mat); e != nil {
			if o.refs == 0 {
				delete(registry.entries, cfg.Dir)
				o.lock.Close()
			}
			return nil, fmt.Errorf("prepare QWAC replacement: %w", e)
		}
		registry.generation++
		s = &session{cfg: cfg, material: mat, generation: registry.generation}
		o.sessions[ctx.Context] = s
	}
	s.refs++
	o.refs++
	return &Lease{owner: o, session: s, ctx: ctx.Context, logger: ctx.Logger()}, nil
}
func (l *Lease) Manager() *store.Manager { return l.owner.manager }

// borrow keeps state ownership and refresh alive for an in-flight request or
// issuance even when Caddy cleans up the originating module during a reload.
func (l *Lease) borrow() (func(), error) {
	registry.Lock()
	defer registry.Unlock()
	if l.ctx.Err() != nil {
		return nil, fmt.Errorf("QWAC configuration has been unloaded")
	}
	l.owner.refs++
	var once sync.Once
	return func() {
		once.Do(func() { registry.Lock(); defer registry.Unlock(); l.owner.refs--; l.owner.closeIfUnused() })
	}, nil
}
func (l *Lease) SetStoredTLS(c *x509.Certificate) {
	registry.Lock()
	defer registry.Unlock()
	l.session.storedTLS = c
}

// Activate is called by live HTTP handling/issuance, after provisioning. Older
// configs continue using the shared current binding and cannot undo a newer leaf.
func (l *Lease) Activate() error {
	return l.activate(nil)
}

// Publish commits the new TLS leaf together with any pending configuration's
// credentials and stored TLS leaf before the ACME issuer may return it.
func (l *Lease) Publish(c *x509.Certificate) error {
	return l.activate(c)
}

func (l *Lease) activate(issued *x509.Certificate) error {
	registry.Lock()
	defer registry.Unlock()
	if l.ctx.Err() != nil {
		return fmt.Errorf("QWAC configuration has been unloaded")
	}
	o := l.owner
	if l.session.generation > o.active {
		if e := o.manager.Activate(l.session.material, l.session.storedTLS, issued); e != nil {
			l.logger.Error("QWAC replacement publication failed", zap.String("state", o.cfg.Dir), zap.Error(e))
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
	gc := time.NewTicker(o.manager.CollectionInterval())
	defer gc.Stop()
	var retryAt time.Time
	failures := 0
	for {
		due := o.manager.NextRefresh()
		if retryAt.After(due) {
			due = retryAt
		}
		delay := time.Until(due)
		if delay < time.Millisecond {
			delay = time.Millisecond
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-o.manager.Changed():
			t.Stop()
			retryAt = time.Time{}
		case now := <-gc.C:
			t.Stop()
			if result, e := o.manager.Collect(now); e != nil {
				l.logger.Error("QWAC document collection failed", zap.String("state", o.cfg.Dir), zap.Error(e))
			} else if result.Documents > 0 {
				l.logger.Info("QWAC historical documents collected", zap.Int("documents", result.Documents), zap.Int64("bytes", result.Bytes))
			}
			l.logStatus("QWAC binding operational state")
		case <-t.C:
			if e := o.manager.Refresh(); e != nil {
				failures++
				delay := retryDelay(failures)
				// Before expiry, retry within half of the remaining validity where
				// possible. After expiry, bounded backoff avoids a busy failure loop.
				remaining := time.Until(time.Unix(o.manager.Snapshot().Expires, 0))
				if remaining > 2*time.Second && delay > remaining/2 {
					delay = remaining / 2
				}
				retryAt = time.Now().Add(delay)
				l.logger.Error("QWAC refresh failed; protected responses fail closed when current binding expires", zap.String("state", o.cfg.Dir), zap.Duration("retry_after", delay), zap.Error(e))
				l.logStatus("QWAC refresh failure state")
			} else {
				if failures > 0 {
					l.logger.Info("QWAC refresh recovered", zap.String("state", o.cfg.Dir))
				}
				failures = 0
				retryAt = time.Time{}
			}
		}
	}
}

func retryDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > 7 {
		failures = 7
	}
	base := time.Second * time.Duration(1<<(failures-1))
	if base > 48*time.Second {
		base = 48 * time.Second
	}
	return time.Duration(float64(base) * (1 + rand.Float64()/4))
}

func (l *Lease) logStatus(message string) {
	s, e := l.owner.manager.Status()
	if e != nil {
		l.logger.Error("QWAC storage diagnostics failed", zap.Error(e))
		return
	}
	l.logger.Info(message, zap.String("hostname", l.owner.cfg.Host), zap.Any("binding", s))
}
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		registry.Lock()
		defer registry.Unlock()
		o := l.owner
		o.refs--
		l.session.refs--
		if l.session.refs == 0 {
			delete(o.sessions, l.ctx)
		}
		if o.refs == 0 {
			o.closeIfUnused()
		}
	})
}

// registry lock is held by the caller.
func (o *owner) closeIfUnused() {
	if o.refs != 0 {
		return
	}
	if o.cancel != nil {
		o.cancel()
		<-o.done
	}
	o.lock.Close()
	delete(registry.entries, o.cfg.Dir)
}
