package coretest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/bindinghttp"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

func TestLifecycleRecoveryMissingMalformedAndInconsistentState(t *testing.T) {
	for _, fault := range []string{"missing-manifest", "malformed-manifest", "mutable-url", "missing-document", "corrupt-document", "signed-time", "tls-identity"} {
		t.Run(fault, func(t *testing.T) {
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
			state := m.Snapshot()
			manifestPath := filepath.Join(cfg.Dir, "current.json")
			documentPath := filepath.Join(cfg.Dir, filepath.Base(state.URL))
			manifest, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			document, err := os.ReadFile(documentPath)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			switch fault {
			case "missing-manifest":
				err = os.Remove(manifestPath)
			case "malformed-manifest":
				err = store.Atomic(manifestPath, []byte("{"))
				want = "unexpected end"
			case "missing-document":
				err = os.Remove(documentPath)
				want = "no such file"
			case "corrupt-document":
				err = store.Atomic(documentPath, []byte("corrupted staged data"))
				want = "hash mismatch"
			default:
				switch fault {
				case "mutable-url":
					state.URL = store.DiscoveryURL
					want = "immutable binding URL"
				case "signed-time":
					state.Expires--
					want = "signed time mismatch"
				case "tls-identity":
					state.IDs[0] = strings.Repeat("0", 64)
					want = "identifier mismatch"
				}
				var changed []byte
				changed, err = json.Marshal(state)
				if err == nil {
					err = store.Atomic(manifestPath, changed)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				recovered, err := store.Recover(cfg.Config, mat, store.Operations{})
				if fault == "missing-manifest" {
					if err != nil || bindinghttp.Available(recovered) {
						t.Fatal("orphan artifact advertised without current state", err)
					}
					if _, err = recovered.Document(state.URL); err == nil {
						t.Fatal("uncommitted artifact served as authenticated coverage")
					}
				} else if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatal("wrong recovery diagnosis", fault, err)
				}
			}
			if err = store.Atomic(documentPath, document); err != nil {
				t.Fatal(err)
			}
			if err = store.Atomic(manifestPath, manifest); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				recovered, err := store.Recover(cfg.Config, mat, store.Operations{})
				if err != nil || !bindinghttp.Available(recovered) {
					t.Fatal("consistent state did not recover repeatedly", err)
				}
			}
		})
	}
}

func TestAgentJournalDamageRemainsClosedAndRepairable(t *testing.T) {
	for _, fault := range []string{"missing", "malformed", "inconsistent"} {
		t.Run(fault, func(t *testing.T) {
			cfg, target := agentFixture(t)
			a, err := agent.Open(cfg, target, store.Operations{})
			if err != nil {
				t.Fatal(err)
			}
			target.publications = a.Manager
			if err = a.Deploy(t.Context()); err != nil {
				a.Close()
				t.Fatal(err)
			}
			path := filepath.Join(cfg.StateDir, "deployment.json")
			journal, err := os.ReadFile(path)
			if err != nil {
				a.Close()
				t.Fatal(err)
			}
			if err = a.Close(); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "missing":
				err = os.Remove(path)
			case "malformed":
				err = store.Atomic(path, []byte("{"))
			case "inconsistent":
				var j agent.Journal
				if err = json.Unmarshal(journal, &j); err != nil {
					t.Fatal(err)
				}
				j.Phase = "unsupported-intent"
				d, _ := json.Marshal(j)
				err = store.Atomic(path, d)
			}
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				a, err = agent.Open(cfg, target, store.Operations{})
				if fault == "missing" {
					if err != nil {
						t.Fatal(err)
					}
					for range 2 {
						err = a.Reconcile(t.Context())
						status, statusErr := a.Status()
						if err == nil || !strings.Contains(err.Error(), "without deployment journal") || statusErr != nil || status.Ready {
							a.Close()
							t.Fatal("missing intent authorized existing HTTPS", err, statusErr, status)
						}
					}
					a.Close()
				} else if err == nil {
					a.Close()
					t.Fatal("damaged journal accepted", fault)
				}
			}
			if err = store.Atomic(path, journal); err != nil {
				t.Fatal(err)
			}
			a, err = agent.Open(cfg, target, store.Operations{})
			if err != nil {
				t.Fatal("ownership leaked after failed open", err)
			}
			defer a.Close()
			target.publications = a.Manager
			for range 2 {
				if err = a.Reconcile(t.Context()); err != nil {
					t.Fatal(err)
				}
				status, err := a.Status()
				if err != nil || !status.Ready {
					t.Fatal("restored journal did not reconcile", err, status)
				}
			}
		})
	}
}
