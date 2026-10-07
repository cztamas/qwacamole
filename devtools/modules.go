// Package devtools registers development-only Caddy adapters and diagnostic routes.
// It is deliberately excluded from a production plugin-only xcaddy build.
package devtools

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

func init() {
	caddy.RegisterModule(Issuer{})
	caddy.RegisterModule(BindingHandler{})
	caddy.RegisterModule(Demo{})
}

// FaultOperations retains the CLI's file protocol only in development builds.
// The failures occur at signing and after immutable publication, before commit.
func FaultOperations(cfg plugin.BindingConfig) store.Operations {
	return store.Operations{
		Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, newCertificate bool) (string, binding.Header, error) {
			if newCertificate {
				if _, e := os.Stat(filepath.Join(cfg.Dir, "fail-sign")); e == nil {
					return "", binding.Header{}, fmt.Errorf("injected binding fail-sign failure for new TLS certificate")
				}
			}
			sign := binding.Sign
			if cfg.FirefoxCompatibility {
				sign = binding.SignFirefox
			}
			return sign(m, cs, now, lifetime)
		},
		CommitManifest: func(path string, d []byte, newCertificate bool) error {
			if newCertificate {
				if _, e := os.Stat(filepath.Join(cfg.Dir, "fail-publish")); e == nil {
					return fmt.Errorf("injected binding fail-publish failure for new TLS certificate")
				}
			}
			return store.Atomic(path, d)
		},
	}
}

type Issuer struct{ plugin.Issuer }

func (Issuer) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "tls.issuance.qwac_demo", New: func() caddy.Module { return new(Issuer) }}
}
func (i *Issuer) Provision(ctx caddy.Context) error {
	return i.Issuer.ProvisionWithOperations(ctx, FaultOperations(i.Binding))
}

type BindingHandler struct{ plugin.Handler }

func (BindingHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.qwac_demo_binding", New: func() caddy.Module { return new(BindingHandler) }}
}
func (h *BindingHandler) Provision(ctx caddy.Context) error {
	return h.Handler.ProvisionWithOperations(ctx, FaultOperations(h.Binding))
}

type Demo struct {
	Binding       plugin.BindingConfig `json:"binding"`
	PKI           string               `json:"pki"`
	PublicPKIOnly bool                 `json:"public_pki_only,omitempty"`
	manager       *store.Manager
	lease         *plugin.Lease
}

func (Demo) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.qwac_demo", New: func() caddy.Module { return new(Demo) }}
}
func (h *Demo) Provision(ctx caddy.Context) error {
	if h.PublicPKIOnly {
		return nil
	}
	l, e := plugin.Acquire(ctx, h.Binding, FaultOperations(h.Binding))
	if e != nil {
		return e
	}
	h.lease = l
	h.manager = l.Manager()
	return nil
}
func (h *Demo) Cleanup() error { h.lease.Release(); return nil }
func (h *Demo) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if strings.HasPrefix(r.URL.Path, "/test-pki/") {
		name := strings.TrimPrefix(r.URL.Path, "/test-pki/")
		if name != "qwac-root.cer" && name != "qwac-intermediate.cer" && name != "qwac-root.crl" && name != "qwac-intermediate.crl" {
			return caddyhttp.Error(404, nil)
		}
		d, e := os.ReadFile(filepath.Join(h.PKI, name))
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
	if r.URL.Path == "/init" {
		w.WriteHeader(200)
		return nil
	}
	state := h.manager.Snapshot()
	if r.URL.Path == "/state" {
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(state)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.URL.Path == "/cache-origin" {
		w.Header().Set("Cache-Control", "public, max-age=600")
	}
	return page.Execute(w, state)
}
