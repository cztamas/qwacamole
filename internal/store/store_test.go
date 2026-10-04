package store

import (
	"crypto/rsa"
	"crypto/x509"
	"github.com/cztamas/qwacamole/internal/binding"
	"github.com/cztamas/qwacamole/internal/pki"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestAtomicRotationFailureRecoveryAndReaders(t *testing.T) {
	dir := t.TempDir()
	pkidir := filepath.Join(dir, "pki")
	if e := pki.Generate(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Synthetic TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); e != nil {
		t.Fatal(e)
	}
	cfg := Config{Dir: filepath.Join(dir, "bindings"), PKI: pkidir, Host: "qwac.example.com", Lifetime: 45 * time.Second}
	m, e := Open(cfg)
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
				if err = binding.Verify(string(doc), cfg.Host, []*x509.Certificate{old, next}, qwac, time.Now()); err != nil {
					t.Fatal("partially published candidate is not valid", err)
				}
			}
			recovered, err := Open(cfg)
			if err != nil || !reflect.DeepEqual(recovered.Snapshot().IDs, first.IDs) {
				t.Fatal("restart adopted an unadvertised candidate", err)
			}
		}
		_ = os.Remove(filepath.Join(cfg.Dir, flag))
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
	if e = binding.Verify(string(doc), cfg.Host, []*x509.Certificate{old, next}, qwac, time.Now()); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(cfg.Dir, filepath.Base(first.URL))); e != nil {
		t.Fatal("previous version removed")
	}
	recovered, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(recovered.Snapshot().IDs, second.IDs) {
		t.Fatal("restart lost certificate coverage")
	}
}
