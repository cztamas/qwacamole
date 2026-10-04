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
	"github.com/cztamas/qwacamole/internal/store"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() { caddy.RegisterModule(Issuer{}); caddy.RegisterModule(Handler{}) }

type Issuer struct {
	caddytls.ACMEIssuer `json:"acme"`
	Binding             store.Config `json:"binding"`
	manager             *store.Manager
	config              *certmagic.Config
	startupErr          error
}

func (Issuer) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "tls.issuance.qwac", New: func() caddy.Module { return new(Issuer) }}
}
func (i *Issuer) Provision(ctx caddy.Context) error {
	if e := i.ACMEIssuer.Provision(ctx); e != nil {
		return e
	}
	m, e := store.Shared(i.Binding)
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
		return fmt.Errorf("demo restricted to %s", i.Binding.Host)
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
	PublicPKIOnly bool         `json:"public_pki_only,omitempty"`
	Binding       store.Config `json:"binding"`
	manager       *store.Manager
	cancel        context.CancelFunc
}

func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.qwac", New: func() caddy.Module { return new(Handler) }}
}
func (h *Handler) Provision(ctx caddy.Context) error {
	m, e := store.Shared(h.Binding)
	if e != nil {
		return e
	}
	h.manager = m
	if h.PublicPKIOnly {
		return nil
	}
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
	if strings.HasPrefix(r.URL.Path, "/test-pki/") {
		name := strings.TrimPrefix(r.URL.Path, "/test-pki/")
		if name != "qwac-root.cer" && name != "qwac-intermediate.cer" && name != "qwac-root.crl" && name != "qwac-intermediate.crl" {
			return caddyhttp.Error(404, nil)
		}
		d, e := os.ReadFile(filepath.Join(h.Binding.PKI, name))
		if e != nil {
			return caddyhttp.Error(404, e)
		}
		content := "application/pkix-cert"
		if strings.HasSuffix(name, ".crl") {
			content = "application/pkix-crl"
		}
		w.Header().Set("Content-Type", content)
		_, e = w.Write(d)
		return e
	}
	if h.PublicPKIOnly {
		return caddyhttp.Error(404, nil)
	}
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
	if r.URL.Path == "/init" {
		w.WriteHeader(200)
		return nil
	}
	if state.URL == "" || state.Expires <= time.Now().Unix() {
		return caddyhttp.Error(503, fmt.Errorf("binding unavailable or expired"))
	}
	w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="tls-certificate-binding"`, state.URL))
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/state" {
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(state)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return page.Execute(w, state)
}

var page = template.Must(template.New("demo").Parse(`<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Qwacamole — TEST PKI</title><style>body{font:18px system-ui;max-width:850px;margin:60px auto;padding:24px;background:#f5f5ef;color:#183b2d}code,a{overflow-wrap:anywhere}li{margin:12px 0}</style></head><body><h1>Qwacamole</h1><p>Synthetic organization: {{.Owner.Organization}}, {{.Owner.Locality}}, {{.Owner.Country}}, {{.Owner.Registration}}.</p><p><strong>TEST PKI. No legal qualification.</strong></p><p>This page reports server state. It cannot independently attest browser QWAC verification; that evidence comes from Chromium NetLog.</p><h2>Published TLS identifiers</h2><ul>{{range .IDs}}<li><code>{{.}}</code></li>{{end}}</ul><p>Binding: <a href="{{.URL}}">{{.URL}}</a></p><p>Expiry (Unix): {{.Expires}}</p></body></html>`))
var _ certmagic.Issuer = (*Issuer)(nil)
var _ certmagic.PreChecker = (*Issuer)(nil)
var _ certmagic.Revoker = (*Issuer)(nil)
var _ certmagic.RenewalInfoGetter = (*Issuer)(nil)
var _ caddytls.ConfigSetter = (*Issuer)(nil)
var _ caddyhttp.MiddlewareHandler = (*Handler)(nil)
