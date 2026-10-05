// Package qwacamole provides Caddy's native ACME issuer gate and HTTP handler.
package qwacamole

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddytls"
	"github.com/caddyserver/certmagic"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

func init() { caddy.RegisterModule(Issuer{}); caddy.RegisterModule(Handler{}) }

type Issuer struct {
	caddytls.ACMEIssuer `json:"acme"`
	Binding             BindingConfig `json:"binding"`
	manager             *store.Manager
	config              *certmagic.Config
	startupErr          error
}

func (Issuer) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "tls.issuance.qwac", New: func() caddy.Module { return new(Issuer) }}
}
func (i *Issuer) Provision(ctx caddy.Context) error {
	return i.ProvisionWithOperations(ctx, store.Operations{})
}

// ProvisionWithOperations supplies narrowly scoped lifecycle operations for development adapters.
func (i *Issuer) ProvisionWithOperations(ctx caddy.Context, ops store.Operations) error {
	if e := i.ACMEIssuer.Provision(ctx); e != nil {
		return e
	}
	m, e := Shared(i.Binding, ops)
	i.manager = m
	return e
}
func (i *Issuer) SetConfig(cfg *certmagic.Config) {
	i.config = cfg
	if len(cfg.Issuers) != 1 {
		i.startupErr = fmt.Errorf("QWAC issuer forbids fallback issuers")
		// Fail closed even if CertMagic tries the next configured issuer.
		cfg.Issuers = []certmagic.Issuer{i}
	}
	i.ACMEIssuer.SetConfig(cfg)
	// Recover even when TLS assets predate binding installation. SetConfig runs
	// before HTTP app startup and certificate management. Errors fail validation.
	d, e := cfg.Storage.Load(context.Background(), certmagic.StorageKeys.SiteCert(i.IssuerKey(), i.Binding.Host))
	if e == nil {
		b, _ := pem.Decode(d)
		if b == nil {
			i.startupErr = fmt.Errorf("stored TLS is not PEM")
			return
		}
		c, e := x509.ParseCertificate(b.Bytes)
		if e == nil {
			e = i.manager.Publish(c)
		}
		if e != nil {
			i.startupErr = e
		}
	} else if !os.IsNotExist(e) {
		i.startupErr = e
	}
}
func (i *Issuer) PreCheck(ctx context.Context, names []string, interactive bool) error {
	if i.startupErr != nil {
		return i.startupErr
	}
	if len(i.config.Issuers) != 1 {
		return fmt.Errorf("QWAC issuer requires exactly one issuer; fallback bypass prohibited")
	}
	if len(names) != 1 || names[0] != i.Binding.Host {
		return fmt.Errorf("QWAC issuer restricted to %s", i.Binding.Host)
	}
	return i.ACMEIssuer.PreCheck(ctx, names, interactive)
}
func (i *Issuer) Issue(ctx context.Context, csr *x509.CertificateRequest) (*certmagic.IssuedCertificate, error) {
	if e := i.PreCheck(ctx, csr.DNSNames, false); e != nil {
		return nil, e
	}
	cert, e := i.ACMEIssuer.Issue(ctx, csr)
	if e != nil {
		return nil, e
	}
	b, _ := pem.Decode(cert.Certificate)
	if b == nil {
		return nil, fmt.Errorf("ACME returned no certificate")
	}
	c, e := x509.ParseCertificate(b.Bytes)
	if e != nil {
		return nil, e
	}
	if e = i.manager.Publish(c); e != nil {
		return nil, fmt.Errorf("binding gate refused TLS certificate: %w", e)
	}
	return cert, nil
}

type Handler struct {
	Binding BindingConfig `json:"binding"`
	manager *store.Manager
	cancel  context.CancelFunc
}

func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.qwac", New: func() caddy.Module { return new(Handler) }}
}
func (h *Handler) Provision(ctx caddy.Context) error {
	return h.ProvisionWithOperations(ctx, store.Operations{})
}

// ProvisionWithOperations is the development adapter entry point.
func (h *Handler) ProvisionWithOperations(ctx caddy.Context, ops store.Operations) error {
	m, e := Shared(h.Binding, ops)
	if e != nil {
		return e
	}
	h.manager = m
	child, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-child.Done():
				return
			case <-t.C:
				if e := m.Refresh(); e != nil {
					ctx.Logger().Warn(e.Error())
				}
			}
		}
	}()
	return nil
}
func (h *Handler) Cleanup() error {
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	state := h.manager.Snapshot()
	if strings.HasPrefix(r.URL.Path, "/bindings/") {
		name := strings.TrimPrefix(r.URL.Path, "/bindings/")
		if strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".jose") {
			return caddyhttp.Error(404, nil)
		}
		d, e := os.ReadFile(filepath.Join(h.Binding.Dir, name))
		if e != nil {
			return caddyhttp.Error(404, e)
		}
		w.Header().Set("Content-Type", "application/jose")
		w.Header().Set("Cache-Control", "no-store")
		_, e = w.Write(d)
		return e
	}
	if state.URL == "" || state.Expires <= time.Now().Unix() {
		return caddyhttp.Error(503, fmt.Errorf("binding unavailable or expired"))
	}
	w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="tls-certificate-binding"`, state.URL))
	w.Header().Set("Cache-Control", "no-store")
	return next.ServeHTTP(w, r)
}

var _ certmagic.Issuer = (*Issuer)(nil)
var _ certmagic.PreChecker = (*Issuer)(nil)
var _ certmagic.Revoker = (*Issuer)(nil)
var _ certmagic.RenewalInfoGetter = (*Issuer)(nil)
var _ caddytls.ConfigSetter = (*Issuer)(nil)
var _ caddyhttp.MiddlewareHandler = (*Handler)(nil)

// BindingConfig selects credentials explicitly; the core never knows a test-PKI layout.
type BindingConfig struct {
	store.Config
	ChainFiles []string `json:"chain_files"`
	KeyFile    string   `json:"key_file"`
}

var registry = struct {
	sync.Mutex
	entries map[string]sharedEntry
}{entries: make(map[string]sharedEntry)}

type sharedEntry struct {
	cfg                      BindingConfig
	manager                  *store.Manager
	customSign, customCommit bool
}

// Shared gives issuer, refresh middleware and development diagnostics one manager.
// Each directory has one configuration and operation mode for the process lifetime.
func Shared(cfg BindingConfig, ops store.Operations) (*store.Manager, error) {
	registry.Lock()
	defer registry.Unlock()
	dir, e := filepath.Abs(cfg.Dir)
	if e != nil {
		return nil, e
	}
	cfg.Dir = dir
	if cfg.Lifetime == 0 {
		cfg.Lifetime = 45 * time.Second
	}
	if entry, ok := registry.entries[dir]; ok {
		if !reflect.DeepEqual(entry.cfg, cfg) || entry.customSign != (ops.Sign != nil) || entry.customCommit != (ops.CommitManifest != nil) {
			return nil, fmt.Errorf("binding manager config or operation mode mismatch")
		}
		return entry.manager, nil
	}
	mat, e := credentials.Load(cfg.ChainFiles, cfg.KeyFile)
	if e != nil {
		return nil, e
	}
	m, e := store.Open(cfg.Config, mat, ops)
	if e != nil {
		return nil, e
	}
	cfg.ChainFiles = append([]string(nil), cfg.ChainFiles...)
	registry.entries[dir] = sharedEntry{cfg, m, ops.Sign != nil, ops.CommitManifest != nil}
	return m, nil
}
