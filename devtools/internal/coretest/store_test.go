package coretest

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	. "github.com/cztamas/qwacamole/store"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAtomicRotationFailureRecoveryAndReaders(t *testing.T) {
	t.Run("strict", func(t *testing.T) { testAtomicRotationFailureRecoveryAndReaders(t, false) })
	t.Run("firefox", func(t *testing.T) { testAtomicRotationFailureRecoveryAndReaders(t, true) })
}
func testAtomicRotationFailureRecoveryAndReaders(t *testing.T, firefox bool) {
	dir := t.TempDir()
	pkidir := filepath.Join(dir, "pki")
	generate := pki.Generate
	verify := binding.Verify
	if firefox {
		generate = pki.GenerateFirefox
		verify = binding.VerifyFirefox
	}
	if e := generate(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Synthetic TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); e != nil {
		t.Fatal(e)
	}
	cfg := Config{Dir: filepath.Join(dir, "bindings"), Host: "qwac.example.com", Lifetime: 45 * time.Second, FirefoxCompatibility: firefox}
	material, e := pki.Load(pkidir)
	if e != nil {
		t.Fatal(e)
	}
	ops := devtools.FaultOperations(plugin.BindingConfig{Config: cfg})
	m, e := Open(cfg, material, ops)
	if e != nil {
		t.Fatal(e)
	}
	old, e := pki.ReadCert(filepath.Join(pkidir, "tls-leaf.pem"))
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Publish(old); e != nil {
		t.Fatal(e)
	}
	first := m.Snapshot()
	manifest, e := os.ReadFile(filepath.Join(cfg.Dir, "current.json"))
	if e != nil {
		t.Fatal(e)
	}
	root, _ := pki.ReadCert(filepath.Join(pkidir, "tls-root.pem"))
	key, _ := pki.ReadKey(filepath.Join(pkidir, "tls-root-key.pem"))
	tplt := *old
	tplt.SerialNumber = pki.Serial()
	next, e := pki.Issue(&tplt, root, old.PublicKey.(*rsa.PublicKey), key)
	if e != nil {
		t.Fatal(e)
	}
	for _, flag := range []string{"fail-sign", "fail-publish"} {
		if e = os.WriteFile(filepath.Join(cfg.Dir, flag), nil, 0600); e != nil {
			t.Fatal(e)
		}
		if e = m.Publish(next); e == nil {
			t.Fatal("injected failure ignored")
		}
		if !reflect.DeepEqual(m.Snapshot(), first) {
			t.Fatal("failed publication mutated state")
		}
		persisted, err := os.ReadFile(filepath.Join(cfg.Dir, "current.json"))
		if err != nil || string(persisted) != string(manifest) {
			t.Fatal("failed publication changed durable current manifest", err)
		}
		documents, err := filepath.Glob(filepath.Join(cfg.Dir, "*.jose"))
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if flag == "fail-publish" {
			want = 2 // the candidate was signed, verified, and durably written
		}
		if len(documents) != want {
			t.Fatalf("%s: got %d durable documents, want %d", flag, len(documents), want)
		}
		if flag == "fail-publish" {
			for _, path := range documents {
				if filepath.Base(path) == filepath.Base(first.URL) {
					continue
				}
				doc, err := os.ReadFile(path)
				qwac, _ := pki.ReadCert(filepath.Join(pkidir, "qwac-root.pem"))
				if err != nil {
					t.Fatal(err)
				}
				if err = verify(string(doc), cfg.Host, []*x509.Certificate{old, next}, qwac, time.Now()); err != nil {
					t.Fatal("partially published candidate is not valid", err)
				}
			}
			recovered, err := Open(cfg, material, ops)
			if err != nil || !reflect.DeepEqual(recovered.Snapshot().IDs, first.IDs) {
				t.Fatal("restart adopted an unadvertised candidate", err)
			}
		}
		_ = os.Remove(filepath.Join(cfg.Dir, flag))
	}

	// An injected signer cannot bypass the gate's independent verification.
	rejectedConfig := cfg
	rejectedConfig.Dir = filepath.Join(dir, "rejected")
	invalidOps := Operations{Sign: func(mat *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
		sign := binding.Sign
		if firefox {
			sign = binding.SignFirefox
		}
		doc, h, err := sign(mat, cs, now, lifetime)
		if err != nil {
			return doc, h, err
		}
		parts := strings.Split(doc, ".")
		signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
		signature[0] ^= 1
		parts[2] = base64.RawURLEncoding.EncodeToString(signature)
		return strings.Join(parts, "."), h, nil
	}}
	rejected, err := Open(rejectedConfig, material, invalidOps)
	if err != nil {
		t.Fatal(err)
	}
	if err = rejected.Publish(old); err == nil {
		t.Fatal("unverified signer output was published")
	}
	if _, err = os.Stat(filepath.Join(rejectedConfig.Dir, "current.json")); !os.IsNotExist(err) {
		t.Fatal("invalid candidate committed", err)
	}
	documents, err := filepath.Glob(filepath.Join(rejectedConfig.Dir, "*.jose"))
	if err != nil || len(documents) != 0 {
		t.Fatal("invalid candidate made durable", err)
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Go(func() {
			for n := 0; n < 200; n++ {
				s := m.Snapshot()
				if s.URL == "" {
					t.Error("reader saw empty state")
				}
				if _, e := os.ReadFile(filepath.Join(cfg.Dir, filepath.Base(s.URL))); e != nil {
					t.Error("reader saw missing document", e)
				}
			}
		})
	}
	if e = m.Publish(next); e != nil {
		t.Fatal(e)
	}
	wg.Wait()
	second := m.Snapshot()
	if len(second.IDs) != 2 {
		t.Fatal("rotation lost overlap")
	}
	doc, e := os.ReadFile(filepath.Join(cfg.Dir, filepath.Base(second.URL)))
	if e != nil {
		t.Fatal(e)
	}
	qwac, _ := pki.ReadCert(filepath.Join(pkidir, "qwac-root.pem"))
	if e = verify(string(doc), cfg.Host, []*x509.Certificate{old, next}, qwac, time.Now()); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(cfg.Dir, filepath.Base(first.URL))); e != nil {
		t.Fatal("previous version removed")
	}
	recovered, e := Open(cfg, material, ops)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(recovered.Snapshot().IDs, second.IDs) {
		t.Fatal("restart lost certificate coverage")
	}
}

func TestExpiredTLSConnectionStaysBoundAfterRefreshAndRestart(t *testing.T) {
	dir := t.TempDir()
	pkidir := filepath.Join(dir, "pki")
	if err := pki.Generate(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Overlap TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Dir: filepath.Join(dir, "bindings"), Host: "qwac.example.com", Lifetime: 45 * time.Second}
	material, err := pki.Load(pkidir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Open(cfg, material, Operations{})
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := pki.ReadCert(filepath.Join(pkidir, "tls-leaf.pem"))
	root, _ := pki.ReadCert(filepath.Join(pkidir, "tls-root.pem"))
	key, _ := pki.ReadKey(filepath.Join(pkidir, "tls-root-key.pem"))
	now := time.Now().Truncate(time.Second)
	template := *leaf
	template.NotBefore, template.NotAfter = now.Add(-time.Minute), now.Add(2*time.Second)
	old, err := pki.Issue(&template, root, leaf.PublicKey.(*rsa.PublicKey), key)
	if err != nil {
		t.Fatal(err)
	}
	// Publish while the first certificate was valid; a replacement is then
	// issued, and the browser's original connection outlives the first leaf.
	if err := m.Publish(old); err != nil {
		t.Fatal(err)
	}
	if err := m.Publish(leaf); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(old.NotAfter.Add(time.Second))) // expire the original leaf
	now = time.Now()
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(cfg, material, Operations{})
	if err != nil {
		t.Fatal(err)
	}
	s := recovered.Snapshot()
	if !reflect.DeepEqual(s.IDs, []string{binding.ID(old), binding.ID(leaf)}) {
		t.Fatal("expired connection lost binding coverage", s.IDs)
	}
	doc, err := os.ReadFile(filepath.Join(cfg.Dir, filepath.Base(s.URL)))
	if err != nil {
		t.Fatal(err)
	}
	qwac, _ := pki.ReadCert(filepath.Join(pkidir, "qwac-root.pem"))
	if err := binding.Verify(string(doc), cfg.Host, []*x509.Certificate{old, leaf}, qwac, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := binding.Sign(material, []*x509.Certificate{old}, now, time.Minute); err == nil {
		t.Fatal("all-expired TLS set extended the binding lifetime")
	}
	pool := x509.NewCertPool()
	pool.AddCert(root)
	if _, err := old.Verify(x509.VerifyOptions{Roots: pool, DNSName: cfg.Host, CurrentTime: now}); err == nil {
		t.Fatal("expired leaf became valid for a new TLS handshake")
	}
}
