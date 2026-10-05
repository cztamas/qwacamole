package coretest

import (
	"context"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProductionMiddlewarePassesApplicationRoutesAndIgnoresFaultFiles(t *testing.T) {
	dir := t.TempDir()
	pkidir := filepath.Join(dir, "pki")
	if e := pki.Generate(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Middleware TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); e != nil {
		t.Fatal(e)
	}
	cfg := plugin.BindingConfig{Config: store.Config{Dir: filepath.Join(dir, "bindings"), Host: "qwac.example.com", Lifetime: time.Minute}, ChainFiles: []string{filepath.Join(pkidir, "qwac-leaf.pem"), filepath.Join(pkidir, "qwac-intermediate.pem"), filepath.Join(pkidir, "qwac-root.pem")}, KeyFile: filepath.Join(pkidir, "qwac-leaf-key.pem")}
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	h := &plugin.Handler{Binding: cfg}
	if e := h.Provision(ctx); e != nil {
		t.Fatal(e)
	}
	defer h.Cleanup()
	m, e := plugin.Shared(cfg, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"fail-sign", "fail-publish"} {
		if e = os.WriteFile(filepath.Join(cfg.Dir, name), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	tls, e := pki.ReadCert(filepath.Join(pkidir, "tls-leaf.pem"))
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Publish(tls); e != nil {
		t.Fatal("production read development fault marker", e)
	}
	for _, path := range []string{"/", "/state", "/init", "/test-pki/qwac-leaf-key.pem"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		next := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, e := w.Write([]byte("application:" + r.URL.Path))
			return e
		})
		if e = h.ServeHTTP(w, r, next); e != nil {
			t.Fatal(e)
		}
		if w.Body.String() != "application:"+path || w.Header().Get("Link") == "" {
			t.Fatal("application route intercepted or unadvertised", path)
		}
	}
	w := httptest.NewRecorder()
	if e = h.ServeHTTP(w, httptest.NewRequest("GET", m.Snapshot().URL, nil), nil); e != nil {
		t.Fatal(e)
	}
	if w.Header().Get("Content-Type") != "application/jose" || w.Header().Get("Cache-Control") != "no-store" || w.Body.Len() == 0 {
		t.Fatal("binding response changed")
	}
	mismatch := cfg
	mismatch.KeyFile = "other"
	if _, e = plugin.Shared(mismatch, store.Operations{}); e == nil {
		t.Fatal("inconsistent sharing accepted")
	}
}
