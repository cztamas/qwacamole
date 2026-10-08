package coretest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

type deploymentTarget struct {
	mu                       sync.Mutex
	cfg                      agent.Config
	served                   *x509.Certificate
	failValidate, failReload bool
	publications             *store.Manager
	reloads                  int
}

func (f *deploymentTarget) Validate(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failValidate {
		f.failValidate = false
		return fmt.Errorf("invalid nginx configuration")
	}
	return nil
}
func (f *deploymentTarget) Reload(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloads++
	if f.failReload {
		f.failReload = false
		return fmt.Errorf("reload failed")
	}
	p, e := os.Readlink(filepath.Join(f.cfg.StateDir, "enabled", "site.conf"))
	if os.IsNotExist(e) {
		f.served = nil
		return nil
	}
	if e != nil {
		return e
	}
	c, e := credentials.ReadCert(filepath.Join(filepath.Dir(p), "fullchain.pem"))
	if e != nil {
		return e
	}
	covered := false
	for _, id := range f.publications.Snapshot().IDs {
		if id == binding.ID(c) {
			covered = true
		}
	}
	if !covered {
		return fmt.Errorf("attempted activation before durable binding")
	}
	f.served = c
	return nil
}
func (f *deploymentTarget) Probe(context.Context) (*x509.Certificate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.served == nil {
		return nil, fmt.Errorf("no TLS deployment")
	}
	return f.served, nil
}
func agentFixture(t *testing.T) (agent.Config, *deploymentTarget) {
	t.Helper()
	p, _ := supplied(t)
	dir := filepath.Dir(p.Dir)
	pkidir := filepath.Dir(p.KeyFile)
	runtimeDir, e := os.MkdirTemp("", "qwac-test-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(runtimeDir) })
	cfg := agent.Config{RuntimeDir: runtimeDir, Host: p.Host, StateDir: filepath.Join(dir, "agent"), ChainFiles: append(p.ChainFiles, p.TrustFiles[0]), KeyFile: p.KeyFile, TrustFile: p.TrustFiles[0], TLSRoots: filepath.Join(pkidir, "tls-root.pem"), CertbotLineage: filepath.Join(dir, "lineage"), PublicListen: "127.0.0.1:18081", HTTPListen: "127.0.0.1:18080", HTTPSListen: "127.0.0.1:18443", ProbeAddress: "127.0.0.1:18443", Application: "http://127.0.0.1:18082", Webroot: filepath.Join(dir, "webroot"), NginxBinary: "/not/executed/nginx", NginxConfig: filepath.Join(dir, "nginx.conf"), NginxPrefix: filepath.Join(dir, "nginx"), Lifetime: "1m"}
	if e := os.MkdirAll(cfg.CertbotLineage, 0700); e != nil {
		t.Fatal(e)
	}
	chain, e := os.ReadFile(filepath.Join(pkidir, "tls-leaf.pem"))
	if e != nil {
		t.Fatal(e)
	}
	intermediate, e := os.ReadFile(filepath.Join(pkidir, "tls-root.pem"))
	if e != nil {
		t.Fatal(e)
	}
	key, e := os.ReadFile(filepath.Join(pkidir, "tls-leaf-key.pem"))
	if e != nil {
		t.Fatal(e)
	}
	for n, d := range map[string][]byte{"fullchain.pem": append(chain, intermediate...), "privkey.pem": key} {
		if e = os.WriteFile(filepath.Join(cfg.CertbotLineage, n), d, 0600); e != nil {
			t.Fatal(e)
		}
	}
	return cfg, &deploymentTarget{cfg: cfg}
}

// Renew under the same ordinary TLS authority, using a genuinely different key
// and leaf; rollback tests must distinguish old and candidate deployments.
func renewAgentCandidate(t *testing.T, c agent.Config) string {
	return issueAgentCandidate(t, c, time.Time{})
}
func issueAgentCandidate(t *testing.T, c agent.Config, expires time.Time) string {
	t.Helper()
	pkidir := filepath.Dir(c.KeyFile)
	old, e := credentials.ReadCert(filepath.Join(c.CertbotLineage, "fullchain.pem"))
	if e != nil {
		t.Fatal(e)
	}
	root, e := credentials.ReadCert(c.TLSRoots)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := credentials.ReadKey(filepath.Join(pkidir, "tls-root-key.pem"))
	if e != nil {
		t.Fatal(e)
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := *old
	template.SerialNumber = pki.Serial()
	if !expires.IsZero() {
		template.NotAfter = expires
	}
	leaf, e := pki.Issue(&template, root, &key.PublicKey, signer)
	if e != nil {
		t.Fatal(e)
	}
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw})...)
	if e = os.WriteFile(filepath.Join(c.CertbotLineage, "fullchain.pem"), chain, 0600); e != nil {
		t.Fatal(e)
	}
	if e = pki.WriteKey(filepath.Join(c.CertbotLineage, "privkey.pem"), key); e != nil {
		t.Fatal(e)
	}
	return binding.ID(leaf)
}

func gateCode(a *agent.Agent) int {
	w := httptest.NewRecorder()
	a.PublicHandler().ServeHTTP(w, httptest.NewRequest("GET", "http://local/gate", nil))
	return w.Code
}
func TestAgentDurableDeploymentRestartAndFailureRollback(t *testing.T) {
	cfg, target := agentFixture(t)
	ctx := context.Background()
	a, e := agent.Open(cfg, target, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if a != nil {
			a.Close()
		}
	}()
	target.publications = a.Manager
	if gateCode(a) != 403 {
		t.Fatal("empty deployment gate open")
	}
	if e = a.Reconcile(ctx); e != nil {
		t.Fatal(e)
	}
	if e = a.Deploy(ctx); e != nil {
		t.Fatal(e)
	}
	status, e := a.Status()
	if e != nil {
		t.Fatal(e)
	}
	if !status.Ready || status.Deployment.Phase != "committed" || gateCode(a) != 204 {
		t.Fatal(status)
	}
	if _, e = agent.Open(cfg, target, store.Operations{}); e == nil {
		t.Fatal("duplicate deployment owner")
	}
	original := status.Deployment.Active
	for _, failure := range []string{"validate", "reload"} {
		candidate := renewAgentCandidate(t, cfg)
		if candidate == original {
			t.Fatal("candidate must differ from old leaf")
		}
		if failure == "validate" {
			target.failValidate = true
		} else {
			target.failReload = true
		}
		if e = a.Deploy(ctx); e == nil {
			t.Fatal("deployment failure hidden")
		}
		status, e = a.Status()
		if e != nil {
			t.Fatal(e)
		}
		if !status.Ready || status.Deployment.Active != original || status.Deployment.Phase != "rolled_back" || status.Deployment.LastError == "" {
			t.Fatal("failed deployment lost prior state", status)
		}
		if served, _ := target.Probe(ctx); binding.ID(served) != original {
			t.Fatal("rollback did not restore served leaf")
		}
		if len(a.Manager.Snapshot().IDs) < 2 {
			t.Fatal("candidate coverage was pruned on rollback")
		}
	}
	if e = a.Deploy(ctx); e != nil {
		t.Fatal("safe retry failed", e)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	a = nil
	reopened, e := agent.Open(cfg, target, store.Operations{})
	if e != nil {
		t.Fatal(e)
	}
	a = reopened
	target.publications = a.Manager
	if gateCode(a) != 403 {
		t.Fatal("restart declared readiness before reconcile")
	}
	if e = a.Reconcile(ctx); e != nil {
		t.Fatal(e)
	}
	if gateCode(a) != 204 {
		t.Fatal("confirmed restart unavailable")
	}
}
func TestAgentPublicationFailureNeverStagesOrReloads(t *testing.T) {
	for _, boundary := range []string{"sign", "document", "manifest"} {
		t.Run(boundary, func(t *testing.T) {
			cfg, target := agentFixture(t)
			ops := store.Operations{}
			switch boundary {
			case "sign":
				ops.Sign = func(*credentials.Credentials, []*x509.Certificate, time.Time, time.Duration, bool) (string, binding.Header, error) {
					return "", binding.Header{}, fmt.Errorf("sign unavailable")
				}
			case "document":
				ops.WriteDocument = func(string, []byte) error { return fmt.Errorf("disk full") }
			case "manifest":
				ops.CommitManifest = func(string, []byte, bool) error { return fmt.Errorf("manifest unavailable") }
			}
			a, e := agent.Open(cfg, target, ops)
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			target.publications = a.Manager
			if e = a.Deploy(context.Background()); e == nil {
				t.Fatal("publication failure hidden")
			}
			if target.reloads != 0 || gateCode(a) != 403 {
				t.Fatal("activation before publication")
			}
			files, e := os.ReadDir(filepath.Join(cfg.StateDir, "generations"))
			if e != nil || len(files) != 0 {
				t.Fatal("TLS staged before binding succeeded", files, e)
			}
		})
	}
}
func TestAgentInterruptedJournalReconcilesServedState(t *testing.T) {
	for _, phase := range []string{"prepared", "bound", "activating", "reloading", "confirming"} {
		t.Run(phase, func(t *testing.T) {
			cfg, target := agentFixture(t)
			ctx := context.Background()
			a, e := agent.Open(cfg, target, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			target.publications = a.Manager
			if e = a.Deploy(ctx); e != nil {
				t.Fatal(e)
			}
			status, e := a.Status()
			if e != nil {
				t.Fatal(e)
			}
			j := status.Deployment
			j.Phase = phase
			j.Previous = j.Active
			data, _ := json.Marshal(j)
			if e = store.Atomic(filepath.Join(cfg.StateDir, "deployment.json"), data); e != nil {
				t.Fatal(e)
			}
			a.Close()
			a, e = agent.Open(cfg, target, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			target.publications = a.Manager
			if gateCode(a) != 403 {
				t.Fatal("unreconciled deployment ready")
			}
			if e = a.Reconcile(ctx); e != nil {
				t.Fatal(e)
			}
			got, e := a.Status()
			if e != nil || !got.Ready || got.Deployment.Active != j.Active {
				t.Fatal(got, e)
			}
		})
	}
}

func TestAgentExpiredDeploymentCanRecoverWithoutPruning(t *testing.T) {
	for _, phase := range []string{"committed", "prepared"} {
		t.Run(phase, func(t *testing.T) {
			cfg, target := agentFixture(t)
			cfg.Lifetime = "3s"
			target.cfg = cfg
			expires := time.Now().Add(4 * time.Second).Truncate(time.Second)
			old := issueAgentCandidate(t, cfg, expires)
			a, e := agent.Open(cfg, target, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			target.publications = a.Manager
			if e = a.Deploy(context.Background()); e != nil {
				a.Close()
				t.Fatal(e)
			}
			status, _ := a.Status()
			fresh := issueAgentCandidate(t, cfg, time.Now().Add(time.Hour))
			if phase == "prepared" {
				j := status.Deployment
				j.Phase, j.Previous, j.Candidate = phase, old, fresh
				d, _ := json.Marshal(j)
				if e = store.Atomic(filepath.Join(cfg.StateDir, "deployment.json"), d); e != nil {
					a.Close()
					t.Fatal(e)
				}
			}
			time.Sleep(time.Until(expires.Add(100 * time.Millisecond)))
			if gateCode(a) != 403 {
				a.Close()
				t.Fatal("expired binding gate is open")
			}
			w := httptest.NewRecorder()
			a.PublicHandler().ServeHTTP(w, httptest.NewRequest("GET", "http://local/bindings/current.jose", nil))
			if w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
				a.Close()
				t.Fatal("expired binding was delivered", w.Code)
			}
			a.Close()
			a, e = agent.Open(cfg, target, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			target.publications = a.Manager
			if e = a.Reconcile(context.Background()); e == nil {
				t.Fatal("expired TLS incorrectly reconciled as usable")
			}
			status, _ = a.Status()
			if status.Ready || status.Deployment.Phase != "needs_deploy" || gateCode(a) != 403 {
				t.Fatal(status)
			}
			// A failed activation cannot claim the expired prior leaf is usable.
			target.failValidate = true
			if e = a.Deploy(context.Background()); e == nil {
				t.Fatal("invalid config accepted")
			}
			status, _ = a.Status()
			if status.Ready || status.Deployment.Phase != "needs_deploy" {
				t.Fatal(status)
			}
			if e = a.Deploy(context.Background()); e != nil {
				t.Fatal("fresh deployment trapped behind expired intent", e)
			}
			status, _ = a.Status()
			if !status.Ready || status.Deployment.Active != fresh || len(a.Manager.Snapshot().IDs) != 2 {
				t.Fatal("fresh deployment lost expired held coverage", status)
			}
		})
	}
}
