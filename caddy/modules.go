// Package qwacamole provides Caddy's native ACME issuer gate and HTTP handler.
package qwacamole

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddytls"
	"github.com/caddyserver/certmagic"
	"github.com/cztamas/qwacamole/store"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

func init() { caddy.RegisterModule(Issuer{}); caddy.RegisterModule(Handler{}) }

type Issuer struct {
	caddytls.ACMEIssuer `json:"acme"`
	Binding             BindingConfig `json:"binding"`
	manager             *store.Manager
	lease               *Lease
	ctx                 caddy.Context
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
	if ctx.Module() != nil {
		if e := i.checkPolicy(ctx); e != nil {
			return e
		}
	}
	if e := i.ACMEIssuer.Provision(ctx); e != nil {
		return e
	}
	lease, e := Acquire(ctx, i.Binding, ops)
	if e != nil {
		return e
	}
	i.lease = lease
	i.manager = lease.Manager()
	i.ctx = ctx
	return nil
}

// Check raw policy shape while it is still available during issuer provisioning.
// This rejects fallback/external/on-demand configurations even without middleware.
func (i *Issuer) checkPolicy(ctx caddy.Context) error {
	app, e := ctx.App("tls")
	if e != nil {
		return e
	}
	tls := app.(*caddytls.TLS)
	if tls.Automation != nil {
		for _, p := range tls.Automation.Policies {
			matches := len(p.SubjectsRaw) == 0
			for _, name := range p.SubjectsRaw {
				if name == i.Binding.Host {
					matches = true
				}
			}
			if !matches {
				continue
			}
			if len(p.SubjectsRaw) != 1 || p.SubjectsRaw[0] != i.Binding.Host || len(p.IssuersRaw) != 1 || len(p.ManagersRaw) > 0 || p.OnDemand {
				return fmt.Errorf("QWAC issuer requires one explicit hostname, sole issuer and no external/on-demand/fallback policy")
			}
			var m struct {
				Module string `json:"module"`
			}
			if e = json.Unmarshal(p.IssuersRaw[0], &m); e != nil {
				return e
			}
			if m.Module != "qwac" && m.Module != "qwac_demo" {
				return fmt.Errorf("intended QWAC issuer must be explicit")
			}
			return nil
		}
	}
	return fmt.Errorf("QWAC issuer requires an explicit hostname automation policy")
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
			if i.lease != nil {
				i.lease.SetStoredTLS(c)
			}
		}
		if e != nil {
			i.startupErr = e
		}
	} else if !os.IsNotExist(e) {
		i.startupErr = e
	}
}
func (i *Issuer) Cleanup() error             { i.lease.Release(); return nil }
func (i *Issuer) QWACBinding() BindingConfig { return i.Binding }
func (i *Issuer) StartupError() error        { return i.startupErr }

func (i *Issuer) PreCheck(ctx context.Context, names []string, interactive bool) error {
	if i.startupErr != nil {
		return i.startupErr
	}
	if i.config == nil || len(i.config.Issuers) != 1 {
		return fmt.Errorf("QWAC issuer requires exactly one issuer; fallback bypass prohibited")
	}
	if len(names) != 1 || names[0] != i.Binding.Host {
		return fmt.Errorf("QWAC issuer restricted to %s", i.Binding.Host)
	}
	return i.ACMEIssuer.PreCheck(ctx, names, interactive)
}
func (i *Issuer) Issue(ctx context.Context, csr *x509.CertificateRequest) (*certmagic.IssuedCertificate, error) {
	release, e := i.lease.borrow()
	if e != nil {
		return nil, e
	}
	defer release()
	if e := i.PreCheck(ctx, csr.DNSNames, false); e != nil {
		return nil, e
	}
	if e := i.lease.Activate(); e != nil {
		return nil, e
	}
	cert, e := i.ACMEIssuer.Issue(ctx, csr)
	if e != nil {
		return nil, e
	}
	if e = i.lease.ctx.Err(); e != nil {
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
		i.ctx.Logger().Error("TLS replacement refused by QWAC publication gate", zap.String("hostname", i.Binding.Host), zap.Error(e))
		return nil, fmt.Errorf("binding gate refused TLS certificate: %w", e)
	}
	return cert, nil
}

type Handler struct {
	Binding BindingConfig `json:"binding"`
	manager *store.Manager
	lease   *Lease
	ctx     caddy.Context
}

func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.qwac", New: func() caddy.Module { return new(Handler) }}
}
func (h *Handler) Provision(ctx caddy.Context) error {
	return h.ProvisionWithOperations(ctx, store.Operations{})
}
func (h *Handler) ProvisionWithOperations(ctx caddy.Context, ops store.Operations) error {
	l, e := Acquire(ctx, h.Binding, ops)
	if e != nil {
		return e
	}
	h.lease = l
	h.manager = l.Manager()
	h.ctx = ctx
	return nil
}
func (h *Handler) Cleanup() error { h.lease.Release(); return nil }

// Validate checks the first matching automation policy, including fallback and
// external-manager bypasses, before any ACME attempt. Embedded dev adapters use
// the same interface and validation.
func (h *Handler) Validate() error {
	app, e := h.ctx.App("tls")
	if e != nil {
		return e
	}
	tls := app.(*caddytls.TLS)
	if tls.Automation != nil {
		for _, p := range tls.Automation.Policies {
			subjects := p.Subjects()
			matched := len(subjects) == 0
			for _, s := range subjects {
				if s == h.Binding.Host || s == "*" || strings.HasPrefix(s, "*.") && strings.HasSuffix(h.Binding.Host, s[1:]) {
					matched = true
				}
			}
			if !matched {
				continue
			}
			if len(subjects) != 1 || subjects[0] != h.Binding.Host || len(p.Issuers) != 1 || p.OnDemand || len(p.Managers) > 0 {
				return fmt.Errorf("QWAC requires one explicit hostname policy, sole QWAC issuer, no on-demand/external/fallback issuers")
			}
			i, ok := p.Issuers[0].(interface {
				QWACBinding() BindingConfig
				StartupError() error
			})
			if !ok {
				return fmt.Errorf("protected hostname requires the QWAC issuer")
			}
			if e := i.StartupError(); e != nil {
				return e
			}
			a, e := i.QWACBinding().normalized()
			if e != nil {
				return e
			}
			b, e := h.Binding.normalized()
			if e != nil {
				return e
			}
			if !reflect.DeepEqual(a, b) {
				return fmt.Errorf("issuer/handler binding configuration mismatch")
			}
			return nil
		}
	}
	return fmt.Errorf("no explicit QWAC issuer policy for protected hostname")
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	host := r.Host
	if v, _, e := net.SplitHostPort(host); e == nil {
		host = v
	}
	if strings.TrimSuffix(strings.ToLower(host), ".") != h.Binding.Host {
		return next.ServeHTTP(w, r)
	}
	release, e := h.lease.borrow()
	if e != nil {
		return caddyhttp.Error(503, e)
	}
	defer release()
	if e := h.lease.Activate(); e != nil {
		return caddyhttp.Error(503, e)
	}
	if strings.HasPrefix(r.URL.Path, "/bindings/") {
		return h.serveDocument(w, r)
	}
	state := h.manager.Snapshot()
	if state.URL == "" || state.Expires <= time.Now().Unix() {
		return caddyhttp.Error(503, fmt.Errorf("binding unavailable or expired"))
	}
	rw := &bindingWriter{ResponseWriter: w, manager: h.manager}
	e = next.ServeHTTP(rw, r)
	if e == nil && !rw.committed {
		rw.WriteHeader(http.StatusOK)
	}
	return e
}

var _ certmagic.Issuer = (*Issuer)(nil)
var _ certmagic.PreChecker = (*Issuer)(nil)
var _ certmagic.Revoker = (*Issuer)(nil)
var _ certmagic.RenewalInfoGetter = (*Issuer)(nil)
var _ caddytls.ConfigSetter = (*Issuer)(nil)
var _ caddyhttp.MiddlewareHandler = (*Handler)(nil)
