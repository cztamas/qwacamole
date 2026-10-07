package coretest

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

// This development-only target persists what a server loaded independently of
// the agent's journal and selected include. The child exits without cleanup at
// actual deployment boundaries; restart must distinguish old and new leaves.
type crashTarget struct {
	cfg      agent.Config
	manager  *store.Manager
	boundary string
}

func (f *crashTarget) servedPath() string { return filepath.Join(f.cfg.StateDir, "test-server.pem") }
func (f *crashTarget) Validate(context.Context) error {
	if f.boundary == "selected-before-reload" {
		os.Exit(77)
	}
	return nil
}
func (f *crashTarget) Reload(context.Context) error {
	p, e := os.Readlink(filepath.Join(f.cfg.StateDir, "enabled", "site.conf"))
	if e != nil {
		return e
	}
	d, e := os.ReadFile(filepath.Join(filepath.Dir(p), "fullchain.pem"))
	if e != nil {
		return e
	}
	leaf, e := credentials.ReadCert(filepath.Join(filepath.Dir(p), "fullchain.pem"))
	if e != nil {
		return e
	}
	covered := false
	for _, id := range f.manager.Snapshot().IDs {
		if id == binding.ID(leaf) {
			covered = true
		}
	}
	if !covered {
		return fmt.Errorf("server activation before durable binding")
	}
	if e = store.Atomic(f.servedPath(), d); e != nil {
		return e
	}
	if f.boundary == "reload-before-return" {
		os.Exit(77)
	}
	return nil
}
func (f *crashTarget) Probe(context.Context) (*x509.Certificate, error) {
	if f.boundary == "confirm-before-commit" {
		os.Exit(77)
	}
	return credentials.ReadCert(f.servedPath())
}

func TestAgentCrashHelper(t *testing.T) {
	path := os.Getenv("QWAC_CRASH_CONFIG")
	if path == "" {
		return
	}
	d, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var cfg agent.Config
	if e = json.Unmarshal(d, &cfg); e != nil {
		t.Fatal(e)
	}
	boundary := os.Getenv("QWAC_CRASH_BOUNDARY")
	target := &crashTarget{cfg: cfg, boundary: boundary}
	ops := store.Operations{}
	if boundary == "before-publication" {
		ops.Sign = func(*credentials.Credentials, []*x509.Certificate, time.Time, time.Duration, bool) (string, binding.Header, error) {
			os.Exit(77)
			return "", binding.Header{}, nil
		}
	}
	if boundary == "published-before-journal" {
		ops.CommitManifest = func(path string, d []byte, _ bool) error {
			if e := store.Atomic(path, d); e != nil {
				return e
			}
			os.Exit(77)
			return nil
		}
	}
	a, e := agent.Open(cfg, target, ops)
	if e != nil {
		t.Fatal(e)
	}
	target.manager = a.Manager
	if e = a.Deploy(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Fatal("crash boundary was not reached")
}

func TestAgentProcessCrashReconciliation(t *testing.T) {
	for _, tc := range []struct {
		boundary, phase            string
		candidateServed, published bool
	}{
		{"before-publication", "prepared", false, false},
		{"published-before-journal", "prepared", false, true},
		{"selected-before-reload", "activating", false, true},
		{"reload-before-return", "reloading", true, true},
		{"confirm-before-commit", "confirming", true, true},
	} {
		t.Run(tc.boundary, func(t *testing.T) {
			cfg, _ := agentFixture(t)
			target := &crashTarget{cfg: cfg}
			a, e := agent.Open(cfg, target, store.Operations{})
			if e != nil {
				t.Fatal(e)
			}
			target.manager = a.Manager
			if e = a.Deploy(context.Background()); e != nil {
				t.Fatal(e)
			}
			original, e := target.Probe(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			old := binding.ID(original)
			candidate := renewAgentCandidate(t, cfg)
			if candidate == old {
				t.Fatal("candidate identity did not change")
			}
			if e = a.Close(); e != nil {
				t.Fatal(e)
			}
			configPath := filepath.Join(filepath.Dir(cfg.StateDir), "crash-config.json")
			d, _ := json.Marshal(cfg)
			if e = os.WriteFile(configPath, d, 0600); e != nil {
				t.Fatal(e)
			}
			child := exec.Command(os.Args[0], "-test.run=^TestAgentCrashHelper$")
			child.Env = append(os.Environ(), "QWAC_CRASH_CONFIG="+configPath, "QWAC_CRASH_BOUNDARY="+tc.boundary)
			out, err := child.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 77 {
				t.Fatalf("child did not crash at boundary: %v: %s", err, out)
			}
			d, e = os.ReadFile(filepath.Join(cfg.StateDir, "deployment.json"))
			if e != nil {
				t.Fatal(e)
			}
			var intent agent.Journal
			if e = json.Unmarshal(d, &intent); e != nil {
				t.Fatal(e)
			}
			if intent.Phase != tc.phase || intent.Previous != old || intent.Candidate != candidate {
				t.Fatal("incorrect durable intent", intent)
			}
			want := old
			if tc.candidateServed {
				want = candidate
			}
			served, e := target.Probe(context.Background())
			if e != nil || binding.ID(served) != want {
				t.Fatalf("wrong actual server before recovery: %v", e)
			}
			// Opening reacquires the OS locks released by abrupt process exit.
			for restart := 0; restart < 2; restart++ {
				a, e = agent.Open(cfg, target, store.Operations{})
				if e != nil {
					t.Fatal(e)
				}
				target.manager = a.Manager
				if gateCode(a) != 403 {
					t.Fatal("restart declared readiness before reconciliation")
				}
				if e = a.Reconcile(context.Background()); e != nil {
					a.Close()
					t.Fatal(e)
				}
				status, e := a.Status()
				if e != nil || !status.Ready || status.Deployment.Active != want {
					a.Close()
					t.Fatal("wrong reconciled server", status, e)
				}
				if tc.published && len(a.Manager.Snapshot().IDs) != 2 {
					a.Close()
					t.Fatal("recovery pruned a retained leaf")
				}
				if !tc.published && len(a.Manager.Snapshot().IDs) != 1 {
					a.Close()
					t.Fatal("unpublished candidate accepted")
				}
				served, e = target.Probe(context.Background())
				if e != nil || binding.ID(served) != want {
					a.Close()
					t.Fatal("recovery disagrees with actual server", e)
				}
				if e = a.Close(); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
