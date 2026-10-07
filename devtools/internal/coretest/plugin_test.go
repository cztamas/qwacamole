package coretest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

func supplied(t *testing.T) (plugin.BindingConfig, *x509.Certificate) {
	t.Helper()
	dir := t.TempDir()
	pkidir := filepath.Join(dir, "pki")
	if e := pki.Generate(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Supplied TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); e != nil {
		t.Fatal(e)
	}
	cfg := plugin.BindingConfig{Config: store.Config{Dir: filepath.Join(dir, "bindings"), Host: "qwac.example.com", Lifetime: time.Minute}, ChainFiles: []string{filepath.Join(pkidir, "qwac-leaf.pem"), filepath.Join(pkidir, "qwac-intermediate.pem")}, KeyFile: filepath.Join(pkidir, "qwac-leaf-key.pem"), TrustFiles: []string{filepath.Join(pkidir, "qwac-root.pem")}}
	tls, e := pki.ReadCert(filepath.Join(pkidir, "tls-leaf.pem"))
	if e != nil {
		t.Fatal(e)
	}
	return cfg, tls
}
func newContext() (caddy.Context, context.CancelFunc) {
	return caddy.NewContext(caddy.Context{Context: context.Background()})
}
func replaceLeaf(t *testing.T, cfg plugin.BindingConfig, owner string) {
	t.Helper()
	old, e := pki.ReadCert(cfg.ChainFiles[0])
	if e != nil {
		t.Fatal(e)
	}
	ca, e := pki.ReadCert(cfg.ChainFiles[1])
	if e != nil {
		t.Fatal(e)
	}
	caKey, e := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "qwac-intermediate-key.pem"))
	if e != nil {
		t.Fatal(e)
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := *old
	template.RawSubject = nil
	template.SerialNumber = pki.Serial()
	template.Subject.Organization = []string{owner}
	template.Subject.Names = nil
	template.Subject.ExtraNames = old.Subject.Names // update O below
	for n, v := range template.Subject.ExtraNames {
		if v.Type.String() == "2.5.4.10" {
			template.Subject.ExtraNames[n].Value = owner
		}
	}
	template.ExtraExtensions = old.Extensions
	replacement, e := pki.Issue(&template, ca, &key.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	if e = pki.WriteCert(cfg.ChainFiles[0], replacement); e != nil {
		t.Fatal(e)
	}
	if e = pki.WriteKey(cfg.KeyFile, key); e != nil {
		t.Fatal(e)
	}
}
func request(h *plugin.Handler, path, method string, next caddyhttp.Handler) (*httptest.ResponseRecorder, error) {
	w := httptest.NewRecorder()
	e := h.ServeHTTP(w, httptest.NewRequest(method, "https://qwac.example.com"+path, nil), next)
	return w, e
}
func TestOverlappingReloadAndTransactionalReplacement(t *testing.T) {
	cfg, tls := supplied(t)
	ctx1, cancel1 := newContext()
	defer cancel1()
	var fail atomic.Bool
	ops := store.Operations{CommitManifest: func(path string, d []byte, _ bool) error {
		if fail.Load() {
			return fmt.Errorf("commit unavailable")
		}
		return store.Atomic(path, d)
	}}
	old, e := plugin.Acquire(ctx1, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	defer old.Release()
	if e = old.Activate(); e != nil {
		t.Fatal(e)
	}
	if e = old.Manager().Publish(tls); e != nil {
		t.Fatal(e)
	}
	initial := old.Manager().Snapshot()
	// Unchanged overlapping validation must neither republish nor stop old owner.
	ctx2, cancel2 := newContext()
	defer cancel2()
	same, e := plugin.Acquire(ctx2, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	if same.Manager() != old.Manager() || same.Manager().Snapshot().URL != initial.URL {
		t.Fatal("validation mutated state")
	}
	same.Release()
	cancel2()
	replaceLeaf(t, cfg, "Replacement TEST")
	ctx3, cancel3 := newContext()
	defer cancel3()
	replacement, e := plugin.Acquire(ctx3, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	defer replacement.Release()
	if old.Manager().Snapshot().Owner.Organization != "Supplied TEST" {
		t.Fatal("staged replacement leaked")
	}
	fail.Store(true)
	if e = replacement.Activate(); e == nil {
		t.Fatal("failed publication activated")
	}
	if old.Manager().Snapshot().URL != initial.URL {
		t.Fatal("failed replacement mutated current binding")
	}
	fail.Store(false)
	if e = replacement.Activate(); e != nil {
		t.Fatal(e)
	}
	state := old.Manager().Snapshot()
	if state.Owner.Organization != "Replacement TEST" || len(state.IDs) != 1 || state.IDs[0] != binding.ID(tls) || state.URL == initial.URL {
		t.Fatal("replacement lost owner/coverage")
	}
	if e = old.Activate(); e != nil {
		t.Fatal(e)
	}
	if old.Manager().Snapshot().Owner.Organization != "Replacement TEST" {
		t.Fatal("old lease rolled back material")
	}
	old.Release()
	cancel1()
	if e = replacement.Manager().Refresh(); e != nil {
		t.Fatal("old cleanup stopped replacement", e)
	}
	if _, e = replacement.Manager().Document(initial.URL); e != nil {
		t.Fatal("historical document lost", e)
	}
	// A valid staged replacement in a rejected configuration never becomes live.
	replaceLeaf(t, cfg, "Rejected candidate TEST")
	rejectedCtx, rejectedStop := newContext()
	staged, e := plugin.Acquire(rejectedCtx, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	staged.Release()
	rejectedStop()
	if replacement.Manager().Snapshot().Owner.Organization != "Replacement TEST" {
		t.Fatal("rejected candidate leaked")
	}
	// Invalid replacement rejects provisioning and retains working manager.
	os.WriteFile(cfg.KeyFile, []byte("invalid key"), 0600)
	ctx4, cancel4 := newContext()
	defer cancel4()
	if _, e = plugin.Acquire(ctx4, cfg, ops); e == nil {
		t.Fatal("invalid reload accepted")
	}
	if replacement.Manager().Snapshot().URL != state.URL {
		t.Fatal("invalid reload changed active state")
	}
	replacement.Release()
	cancel3()
	// Reacquisition after every lease is released proves entry and lock cleanup.
	// Restore a valid key by generating another leaf under the same authority.
	replaceLeaf(t, cfg, "Recovered TEST")
	ctx5, cancel5 := newContext()
	defer cancel5()
	last, e := plugin.Acquire(ctx5, cfg, ops)
	if e != nil {
		t.Fatal("owner leaked after cleanup", e)
	}
	last.Release()
}
func TestEarlyCredentialsAndConflictingOwnership(t *testing.T) {
	cfg, _ := supplied(t)
	ctx, cancel := newContext()
	defer cancel()
	l, e := plugin.Acquire(ctx, cfg, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release()
	for _, mutate := range []func(*plugin.BindingConfig){func(c *plugin.BindingConfig) { c.Host = "other.example.com" }, func(c *plugin.BindingConfig) { c.TrustFiles = nil }, func(c *plugin.BindingConfig) { c.Lifetime = 2 * time.Minute }, func(c *plugin.BindingConfig) { c.FirefoxCompatibility = true }, func(c *plugin.BindingConfig) { c.Host = "*.example.com" }} {
		other := cfg
		mutate(&other)
		newctx, stop := newContext()
		_, e := plugin.Acquire(newctx, other, store.Operations{})
		stop()
		if e == nil {
			t.Fatal("invalid/conflicting config accepted", other)
		}
	}
	// Separate issuer/handler paths within a config may not silently diverge.
	other := cfg
	other.ChainFiles = append(append([]string(nil), cfg.ChainFiles...), cfg.TrustFiles...)
	if _, e := plugin.Acquire(ctx, other, store.Operations{}); e == nil {
		t.Fatal("inconsistent issuer/handler accepted")
	}
}
func TestHeadersDocumentsStreamingAndUnrelatedHosts(t *testing.T) {
	cfg, tls := supplied(t)
	ctx, cancel := newContext()
	defer cancel()
	h := &plugin.Handler{Binding: cfg}
	if e := h.Provision(ctx); e != nil {
		t.Fatal(e)
	}
	defer h.Cleanup()
	l, e := plugin.Acquire(ctx, cfg, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release()
	l.SetStoredTLS(tls)
	next := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Link", `</style.css>; rel="preload"`)
		w.Header().Add("Link", `</next>; rel="next"`)
		w.Header().Set("Cache-Control", "public, max-age=321")
		w.WriteHeader(201)
		w.Write([]byte("first"))
		if e := http.NewResponseController(w).Flush(); e != nil {
			return e
		}
		_, e := w.Write([]byte("second"))
		return e
	})
	w, e := request(h, "/app", "GET", next)
	if e != nil {
		t.Fatal(e)
	}
	if w.Code != 201 || w.Body.String() != "firstsecond" || !w.Flushed || w.Header().Get("Cache-Control") != "public, max-age=321" || len(w.Header().Values("Link")) != 3 || !strings.Contains(strings.Join(w.Header().Values("Link"), ","), store.DiscoveryURL) {
		t.Fatal("application response changed", w)
	}
	url := store.DiscoveryURL
	get, e := request(h, url, "GET", nil)
	if e != nil {
		t.Fatal(e)
	}
	head, e := request(h, url, "HEAD", nil)
	if e != nil || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") {
		t.Fatal("HEAD", e)
	}
	if get.Header().Get("Cache-Control") != "no-store" || get.Header().Get("Link") != "" {
		t.Fatal("document headers")
	}
	for _, path := range []string{"/bindings/../current.json", "/bindings/current.json", "/bindings/" + strings.Repeat("0", 64) + ".jose"} {
		if _, e = request(h, path, "GET", nil); e == nil {
			t.Fatal("invalid document served", path)
		}
	}
	if post, e := request(h, url, "POST", nil); e == nil || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatal("POST allowed")
	}
	w = httptest.NewRecorder()
	e = h.ServeHTTP(w, httptest.NewRequest("GET", "https://unrelated.example.com/app", nil), next)
	if e != nil || len(w.Header().Values("Link")) != 2 {
		t.Fatal("unrelated host affected")
	}
}
func TestExplicitTrustBundlePKCS8(t *testing.T) {
	cfg, _ := supplied(t)
	leaf, _ := os.ReadFile(cfg.ChainFiles[0])
	inter, _ := os.ReadFile(cfg.ChainFiles[1])
	bundle := filepath.Join(filepath.Dir(cfg.KeyFile), "bundle.pem")
	os.WriteFile(bundle, append(leaf, inter...), 0600)
	cfg.ChainFiles = []string{bundle}
	key, e := credentials.ReadKey(cfg.KeyFile)
	if e != nil {
		t.Fatal(e)
	}
	der, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(cfg.KeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600)
	ctx, cancel := newContext()
	defer cancel()
	l, e := plugin.Acquire(ctx, cfg, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	l.Release()
}

func TestSuppliedCredentialsRejectBeforeIssuance(t *testing.T) {
	cfg, _ := supplied(t)
	leafPath := cfg.ChainFiles[0]
	original, e := os.ReadFile(leafPath)
	if e != nil {
		t.Fatal(e)
	}
	leaf, e := pki.ReadCert(leafPath)
	if e != nil {
		t.Fatal(e)
	}
	ca, e := pki.ReadCert(cfg.ChainFiles[1])
	if e != nil {
		t.Fatal(e)
	}
	caKey, e := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "qwac-intermediate-key.pem"))
	if e != nil {
		t.Fatal(e)
	}
	key, e := pki.ReadKey(cfg.KeyFile)
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		name   string
		mutate func(*x509.Certificate)
		want   string
	}{
		{"expired", func(c *x509.Certificate) {
			c.NotBefore = time.Now().Add(-2 * time.Hour)
			c.NotAfter = time.Now().Add(-time.Hour)
		}, "outside validity"},
		{"not-yet-valid", func(c *x509.Certificate) {
			c.NotBefore = time.Now().Add(time.Hour)
			c.NotAfter = time.Now().Add(2 * time.Hour)
		}, "outside validity"},
		{"wrong-host", func(c *x509.Certificate) {
			c.DNSNames = []string{"other.example.com"}
			var exts []pkix.Extension
			for _, e := range c.ExtraExtensions {
				if e.Id.String() != "2.5.29.17" {
					exts = append(exts, e)
				}
			}
			c.ExtraExtensions = exts
		}, "valid for"},
		{"wrong-profile", func(c *x509.Certificate) {
			c.Policies = nil
			c.PolicyIdentifiers = nil
			var exts []pkix.Extension
			for _, e := range c.ExtraExtensions {
				if e.Id.String() != "2.5.29.32" {
					exts = append(exts, e)
				}
			}
			c.ExtraExtensions = exts
		}, "missing QNCP"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tmpl := *leaf
			tmpl.ExtraExtensions = leaf.Extensions
			test.mutate(&tmpl)
			bad, e := pki.Issue(&tmpl, ca, &key.PublicKey, caKey)
			if e != nil {
				t.Fatal(e)
			}
			pki.WriteCert(leafPath, bad)
			defer os.WriteFile(leafPath, original, 0600)
			ctx, cancel := newContext()
			defer cancel()
			if _, e := plugin.Acquire(ctx, cfg, store.Operations{}); e == nil || !strings.Contains(e.Error(), test.want) {
				t.Fatal("early validation missing", e)
			}
		})
	}
	bad := cfg
	bad.TrustFiles = []string{filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root.pem")}
	ctx, cancel := newContext()
	defer cancel()
	if _, e := plugin.Acquire(ctx, bad, store.Operations{}); e == nil {
		t.Fatal("untrusted chain accepted")
	}
	bad = cfg
	bad.ChainFiles = []string{cfg.ChainFiles[0]}
	if _, e := plugin.Acquire(ctx, bad, store.Operations{}); e == nil {
		t.Fatal("missing intermediate accepted")
	}
}
