package coretest

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/cacheproxy"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

func TestActivationFailureIncludingUnretainedStoredTLSIsAtomic(t *testing.T) {
	cfg, tls := supplied(t)
	var fail atomic.Bool
	var commits atomic.Int32
	ops := store.Operations{
		Sign: func(mat *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
			if fail.Load() && len(cs) > 1 {
				return "", binding.Header{}, fmt.Errorf("stored TLS inclusion signing failed")
			}
			return binding.Sign(mat, cs, now, lifetime)
		},
		CommitManifest: func(path string, d []byte, _ bool) error {
			commits.Add(1)
			return store.Atomic(path, d)
		},
	}
	ctx, cancel := newContext()
	initial, e := plugin.Acquire(ctx, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	if e = initial.Publish(tls); e != nil {
		t.Fatal(e)
	}
	initial.Release()
	cancel()
	// Reacquire from disk so the regression exercises recovered coverage.
	oldCtx, oldCancel := newContext()
	defer oldCancel()
	old, e := plugin.Acquire(oldCtx, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	defer old.Release()
	if e = old.Activate(); e != nil {
		t.Fatal(e)
	}
	before := old.Manager().Snapshot()
	manifestPath := filepath.Join(cfg.Dir, "current.json")
	manifest, e := os.ReadFile(manifestPath)
	if e != nil {
		t.Fatal(e)
	}
	document, e := old.Manager().CurrentDocument()
	if e != nil {
		t.Fatal(e)
	}
	pkidir := filepath.Dir(cfg.KeyFile)
	root, _ := pki.ReadCert(filepath.Join(pkidir, "tls-root.pem"))
	rootKey, _ := pki.ReadKey(filepath.Join(pkidir, "tls-root-key.pem"))
	tmpl := *tls
	tmpl.SerialNumber = pki.Serial()
	stored, e := pki.Issue(&tmpl, root, tls.PublicKey.(*rsa.PublicKey), rootKey)
	if e != nil {
		t.Fatal(e)
	}
	if reflect.DeepEqual(before.IDs, []string{binding.ID(tls), binding.ID(stored)}) {
		t.Fatal("stored leaf must be absent from recovered binding")
	}
	replaceLeaf(t, cfg, "Atomic replacement TEST")
	nextCtx, nextCancel := newContext()
	defer nextCancel()
	next, e := plugin.Acquire(nextCtx, cfg, ops) // preparation succeeds
	if e != nil {
		t.Fatal(e)
	}
	defer next.Release()
	next.SetStoredTLS(stored)
	commits.Store(0)
	fail.Store(true)
	if e = next.Activate(); e == nil {
		t.Fatal("stored TLS inclusion failure accepted")
	}
	afterManifest, _ := os.ReadFile(manifestPath)
	afterDocument, docErr := old.Manager().CurrentDocument()
	if !reflect.DeepEqual(old.Manager().Snapshot(), before) || string(afterManifest) != string(manifest) || string(afterDocument) != string(document) || docErr != nil || commits.Load() != 0 {
		t.Fatal("partial replacement escaped the failed complete-set transaction", docErr, commits.Load())
	}
	fail.Store(false)
	if e = next.Activate(); e != nil {
		t.Fatal(e)
	}
	after := old.Manager().Snapshot()
	if commits.Load() != 1 || after.Owner.Organization != "Atomic replacement TEST" || !reflect.DeepEqual(after.IDs, []string{binding.ID(tls), binding.ID(stored)}) {
		t.Fatal("successful activation must commit signer and complete coverage once", after, commits.Load())
	}
	if e = old.Activate(); e != nil || old.Manager().Snapshot().Owner != after.Owner {
		t.Fatal("older generation reinstalled its credentials", e)
	}
}

func TestRefreshWorkerFailsClosedAndRecoversWithoutReload(t *testing.T) {
	cfg, leaf := supplied(t)
	cfg.Lifetime = 3 * time.Second
	var fail atomic.Bool
	var attempts atomic.Int32
	ops := store.Operations{Sign: func(mat *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
		attempts.Add(1)
		if fail.Load() {
			return "", binding.Header{}, fmt.Errorf("injected refresh storage outage")
		}
		return binding.Sign(mat, cs, now, lifetime)
	}}
	ctx, cancel := newContext()
	defer cancel()
	h := &plugin.Handler{Binding: cfg}
	if e := h.ProvisionWithOperations(ctx, ops); e != nil {
		t.Fatal(e)
	}
	defer h.Cleanup()
	l, e := plugin.Acquire(ctx, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release()
	l.SetStoredTLS(leaf)
	if e = l.Activate(); e != nil {
		t.Fatal(e)
	}
	before := l.Manager().Snapshot()
	fail.Store(true)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Unix() < before.Expires; {
		if time.Now().After(deadline) {
			t.Fatal("initial binding never expired")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, e = l.Manager().CurrentDocument(); e == nil {
		t.Fatal("expired document remained available during refresh failure")
	}
	w := httptest.NewRecorder()
	e = h.ServeHTTP(w, httptest.NewRequest("GET", "https://"+cfg.Host+"/", nil), caddyhttp.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(200)
		return nil
	}))
	if e == nil {
		t.Fatal("protected response accepted expired binding")
	}
	if after := l.Manager().Snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("failed refresh changed committed state")
	}
	if attempts.Load() < 2 || attempts.Load() > 5 {
		t.Fatal("worker did not retry with bounded backoff", attempts.Load())
	}
	fail.Store(false)
	for deadline := time.Now().Add(8 * time.Second); ; {
		if after := l.Manager().Snapshot(); after.Published > before.Published && after.Expires > time.Now().Unix() {
			if _, e = l.Manager().CurrentDocument(); e != nil {
				t.Fatal(e)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not recover after outage without reload")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestConcurrentReloadPublicationRefreshCollectionAndReaders(t *testing.T) {
	cfg, first := supplied(t)
	cfg.Lifetime = 3 * time.Second
	cfg.DocumentGrace = time.Second
	ctx, cancel := newContext()
	defer cancel()
	l, e := plugin.Acquire(ctx, cfg, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release()
	l.SetStoredTLS(first)
	if e = l.Activate(); e != nil {
		t.Fatal(e)
	}
	root, _ := pki.ReadCert(cfg.TrustFiles[0])
	ca, _ := pki.ReadCert(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root.pem"))
	caKey, _ := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root-key.pem"))
	var next []*x509.Certificate
	for range 6 {
		tmpl := *first
		tmpl.SerialNumber = pki.Serial()
		cert, e := pki.Issue(&tmpl, ca, first.PublicKey.(*rsa.PublicKey), caKey)
		if e != nil {
			t.Fatal(e)
		}
		next = append(next, cert)
	}
	errors := make(chan error, 32)
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			for range 25 {
				doc, e := l.Manager().CurrentDocument()
				if e == nil {
					e = binding.Verify(string(doc), cfg.Host, []*x509.Certificate{first}, root, time.Now())
				}
				if e != nil {
					errors <- fmt.Errorf("concurrent reader: %w", e)
					return
				}
			}
		})
	}
	group.Go(func() {
		for _, cert := range next {
			if e := l.Publish(cert); e != nil {
				errors <- e
				return
			}
		}
	})
	group.Go(func() {
		for range 25 {
			if e := l.Manager().Refresh(); e != nil {
				errors <- e
				return
			}
		}
	})
	group.Go(func() {
		for range 25 {
			if _, e := l.Manager().Collect(time.Now().Add(time.Hour)); e != nil {
				errors <- e
				return
			}
		}
	})
	for _, owner := range []string{"Concurrent second TEST", "Concurrent final TEST"} {
		replaceLeaf(t, cfg, owner)
		newCtx, stop := newContext()
		defer stop()
		replacement, e := plugin.Acquire(newCtx, cfg, store.Operations{})
		if e != nil {
			t.Fatal(e)
		}
		defer replacement.Release()
		if e = replacement.Activate(); e != nil {
			t.Fatal(e)
		}
	}
	group.Wait()
	close(errors)
	for e := range errors {
		t.Error(e)
	}
	after := l.Manager().Snapshot()
	if after.Owner.Organization != "Concurrent final TEST" || len(after.IDs) != len(next)+1 {
		t.Fatal("reload or renewal lost committed signer/coverage", after)
	}
	if e := l.Activate(); e != nil || l.Manager().Snapshot().Owner != after.Owner {
		t.Fatal("old generation rolled back concurrent replacement", e)
	}
	if _, e := l.Manager().CurrentDocument(); e != nil {
		t.Fatal(e)
	}
	if _, e := l.Manager().Document(after.URL); e != nil {
		t.Fatal("collection removed committed artifact", e)
	}
}

func TestCachedDiscoveryOutlivesExpiryRenewalAndReplacement(t *testing.T) {
	cfg, leaf := supplied(t)
	cfg.Lifetime = 3 * time.Second
	cfg.DocumentGrace = 2 * time.Second
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
	l.SetStoredTLS(leaf)
	if e = l.Activate(); e != nil {
		t.Fatal(e)
	}
	first := l.Manager().Snapshot()
	pkidir := filepath.Dir(cfg.KeyFile)
	key, e := pki.ReadKey(filepath.Join(pkidir, "tls-leaf-key.pem"))
	if e != nil {
		t.Fatal(e)
	}
	var served atomic.Pointer[tls.Certificate]
	served.Store(&tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key})
	var proxyURL string
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cache sits on a separate real HTTP hop. This route has no binding
		// middleware: its Link can only come from the cached origin response.
		if r.URL.Path == "/cached" {
			res, e := http.Get(proxyURL)
			if e != nil {
				http.Error(w, e.Error(), 502)
				return
			}
			defer res.Body.Close()
			for name, values := range res.Header {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.WriteHeader(res.StatusCode)
			_, _ = io.Copy(w, res.Body)
			return
		}
		e := h.ServeHTTP(w, r, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "public, max-age=120")
			w.Header().Add("Link", `</style>; rel="preload"`)
			_, e := io.WriteString(w, "cached origin body")
			return e
		}))
		if e != nil {
			http.Error(w, e.Error(), 503)
		}
	}))
	front.TLS = &tls.Config{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return served.Load(), nil }}
	front.StartTLS()
	defer front.Close()
	root, e := pki.ReadCert(filepath.Join(pkidir, "tls-root.pem"))
	if e != nil {
		t.Fatal(e)
	}
	pool := x509.NewCertPool()
	pool.AddCert(root)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, front.Listener.Addr().String())
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	url := "https://" + cfg.Host + ":" + strings.Split(front.Listener.Addr().String(), ":")[1]
	cache := &cacheproxy.Proxy{Origin: url + "/origin", Client: client}
	intermediary := httptest.NewServer(cache)
	defer intermediary.Close()
	proxyURL = intermediary.URL
	fetch := func(path string) (*http.Response, []byte) {
		t.Helper()
		res, e := client.Get(url + path)
		if e != nil {
			t.Fatal(e)
		}
		d, e := io.ReadAll(res.Body)
		res.Body.Close()
		if e != nil {
			t.Fatal(e)
		}
		return res, d
	}
	cached, body := fetch("/cached")
	links := strings.Join(cached.Header.Values("Link"), ",")
	if cached.StatusCode != 200 || cached.Header.Get("X-Qwac-Cache") != "miss" || !strings.Contains(links, store.DiscoveryURL) {
		t.Fatal("response was not cached with stable discovery", cached)
	}
	// Outlive the original binding, with automatic worker refresh independent of
	// any application response. The intermediary entry remains unchanged.
	time.Sleep(time.Until(time.Unix(first.Expires+1, 0)))
	refreshed := l.Manager().Snapshot()
	if refreshed.Published <= first.Published || refreshed.Expires <= first.Expires {
		t.Fatal("automatic refresh did not advance signed times", refreshed)
	}
	// Gate publication before activating a distinct server certificate.
	caKey, _ := pki.ReadKey(filepath.Join(pkidir, "tls-root-key.pem"))
	tmpl := *leaf
	tmpl.SerialNumber = pki.Serial()
	renewed, e := pki.Issue(&tmpl, root, leaf.PublicKey.(*rsa.PublicKey), caKey)
	if e != nil {
		t.Fatal(e)
	}
	if e = l.Publish(renewed); e != nil {
		t.Fatal(e)
	}
	served.Store(&tls.Certificate{Certificate: [][]byte{renewed.Raw}, PrivateKey: key})
	transport.CloseIdleConnections()
	qwacRoot, _ := pki.ReadCert(cfg.TrustFiles[0])
	check := func() {
		t.Helper()
		res, d := fetch("/cached")
		if res.StatusCode != 200 || string(d) != string(body) || strings.Join(res.Header.Values("Link"), ",") != links || res.Header.Get("Cache-Control") != "public, max-age=120" || res.Header.Get("X-Qwac-Cache") != "hit" {
			t.Fatal("cache entry was rewritten or bypassed", res)
		}
		_, doc := fetch(store.DiscoveryURL)
		if e := binding.Verify(string(doc), cfg.Host, []*x509.Certificate{res.TLS.PeerCertificates[0], leaf}, qwacRoot, time.Now()); e != nil {
			t.Fatal("cached Link did not resolve current connection and held-leaf coverage", e)
		}
		if binding.ID(res.TLS.PeerCertificates[0]) != binding.ID(renewed) {
			t.Fatal("replacement server certificate not activated")
		}
	}
	check()
	replaceLeaf(t, cfg, "Cache replacement TEST")
	newCtx, stop := newContext()
	defer stop()
	replacement, e := plugin.Acquire(newCtx, cfg, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	defer replacement.Release()
	replacement.SetStoredTLS(renewed)
	if e = replacement.Activate(); e != nil {
		t.Fatal(e)
	}
	check()
	if _, hits := cache.Counts(); hits != 2 {
		t.Fatal("real intermediary cache hits not proved", hits)
	}
}

func TestPublicationFailuresPreserveLiveAndDurableCommit(t *testing.T) {
	cfg, tls := supplied(t)
	mat := loadSupplied(t, cfg)
	for _, boundary := range []string{"sign", "verify", "document", "document-corruption", "manifest", "manifest-after-rename"} {
		t.Run(boundary, func(t *testing.T) {
			local := cfg.Config
			local.Dir = filepath.Join(t.TempDir(), "bindings")
			var fail atomic.Bool
			ops := store.Operations{
				Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
					if fail.Load() && boundary == "sign" {
						return "", binding.Header{}, fmt.Errorf("sign failure")
					}
					doc, h, e := binding.Sign(m, cs, now, lifetime)
					if fail.Load() && boundary == "verify" {
						d := []byte(doc)
						d[len(d)-5] ^= 1
						doc = string(d)
					}
					return doc, h, e
				},
				WriteDocument: func(path string, d []byte) error {
					if fail.Load() && boundary == "document" {
						return fmt.Errorf("write failure")
					}
					if fail.Load() && boundary == "document-corruption" {
						d = []byte("corrupt durable write")
					}
					return store.Atomic(path, d)
				},
				CommitManifest: func(path string, d []byte, _ bool) error {
					if fail.Load() && boundary == "manifest" {
						return fmt.Errorf("manifest failure")
					}
					if e := store.Atomic(path, d); e != nil {
						return e
					}
					if fail.Load() && boundary == "manifest-after-rename" {
						return fmt.Errorf("directory sync failure after rename")
					}
					return nil
				},
			}
			m, e := store.Open(local, mat, ops)
			if e != nil {
				t.Fatal(e)
			}
			if e = m.Publish(tls); e != nil {
				t.Fatal(e)
			}
			before := m.Snapshot()
			manifestPath := filepath.Join(local.Dir, "current.json")
			manifest, _ := os.ReadFile(manifestPath)
			// A distinct stored leaf forces a complete-set publication.
			root, _ := pki.ReadCert(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root.pem"))
			key, _ := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root-key.pem"))
			tmpl := *tls
			tmpl.SerialNumber = pki.Serial()
			next, e := pki.Issue(&tmpl, root, tls.PublicKey.(*rsa.PublicKey), key)
			if e != nil {
				t.Fatal(e)
			}
			fail.Store(true)
			if e = m.Activate(mat, next); e == nil {
				t.Fatal("boundary failure accepted")
			}
			afterManifest, _ := os.ReadFile(manifestPath)
			if !reflect.DeepEqual(m.Snapshot(), before) || string(afterManifest) != string(manifest) {
				t.Fatal("failure changed live or durable commit")
			}
			if _, e = m.CurrentDocument(); e != nil {
				t.Fatal("prior live binding unavailable", e)
			}
			recovered, e := store.Recover(local, mat, store.Operations{})
			if e != nil || !reflect.DeepEqual(recovered.Snapshot(), before) {
				t.Fatal("failure broke recovery", e)
			}
			fail.Store(false)
			if e = m.Activate(mat, next); e != nil || len(m.Snapshot().IDs) != 2 {
				t.Fatal("failure did not recover", e)
			}
		})
	}
}

func TestPublicationCrashBoundary(t *testing.T) {
	if boundary := os.Getenv("QWAC_TEST_CRASH_BOUNDARY"); boundary != "" {
		var cfg plugin.BindingConfig
		if e := json.Unmarshal([]byte(os.Getenv("QWAC_TEST_CRASH_CONFIG")), &cfg); e != nil {
			t.Fatal(e)
		}
		mat := loadSupplied(t, cfg)
		ops := store.Operations{
			WriteDocument: func(path string, d []byte) error {
				if e := store.Atomic(path, d); e != nil {
					return e
				}
				if boundary == "document" {
					os.Exit(73)
				}
				return nil
			},
			CommitManifest: func(path string, d []byte, _ bool) error {
				if boundary == "before-manifest" {
					os.Exit(73)
				}
				if e := store.Atomic(path, d); e != nil {
					return e
				}
				os.Exit(74)
				return nil
			},
		}
		m, e := store.Recover(cfg.Config, mat, ops)
		if e != nil {
			t.Fatal(e)
		}
		next, e := pki.ReadCert(os.Getenv("QWAC_TEST_CRASH_TLS"))
		if e != nil {
			t.Fatal(e)
		}
		if e = m.Activate(mat, next); e != nil {
			t.Fatal(e)
		}
		t.Fatal("crash boundary not reached")
	}
	cfg, tls := supplied(t)
	mat := loadSupplied(t, cfg)
	root, _ := pki.ReadCert(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root.pem"))
	key, _ := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root-key.pem"))
	tmpl := *tls
	tmpl.SerialNumber = pki.Serial()
	next, e := pki.Issue(&tmpl, root, tls.PublicKey.(*rsa.PublicKey), key)
	if e != nil {
		t.Fatal(e)
	}
	nextPath := filepath.Join(t.TempDir(), "next.pem")
	if e = pki.WriteCert(nextPath, next); e != nil {
		t.Fatal(e)
	}
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	for _, boundary := range []string{"document", "before-manifest", "after-manifest"} {
		t.Run(boundary, func(t *testing.T) {
			local := cfg
			local.Dir = filepath.Join(t.TempDir(), "bindings")
			m, e := store.Open(local.Config, mat, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.Publish(tls); e != nil {
				t.Fatal(e)
			}
			before := m.Snapshot()
			d, _ := json.Marshal(local)
			cmd := exec.Command(executable, "-test.run=^TestPublicationCrashBoundary$")
			cmd.Env = append(os.Environ(), "QWAC_TEST_CRASH_BOUNDARY="+boundary, "QWAC_TEST_CRASH_CONFIG="+string(d), "QWAC_TEST_CRASH_TLS="+nextPath)
			out, e := cmd.CombinedOutput()
			want := 73
			if boundary == "after-manifest" {
				want = 74
			}
			if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != want {
				t.Fatalf("crash boundary not proved: %v %s", e, out)
			}
			recovered, e := store.Recover(local.Config, mat, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			wantIDs := before.IDs
			if boundary == "after-manifest" {
				wantIDs = append(wantIDs, binding.ID(next))
			}
			if !reflect.DeepEqual(recovered.Snapshot().IDs, wantIDs) {
				t.Fatal("restart adopted wrong side of durable commit", boundary, recovered.Snapshot().IDs)
			}
			if _, e = recovered.CurrentDocument(); e != nil {
				t.Fatal("crash recovery binding invalid", e)
			}
		})
	}
}

func loadSupplied(t *testing.T, cfg plugin.BindingConfig) *credentials.Credentials {
	t.Helper()
	m, e := credentials.Load(append(append([]string(nil), cfg.ChainFiles...), cfg.TrustFiles...), cfg.KeyFile)
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func TestCollectionAuthenticatesOwnershipProtectsGraceAndCurrent(t *testing.T) {
	cfg, tls := supplied(t)
	cfg.DocumentGrace = 3 * time.Second
	cfg.Lifetime = 6 * time.Second
	mat := loadSupplied(t, cfg)
	var reject atomic.Bool
	m, e := store.Open(cfg.Config, mat, store.Operations{CommitManifest: func(path string, d []byte, _ bool) error {
		if reject.Load() {
			return fmt.Errorf("manifest refused")
		}
		return store.Atomic(path, d)
	}})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Publish(tls); e != nil {
		t.Fatal(e)
	}
	old := m.Snapshot()
	replaceLeaf(t, cfg, "Collection current TEST")
	if e = m.Activate(loadSupplied(t, cfg)); e != nil {
		t.Fatal(e)
	}
	current := m.Snapshot()
	// A verified orphan includes a TLS leaf absent from the committed manifest.
	root, _ := pki.ReadCert(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root.pem"))
	key, _ := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root-key.pem"))
	tmpl := *tls
	tmpl.SerialNumber = pki.Serial()
	next, e := pki.Issue(&tmpl, root, tls.PublicKey.(*rsa.PublicKey), key)
	if e != nil {
		t.Fatal(e)
	}
	reject.Store(true)
	if e = m.Publish(next); e == nil {
		t.Fatal("failed manifest accepted")
	}
	if !reflect.DeepEqual(m.Snapshot(), current) {
		t.Fatal("orphan became current")
	}
	collectAt := time.Now().Add(time.Hour)
	// Authenticated documents remain protected by signed expiry OR modification
	// grace, regardless of age under the other criterion.
	write := func(doc string) string {
		sum := sha256.Sum256([]byte(doc))
		name := hex.EncodeToString(sum[:]) + ".jose"
		if e := os.WriteFile(filepath.Join(cfg.Dir, name), []byte(doc), 0600); e != nil {
			t.Fatal(e)
		}
		return name
	}
	future, _, e := binding.Sign(mat, []*x509.Certificate{tls}, time.Now(), 2*time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	futureName := write(future)
	graceDoc, _, e := binding.Sign(mat, []*x509.Certificate{tls}, time.Now().Add(time.Second), 6*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	graceName := write(graceDoc)
	gracePath := filepath.Join(cfg.Dir, graceName)
	if e = os.Chtimes(gracePath, collectAt, collectAt.Add(-time.Second)); e != nil {
		t.Fatal(e)
	}
	corrupt := []byte(future)
	corrupt[len(corrupt)-5] ^= 1
	corruptName := write(string(corrupt))
	unknownName := strings.Repeat("a", 64) + ".jose"
	if e = os.WriteFile(filepath.Join(cfg.Dir, unknownName), []byte("unknown"), 0600); e != nil {
		t.Fatal(e)
	}
	stage := filepath.Join(cfg.Dir, ".atomic-staged")
	if e = os.WriteFile(stage, []byte("staged"), 0600); e != nil {
		t.Fatal(e)
	}
	symlink := filepath.Join(cfg.Dir, strings.Repeat("b", 64)+".jose")
	if e = os.Symlink(filepath.Join(cfg.Dir, filepath.Base(old.URL)), symlink); e != nil {
		t.Fatal(e)
	}
	result, e := m.Collect(collectAt)
	if e != nil || result.Documents != 2 || result.Bytes <= 0 {
		t.Fatal("old and orphan should be collected", result, e)
	}
	for _, name := range []string{filepath.Base(current.URL), futureName, graceName, corruptName, unknownName, filepath.Base(stage), filepath.Base(symlink)} {
		if _, e := os.Lstat(filepath.Join(cfg.Dir, name)); e != nil {
			t.Fatal("protected or unowned file removed", name, e)
		}
	}
	if _, e := os.Stat(filepath.Join(cfg.Dir, filepath.Base(old.URL))); !os.IsNotExist(e) {
		t.Fatal("old artifact retained past grace", e)
	}
	if !reflect.DeepEqual(m.Snapshot().IDs, current.IDs) {
		t.Fatal("document collection pruned TLS coverage")
	}
	recovered, e := store.Recover(cfg.Config, loadSupplied(t, cfg), store.Operations{})
	if e != nil || !reflect.DeepEqual(recovered.Snapshot(), current) {
		t.Fatal("collection broke restart recovery", e)
	}
}

func TestCapacitiesRefuseAdditionsWithoutPruningAndMigrateLegacyState(t *testing.T) {
	cfg, tls := supplied(t)
	cfg.MaxCertificates = 1
	mat := loadSupplied(t, cfg)
	m, e := store.Open(cfg.Config, mat, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Publish(tls); e != nil {
		t.Fatal(e)
	}
	before := m.Snapshot()
	root, _ := pki.ReadCert(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root.pem"))
	key, _ := pki.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-root-key.pem"))
	tmpl := *tls
	tmpl.SerialNumber = pki.Serial()
	next, e := pki.Issue(&tmpl, root, tls.PublicKey.(*rsa.PublicKey), key)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Publish(next); e == nil || !strings.Contains(e.Error(), "drain") {
		t.Fatal("capacity failed open", e)
	}
	if !reflect.DeepEqual(m.Snapshot(), before) {
		t.Fatal("capacity pruned retained coverage")
	}
	status, e := m.Status()
	if e != nil || status.RetainedCertificates != 1 || status.QWACExpires == 0 || status.LastFailure == "" || status.StorageBytes < int64(status.DocumentBytes) {
		t.Fatal("operational diagnostics incomplete", status, e)
	}
	// Legacy manifests omitted discovery_url; their immutable recovery identity
	// and all certificate coverage survive the additive migration.
	legacy := before
	legacy.DiscoveryURL = ""
	d, _ := json.Marshal(legacy)
	if e = store.Atomic(filepath.Join(cfg.Dir, "current.json"), d); e != nil {
		t.Fatal(e)
	}
	recovered, e := store.Recover(cfg.Config, mat, store.Operations{})
	if e != nil || recovered.Snapshot().DiscoveryURL != store.DiscoveryURL || !reflect.DeepEqual(recovered.Snapshot().IDs, before.IDs) {
		t.Fatal("legacy recovery failed", e)
	}
	limited := cfg.Config
	limited.MaxDocumentBytes = int64(status.DocumentBytes) - 1
	limited.Dir = filepath.Join(t.TempDir(), "limited")
	m, e = store.Open(limited, mat, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Publish(tls); e == nil || !strings.Contains(e.Error(), "max_document_bytes") {
		t.Fatal("document capacity not enforced", e)
	}
	if files, e := os.ReadDir(limited.Dir); e != nil || len(files) != 0 {
		t.Fatal("oversized publication wrote state", files, e)
	}
}

func TestStableDiscoveryUsesOneCommittedVerifiedDocument(t *testing.T) {
	cfg, tls := supplied(t)
	mat, e := credentials.Load(append(append([]string(nil), cfg.ChainFiles...), cfg.TrustFiles...), cfg.KeyFile)
	if e != nil {
		t.Fatal(e)
	}
	m, e := store.Open(cfg.Config, mat, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.CurrentDocument(); e == nil {
		t.Fatal("empty current document served")
	}
	if e = m.Publish(tls); e != nil {
		t.Fatal(e)
	}
	first := m.Snapshot()
	doc, e := m.CurrentDocument()
	if e != nil {
		t.Fatal(e)
	}
	root, _ := pki.ReadCert(cfg.TrustFiles[0])
	if first.DiscoveryURL != store.DiscoveryURL || first.URL == first.DiscoveryURL || binding.Verify(string(doc), cfg.Host, []*x509.Certificate{tls}, root, time.Now()) != nil {
		t.Fatal("discovery must retain a separately identified immutable artifact")
	}
	doc[0] ^= 1
	copy, e := m.CurrentDocument()
	if e != nil || copy[0] == doc[0] {
		t.Fatal("caller mutated committed document", e)
	}
	replaceLeaf(t, cfg, "Discovery replacement TEST")
	mat, e = credentials.Load(append(append([]string(nil), cfg.ChainFiles...), cfg.TrustFiles...), cfg.KeyFile)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Activate(mat, tls); e != nil {
		t.Fatal(e)
	}
	second := m.Snapshot()
	doc, e = m.CurrentDocument()
	if e != nil || second.DiscoveryURL != first.DiscoveryURL || second.URL == first.URL || string(doc) == string(copy) {
		t.Fatal("stable discovery did not resolve new committed document", e)
	}
	if e = binding.Verify(string(doc), cfg.Host, []*x509.Certificate{tls}, root, time.Now()); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Document(first.URL); e != nil {
		t.Fatal("immutable historical document lost", e)
	}
}
