package coretest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

// Exercise both consumers of the shared validation path and the existing
// publication transaction, with renewed certificates using either key choice.
func TestSuppliedCredentialReplacementAndRestart(t *testing.T) {
	for _, integration := range []string{"caddy", "agent"} {
		for _, sameKey := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/same-key=%v", integration, sameKey), func(t *testing.T) {
				cfg, tls := supplied(t)
				var ac agent.Config
				var target *deploymentTarget
				if integration == "agent" {
					ac, target = agentFixture(t)
					// Root omitted, matching ordinary supplied bundles and Caddy.
					ac.ChainFiles = ac.ChainFiles[:len(ac.ChainFiles)-1]
					cfg = plugin.BindingConfig{ChainFiles: ac.ChainFiles, KeyFile: ac.KeyFile, TrustFiles: []string{ac.TrustFile}, Config: store.Config{Host: ac.Host}}
				}
				var fail atomic.Bool
				ops := store.Operations{CommitManifest: func(path string, d []byte, _ bool) error {
					if fail.Load() {
						return fmt.Errorf("credential commit unavailable")
					}
					return store.Atomic(path, d)
				}}
				var manager *store.Manager
				var replace func() error
				var closeActive func()
				var reopen func() error
				if integration == "agent" {
					a, e := agent.Open(ac, target, ops)
					if e != nil {
						t.Fatal(e)
					}
					target.publications = a.Manager
					if e = a.Deploy(t.Context()); e != nil {
						t.Fatal(e)
					}
					manager = a.Manager
					replace = a.ReplaceCredentials
					closeActive = func() { a.Close() }
					reopen = func() error {
						var e error
						a, e = agent.Open(ac, target, ops)
						if e != nil {
							return e
						}
						manager = a.Manager
						target.publications = manager
						return a.Reconcile(t.Context())
					}
				} else {
					ctx, cancel := newContext()
					lease, e := plugin.Acquire(ctx, cfg, ops)
					if e != nil {
						cancel()
						t.Fatal(e)
					}
					if e = lease.Publish(tls); e != nil {
						t.Fatal(e)
					}
					manager = lease.Manager()
					leases := []*plugin.Lease{lease}
					stops := []func(){cancel}
					replace = func() error {
						ctx, stop := newContext()
						next, e := plugin.Acquire(ctx, cfg, ops)
						if e != nil {
							stop()
							return e
						}
						leases = append(leases, next)
						stops = append(stops, stop)
						return next.Activate()
					}
					closeActive = func() {
						for i, l := range leases {
							l.Release()
							stops[i]()
						}
						leases = nil
						stops = nil
					}
					reopen = func() error {
						ctx, stop := newContext()
						next, e := plugin.Acquire(ctx, cfg, ops)
						if e != nil {
							stop()
							return e
						}
						leases = append(leases, next)
						stops = append(stops, stop)
						manager = next.Manager()
						return next.Activate()
					}
				}
				defer func() { closeActive() }()
				initial := manager.Snapshot()
				old, e := credentials.ReadCert(cfg.ChainFiles[0])
				if e != nil {
					t.Fatal(e)
				}
				key, e := credentials.ReadKey(cfg.KeyFile)
				if e != nil {
					t.Fatal(e)
				}
				if !sameKey {
					key, e = rsa.GenerateKey(rand.Reader, 2048)
					if e != nil {
						t.Fatal(e)
					}
				}
				ca, e := credentials.ReadCert(cfg.ChainFiles[1])
				if e != nil {
					t.Fatal(e)
				}
				caKey, e := credentials.ReadKey(filepath.Join(filepath.Dir(cfg.KeyFile), "qwac-intermediate-key.pem"))
				if e != nil {
					t.Fatal(e)
				}
				template := *old
				template.SerialNumber = pki.Serial()
				template.ExtraExtensions = old.Extensions
				candidate, e := pki.Issue(&template, ca, &key.PublicKey, caKey)
				if e != nil {
					t.Fatal(e)
				}
				if e = pki.WriteCert(cfg.ChainFiles[0], candidate); e != nil {
					t.Fatal(e)
				}
				validKey, e := os.ReadFile(cfg.KeyFile)
				if e != nil {
					t.Fatal(e)
				}
				if e = pki.WriteKey(cfg.KeyFile, caKey); e != nil {
					t.Fatal(e)
				}
				if e = replace(); e == nil || !strings.Contains(e.Error(), "does not match") {
					t.Fatal("candidate mismatch", e)
				}
				if !reflect.DeepEqual(initial, manager.Snapshot()) {
					t.Fatal("rejected candidate changed binding")
				}
				if _, e = manager.Document(initial.URL); e != nil {
					t.Fatal("rejection lost old document", e)
				}
				if e = os.WriteFile(cfg.KeyFile, validKey, 0600); e != nil {
					t.Fatal(e)
				}
				if e = pki.WriteKey(cfg.KeyFile, key); e != nil {
					t.Fatal(e)
				}
				fail.Store(true)
				if e = replace(); e == nil || !strings.Contains(e.Error(), "credential commit unavailable") {
					t.Fatal("publication failure hidden", e)
				}
				if !reflect.DeepEqual(initial, manager.Snapshot()) {
					t.Fatal("failed publication changed active state")
				}
				if _, e = manager.Document(initial.URL); e != nil {
					t.Fatal("failure lost old document", e)
				}
				fail.Store(false)
				if e = manager.Refresh(); e != nil {
					t.Fatal("old signer unusable after failure", e)
				}
				assertSigner := func(want *x509.Certificate) {
					t.Helper()
					current := manager.Snapshot()
					doc, e := manager.Document(current.URL)
					if e != nil {
						t.Fatal(e)
					}
					_, chain, _, _, e := binding.Decode(string(doc))
					if e != nil || !chain[0].Equal(want) {
						t.Fatal("wrong active signer", e)
					}
				}
				assertSigner(old)
				if e = replace(); e != nil {
					t.Fatal(e)
				}
				assertSigner(candidate)
				if !reflect.DeepEqual(initial.IDs, manager.Snapshot().IDs) {
					t.Fatal("replacement lost TLS coverage")
				}
				closeActive()
				if e = reopen(); e != nil {
					t.Fatal("replacement restart", e)
				}
				assertSigner(candidate)
				if !reflect.DeepEqual(initial.IDs, manager.Snapshot().IDs) {
					t.Fatal("restart lost TLS coverage")
				}
				if integration == "agent" && verifyAgentReplacement(manager, ac, initial.IDs) != nil {
					t.Fatal("binding verification after restart failed")
				}
			})
		}
	}
}

func verifyAgentReplacement(m *store.Manager, c agent.Config, ids []string) error {
	mat, e := c.Material()
	if e != nil {
		return e
	}
	leaf, e := credentials.ReadCert(filepath.Join(c.CertbotLineage, "fullchain.pem"))
	if e != nil {
		return e
	}
	if len(ids) != 1 || ids[0] != binding.ID(leaf) {
		return fmt.Errorf("lost deployed leaf")
	}
	d, e := m.Document(m.Snapshot().URL)
	if e != nil {
		return e
	}
	return binding.Verify(string(d), c.Host, []*x509.Certificate{leaf}, mat.Chain[len(mat.Chain)-1], time.Now())
}

func TestCaddyFailedCredentialPublicationKeepsCoveredServingOnly(t *testing.T) {
	cfg, tls := supplied(t)
	var fail atomic.Bool
	ops := store.Operations{CommitManifest: func(path string, data []byte, _ bool) error {
		if fail.Load() {
			return fmt.Errorf("replacement disk failure")
		}
		return store.Atomic(path, data)
	}}
	ctx, stop := newContext()
	defer stop()
	old, e := plugin.Acquire(ctx, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	defer old.Release()
	if e = old.Publish(tls); e != nil {
		t.Fatal(e)
	}
	initial := old.Manager().Snapshot()
	replaceLeaf(t, cfg, "Pending TEST")
	nextCtx, nextStop := newContext()
	defer nextStop()
	next, e := plugin.Acquire(nextCtx, cfg, ops)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Release()
	next.SetStoredTLS(tls)
	fail.Store(true)
	if e = next.ActivateForServing(); e != nil {
		t.Fatal("failed replacement interrupted covered serving", e)
	}
	if !reflect.DeepEqual(initial, old.Manager().Snapshot()) {
		t.Fatal("fallback activated candidate")
	}
	// A distinct leaf must never use the serving fallback to bypass publication.
	_, unbound := supplied(t)
	if e = next.Publish(unbound); e == nil {
		t.Fatal("issuance bypassed publication gate")
	}
	next.SetStoredTLS(unbound)
	if e = next.ActivateForServing(); e == nil {
		t.Fatal("fallback allowed an uncovered stored TLS leaf")
	}
	next.SetStoredTLS(tls)
	if e = next.ActivateForServing(); e != nil {
		t.Fatal("covered old binding no longer usable", e)
	}
	fail.Store(false)
	if e = next.ActivateForServing(); e != nil {
		t.Fatal(e)
	}
	if old.Manager().Snapshot().Owner.Organization != "Pending TEST" {
		t.Fatal("repaired replacement not activated")
	}
}
