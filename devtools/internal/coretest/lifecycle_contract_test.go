package coretest

import (
	"bufio"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/certmagic"
	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/lifecycle"
	"github.com/cztamas/qwacamole/store"
)

func TestBindingValidityIncludesEntireQWACChain(t *testing.T) {
	for _, index := range []int{1, 2} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			cfg, leaf := supplied(t)
			mat, err := credentials.LoadFor(cfg.ChainFiles, cfg.KeyFile, cfg.TrustFiles, []string{cfg.Host}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(cfg.KeyFile)
			rootKey, err := pki.ReadKey(filepath.Join(dir, "qwac-root-key.pem"))
			if err != nil {
				t.Fatal(err)
			}
			intermediateKey, err := pki.ReadKey(filepath.Join(dir, "qwac-intermediate-key.pem"))
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().Truncate(time.Second)
			end := now.Add(20 * time.Second)
			template := *mat.Chain[index]
			template.NotAfter = end
			parent, key := mat.Chain[2], rootKey
			if index == 2 {
				parent = &template
			}
			changed, err := pki.Issue(&template, parent, template.PublicKey.(*rsa.PublicKey), key)
			if err != nil {
				t.Fatal(err)
			}
			mat.Chain[index] = changed
			if index == 2 { // Re-sign the intermediate under the shortened root.
				template = *mat.Chain[1]
				mat.Chain[1], err = pki.Issue(&template, changed, template.PublicKey.(*rsa.PublicKey), rootKey)
				if err != nil {
					t.Fatal(err)
				}
			}
			template = *mat.Chain[0]
			template.ExtraExtensions = template.Extensions
			mat.Chain[0], err = pki.Issue(&template, mat.Chain[1], &mat.Key.PublicKey, intermediateKey)
			if err != nil {
				t.Fatal(err)
			}
			tlsRoot, err := pki.ReadCert(filepath.Join(dir, "tls-root.pem"))
			if err != nil {
				t.Fatal(err)
			}
			tlsKey, err := pki.ReadKey(filepath.Join(dir, "tls-root-key.pem"))
			if err != nil {
				t.Fatal(err)
			}
			expiredTemplate := *leaf
			expiredTemplate.SerialNumber = pki.Serial()
			expiredTemplate.NotBefore = now.Add(-time.Minute)
			expiredTemplate.NotAfter = now.Add(-time.Second)
			expired, err := pki.Issue(&expiredTemplate, tlsRoot, leaf.PublicKey.(*rsa.PublicKey), tlsKey)
			if err != nil {
				t.Fatal(err)
			}
			for _, firefox := range []bool{false, true} {
				sign, verify := binding.Sign, binding.Verify
				if firefox {
					sign, verify = binding.SignFirefox, binding.VerifyFirefox
				}
				doc, h, err := sign(mat, []*x509.Certificate{expired, leaf}, now, time.Hour)
				if err != nil {
					t.Fatal(err)
				}
				if h.Exp != end.Unix() {
					t.Fatal("full chain did not cap signed lifetime", h.Exp, end)
				}
				if err = verify(doc, cfg.Host, []*x509.Certificate{expired, leaf}, mat.Chain[2], now); err != nil {
					t.Fatal(err)
				}
				h.Exp++
				forged, _, err := binding.SignHeader(mat, h)
				if err != nil {
					t.Fatal(err)
				}
				if err = verify(forged, cfg.Host, []*x509.Certificate{leaf}, mat.Chain[2], now); err == nil || !strings.Contains(err.Error(), "QWAC chain maximum") {
					t.Fatal("independent verifier accepted chain-overrun", err)
				}
			}
			m, err := store.Open(store.Config{Dir: t.TempDir(), Host: cfg.Host, Lifetime: time.Hour}, mat, store.Operations{})
			if err != nil {
				t.Fatal(err)
			}
			if err = m.Publish(leaf); err != nil {
				t.Fatal(err)
			}
			status, err := m.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.QWACChainExpires != end.Unix() || status.SignerID != binding.ID(mat.Chain[0]) || len(status.TLSIDs) != 1 || status.NextRefresh >= end.Unix() {
				t.Fatal("diagnostics lost chain deadline/identity/coverage", status)
			}
		})
	}
}

func TestStoredCaddyTLSGatedBeforeHandshakeAndMaintainedWithoutTraffic(t *testing.T) {
	cfg, leaf := supplied(t)
	cfg.Lifetime = 3 * time.Second
	var fail atomic.Bool
	ops := store.Operations{CommitManifest: func(path string, d []byte, _ bool) error {
		if fail.Load() {
			return fmt.Errorf("stored coverage unavailable")
		}
		return store.Atomic(path, d)
	}}
	ctx, cancel := newContext()
	defer cancel()
	issuer := &plugin.Issuer{Binding: cfg}
	if err := issuer.ProvisionWithOperations(ctx, ops); err != nil {
		t.Fatal(err)
	}
	defer issuer.Cleanup()
	storage := &certmagic.FileStorage{Path: t.TempDir()}
	cm := &certmagic.Config{Storage: storage, Issuers: []certmagic.Issuer{issuer}}
	issuer.ACMEIssuer.SetConfig(cm)
	// SetConfig needs the configured issuer key but must not publish during provisioning.
	name := certmagic.StorageKeys.SiteCert(issuer.IssuerKey(), cfg.Host)
	if err := storage.Store(t.Context(), name, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})); err != nil {
		t.Fatal(err)
	}
	var forwarded atomic.Int32
	cm.OnEvent = func(context.Context, string, map[string]any) error { forwarded.Add(1); return nil }
	issuer.SetConfig(cm)
	if err := issuer.StartupError(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Dir, "current.json")); !os.IsNotExist(err) {
		t.Fatal("provisioning activated stored TLS", err)
	}
	// Certificate cache events are non-abortable. A failed publication must still
	// abort the subsequent ordinary handshake, while ACME TLS-ALPN remains usable.
	fail.Store(true)
	if err := cm.OnEvent(t.Context(), "cached_managed_cert", nil); err == nil {
		t.Fatal("stored publication failure ignored")
	}
	hello := map[string]any{"client_hello": struct{ SupportedProtos []string }{[]string{"h2"}}}
	if err := cm.OnEvent(t.Context(), "tls_get_certificate", hello); err == nil {
		t.Fatal("unbound stored certificate allowed through handshake")
	}
	hello["client_hello"] = struct{ SupportedProtos []string }{[]string{"acme-tls/1"}}
	if err := cm.OnEvent(t.Context(), "tls_get_certificate", hello); err != nil {
		t.Fatal("ACME validation blocked", err)
	}
	fail.Store(false)
	if err := cm.OnEvent(t.Context(), "cached_managed_cert", nil); err != nil {
		t.Fatal(err)
	}
	var before store.State
	d, err := os.ReadFile(filepath.Join(cfg.Dir, "current.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Observe durable state only: no HTTP request or handshake triggers refresh.
	if err = json.Unmarshal(d, &before); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		d, err = os.ReadFile(filepath.Join(cfg.Dir, "current.json"))
		if err != nil {
			t.Fatal(err)
		}
		var after store.State
		if err = json.Unmarshal(d, &after); err != nil {
			t.Fatal(err)
		}
		if after.Published > before.Published {
			if len(after.IDs) != 1 || after.IDs[0] != binding.ID(leaf) {
				t.Fatal(after)
			}
			break
		}
		if time.Now().After(end) {
			t.Fatal("quiet cached-certificate startup did not maintain bindings")
		}
	}
	if forwarded.Load() < 4 {
		t.Fatal("Caddy event forwarding lost")
	}
}

type hijackProbe struct {
	*httptest.ResponseRecorder
	called bool
}

func (w *hijackProbe) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.called = true
	return nil, nil, fmt.Errorf("unexpected underlying hijack")
}

func TestCaddyDelayedProtocolUpgradeRejectsAtCommit(t *testing.T) {
	cfg, leaf := supplied(t)
	cfg.Lifetime = 3 * time.Second
	var fail atomic.Bool
	ops := store.Operations{Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
		if fail.Load() {
			return "", binding.Header{}, fmt.Errorf("signing outage")
		}
		return binding.Sign(m, cs, now, lifetime)
	}}
	ctx, cancel := newContext()
	defer cancel()
	h := &plugin.Handler{Binding: cfg}
	if err := h.ProvisionWithOperations(ctx, ops); err != nil {
		t.Fatal(err)
	}
	defer h.Cleanup()
	l, err := plugin.Acquire(ctx, cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if err = l.Publish(leaf); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	end := l.Manager().Snapshot().Expires
	w := &hijackProbe{ResponseRecorder: httptest.NewRecorder()}
	err = h.ServeHTTP(w, httptest.NewRequest("GET", "https://"+cfg.Host+"/upgrade", nil), caddyhttp.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Set-Cookie", "protected=secret")
		w.Header().Set("Link", "</protected>; rel=preload")
		for time.Now().Unix() < end {
			time.Sleep(10 * time.Millisecond)
		}
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte("protected body")); err == nil {
			t.Fatal("rejected body write did not stop copying")
		}
		if _, err := w.(io.ReaderFrom).ReadFrom(strings.NewReader("protected copied body")); err == nil {
			t.Fatal("rejected copy did not stop reading")
		}
		_, _, err := http.NewResponseController(w).Hijack()
		return err
	}))
	if err == nil || w.called || w.Code != 503 || w.Body.Len() != 0 || w.Header().Get("Upgrade") != "" || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Link") != "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("expired upgrade escaped gate", err, w.called, w.Code, w.Header())
	}
}

func TestMaintenanceRetryDiagnosticsAndPublicationWake(t *testing.T) {
	cfg, leaf := supplied(t)
	cfg.Lifetime = 3 * time.Second
	mat, err := credentials.LoadFor(cfg.ChainFiles, cfg.KeyFile, cfg.TrustFiles, []string{cfg.Host}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var fail atomic.Bool
	ops := store.Operations{Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
		if fail.Load() {
			return "", binding.Header{}, fmt.Errorf("temporary signing failure")
		}
		return binding.Sign(m, cs, now, lifetime)
	}}
	m, err := store.Open(cfg.Config, mat, ops)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Publish(leaf); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); lifecycle.Run(ctx, m, nil) }()
	defer func() { cancel(); <-done }()
	fail.Store(true)
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		s, err := m.Status()
		if err != nil {
			t.Fatal(err)
		}
		if s.RefreshFailures > 0 {
			if s.RetryAt == 0 || s.RefreshFailure != "temporary signing failure" {
				t.Fatal(s)
			}
			break
		}
		if time.Now().After(end) {
			t.Fatal("retry state absent")
		}
	}
	fail.Store(false)
	if err = m.Activate(mat); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(time.Second); ; time.Sleep(10 * time.Millisecond) {
		s, err := m.Status()
		if err != nil {
			t.Fatal(err)
		}
		if s.RefreshFailures == 0 && s.RetryAt == 0 && s.RefreshFailure == "" {
			break
		}
		if time.Now().After(end) {
			t.Fatal("publication did not wake/reset maintenance", s)
		}
	}
}

func TestAgentConcurrentLifecycleAndReplacementDiagnostics(t *testing.T) {
	cfg, target := agentFixture(t)
	a, err := agent.Open(cfg, target, store.Operations{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	target.publications = a.Manager
	if err = a.Deploy(t.Context()); err != nil {
		t.Fatal(err)
	}
	before, err := a.Status()
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(cfg.KeyFile, []byte("malformed local key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = a.ReplaceCredentials(); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	failed, err := a.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !failed.Ready || !failed.ReplacementPending || failed.ReplacementError == "" || failed.Binding.SignerID != before.Binding.SignerID {
		t.Fatal("failed candidate not observable or changed active signer", failed)
	}
	if err = os.WriteFile(cfg.KeyFile, key, 0600); err != nil {
		t.Fatal(err)
	}
	candidate := renewAgentCandidate(t, cfg)
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for _, operation := range []func() error{
		func() error { return a.Deploy(t.Context()) },
		func() error { return a.Reconcile(t.Context()) },
		a.ReplaceCredentials,
		a.Manager.Refresh,
		func() error { _, err := a.Manager.Collect(time.Now().Add(time.Hour)); return err },
	} {
		wg.Go(func() {
			for range 5 {
				if err := operation(); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	after, err := a.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !after.Ready || after.ReplacementPending || after.ReplacementError != "" || after.Deployment.Active != candidate || len(after.Binding.TLSIDs) != 2 {
		t.Fatal("concurrent operations lost committed deployment/coverage", after)
	}
	for range 2 {
		if err = a.Reconcile(t.Context()); err != nil {
			t.Fatal("repeated recovery", err)
		}
	}
	if _, err = a.Manager.Document(a.Manager.Snapshot().URL); err != nil {
		t.Fatal("collection removed current document", err)
	}
}

func TestRecoveredStatusDistinguishesConfiguredAndCommittedSigner(t *testing.T) {
	cfg, leaf := supplied(t)
	mat, err := credentials.LoadFor(cfg.ChainFiles, cfg.KeyFile, cfg.TrustFiles, []string{cfg.Host}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m, err := store.Open(cfg.Config, mat, store.Operations{})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Publish(leaf); err != nil {
		t.Fatal(err)
	}
	before, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	replaceLeaf(t, cfg, "Pending restart replacement TEST")
	next, err := credentials.LoadFor(cfg.ChainFiles, cfg.KeyFile, cfg.TrustFiles, []string{cfg.Host}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Recover(cfg.Config, next, store.Operations{})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := recovered.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !pending.ReplacementPending || pending.SignerID != before.SignerID || pending.CredentialID != binding.ID(next.Chain[0]) {
		t.Fatal("configured inputs reported as activated signer", pending)
	}
	if err = recovered.Activate(next); err != nil {
		t.Fatal(err)
	}
	active, err := recovered.Status()
	if err != nil || active.ReplacementPending || active.SignerID != pending.CredentialID {
		t.Fatal(active, err)
	}
}
