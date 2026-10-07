package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/bindinghttp"
	"github.com/cztamas/qwacamole/store"
)

// Journal records intent before every externally observable deployment boundary.
// Extra bound candidates survive rollback and are never automatically pruned.
type Journal struct {
	Phase     string    `json:"phase"`
	Active    string    `json:"active,omitempty"`
	Candidate string    `json:"candidate,omitempty"`
	Previous  string    `json:"previous,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	Updated   time.Time `json:"updated"`
}

// Agent serializes deployment requests. The shared manager serializes all
// binding publication, refresh, delivery and collection under its own lock.
type Agent struct {
	Config      Config
	Manager     *store.Manager
	target      Target
	lock        *os.File
	controlLock *os.File
	mu          sync.Mutex
	journal     Journal
	ready       atomic.Bool
	requestMu   sync.Mutex
	requests    sync.WaitGroup
	stopping    bool
}

// Open acquires exclusive ownership and verifies durable binding recovery. Call
// Reconcile before readiness; public delivery alone cannot select certificates.
func Open(c Config, target Target, ops store.Operations) (*Agent, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	if e := os.MkdirAll(c.StateDir, 0700); e != nil {
		return nil, e
	}
	st, e := os.Lstat(c.StateDir)
	if e != nil {
		return nil, e
	}
	if !st.IsDir() || st.Mode()&0077 != 0 {
		return nil, fmt.Errorf("state directory must be a private real directory (0700)")
	}
	lock, e := lockState(filepath.Join(c.StateDir, ".agent.lock"))
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(c.runtimeDir(), 0700); e != nil {
		lock.Close()
		return nil, e
	}
	runtimeInfo, e := os.Lstat(c.runtimeDir())
	if e != nil || !runtimeInfo.IsDir() || runtimeInfo.Mode()&0077 != 0 {
		lock.Close()
		return nil, fmt.Errorf("runtime directory must be a private real directory (0700)")
	}
	controlLock, e := lockState(filepath.Join(c.runtimeDir(), ".control.lock"))
	if e != nil {
		lock.Close()
		return nil, e
	}
	fail := func(e error) (*Agent, error) { controlLock.Close(); lock.Close(); return nil, e }
	for _, name := range []string{"enabled", "generations"} {
		if e = os.MkdirAll(filepath.Join(c.StateDir, name), 0700); e != nil {
			return fail(e)
		}
	}
	// Deployment settings cannot silently change across a retained state. A
	// full Nginx+agent stop and offline reconstruction is required for migration.
	configBytes, e := json.Marshal(c)
	if e != nil {
		return fail(e)
	}
	configPath := filepath.Join(c.StateDir, "deployment-config.json")
	priorConfig, e := os.ReadFile(configPath)
	if e == nil {
		if !bytes.Equal(priorConfig, configBytes) {
			return fail(fmt.Errorf("deployment settings changed; full-stop offline migration required"))
		}
	} else if os.IsNotExist(e) {
		if e = store.Atomic(configPath, configBytes); e != nil {
			return fail(e)
		}
	} else {
		return fail(e)
	}
	mat, e := c.Material()
	if e != nil {
		return fail(e)
	}
	cfg, e := c.StoreConfig()
	if e != nil {
		return fail(e)
	}
	m, e := store.Recover(cfg, mat, ops)
	if e != nil {
		return fail(e)
	}
	if target == nil {
		target = Nginx{Config: c}
	}
	a := &Agent{Config: c, Manager: m, target: target, lock: lock, controlLock: controlLock, journal: Journal{Phase: "empty"}}
	d, e := os.ReadFile(a.journalPath())
	if e == nil {
		if e = json.Unmarshal(d, &a.journal); e != nil {
			return fail(e)
		}
	} else if !os.IsNotExist(e) {
		return fail(e)
	}
	switch a.journal.Phase {
	case "empty", "prepared", "bound", "activating", "reloading", "confirming", "rolling_back", "rolled_back", "committed":
	default:
		return fail(fmt.Errorf("unsupported deployment journal phase"))
	}
	if a.journal.Phase == "committed" && (a.journal.Active == "" || a.journal.Active != a.journal.Candidate) {
		return fail(fmt.Errorf("inconsistent committed deployment journal"))
	}
	for _, id := range []string{a.journal.Active, a.journal.Previous, a.journal.Candidate} {
		if id != "" && !generationID.MatchString(id) {
			return fail(fmt.Errorf("invalid journal generation"))
		}
	}
	return a, nil
}
func (a *Agent) Close() error {
	a.ready.Store(false)
	return errors.Join(a.controlLock.Close(), a.lock.Close())
}
func (a *Agent) journalPath() string { return filepath.Join(a.Config.StateDir, "deployment.json") }
func (a *Agent) save(j Journal) error {
	j.Updated = time.Now().UTC()
	d, e := json.MarshalIndent(j, "", "  ")
	if e != nil {
		return e
	}
	if e = store.Atomic(a.journalPath(), d); e != nil {
		return e
	}
	a.journal = j
	return nil
}

var generationID = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (a *Agent) generation(id string) string {
	return filepath.Join(a.Config.StateDir, "generations", id)
}
func (a *Agent) pointer() string { return filepath.Join(a.Config.StateDir, "enabled", "site.conf") }
func syncDir(dir string) error {
	f, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func (a *Agent) selectGeneration(id string) error {
	path := a.pointer()
	if id == "" {
		e := os.Remove(path)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		return syncDir(filepath.Dir(path))
	}
	if !generationID.MatchString(id) {
		return fmt.Errorf("invalid generation")
	}
	tmp := path + ".staging"
	if e := os.Remove(tmp); e != nil && !os.IsNotExist(e) {
		return e
	}
	if e := os.Symlink(filepath.Join(a.generation(id), "https.conf"), tmp); e != nil {
		return e
	}
	if e := os.Rename(tmp, path); e != nil {
		return e
	}
	return syncDir(filepath.Dir(path))
}
func (a *Agent) selected() (string, error) {
	path, e := os.Readlink(a.pointer())
	if os.IsNotExist(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	id := filepath.Base(filepath.Dir(path))
	if !generationID.MatchString(id) || path != filepath.Join(a.generation(id), "https.conf") {
		return "", fmt.Errorf("unmanaged HTTPS include")
	}
	return id, nil
}
func (a *Agent) verifyPair(chain, key []byte) (*x509.Certificate, error) {
	pair, e := tls.X509KeyPair(chain, key)
	if e != nil {
		return nil, e
	}
	leaf, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		return nil, e
	}
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != a.Config.Host || len(leaf.IPAddresses) != 0 || leaf.IsCA {
		return nil, fmt.Errorf("candidate must be one-host TLS leaf for %s", a.Config.Host)
	}
	roots, e := a.Config.roots()
	if e != nil {
		return nil, e
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		c, e := x509.ParseCertificate(der)
		if e != nil {
			return nil, e
		}
		intermediates.AddCert(c)
	}
	_, e = leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, DNSName: a.Config.Host, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
	return leaf, e
}
func (a *Agent) generationLeaf(id string) (*x509.Certificate, error) {
	chain, e := os.ReadFile(filepath.Join(a.generation(id), "fullchain.pem"))
	if e != nil {
		return nil, e
	}
	key, e := os.ReadFile(filepath.Join(a.generation(id), "privkey.pem"))
	if e != nil {
		return nil, e
	}
	config, e := os.ReadFile(filepath.Join(a.generation(id), "https.conf"))
	if e != nil || string(config) != a.Config.httpsConfig(a.generation(id)) {
		return nil, fmt.Errorf("generation Nginx config mismatch")
	}
	leaf, e := a.verifyPair(chain, key)
	if e != nil {
		return nil, e
	}
	if binding.ID(leaf) != id {
		return nil, fmt.Errorf("generation content mismatch")
	}
	return leaf, nil
}
func (a *Agent) covered(id string) bool {
	for _, v := range a.Manager.Snapshot().IDs {
		if v == id {
			return true
		}
	}
	return false
}
func (a *Agent) confirm(ctx context.Context, id string) error {
	end := time.Now().Add(10 * time.Second)
	var last error
	for {
		c, e := a.target.Probe(ctx)
		if e == nil && binding.ID(c) == id {
			return nil
		}
		if e == nil {
			e = fmt.Errorf("served TLS leaf %s, expected %s", binding.ID(c), id)
		}
		last = e
		if time.Now().After(end) {
			return fmt.Errorf("fresh verified TLS confirmation: %w", last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
func (a *Agent) rollback(ctx context.Context, j Journal, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	a.ready.Store(false)
	j.LastError = cause.Error()
	j.Phase = "rolling_back"
	journalErr := a.save(j)
	// Existing durable intent already records both generations. Even if another
	// journal write fails, attempt restoration instead of abandoning active TLS.
	if journalErr != nil {
		cause = errors.Join(cause, journalErr)
	}
	if e := a.selectGeneration(j.Previous); e != nil {
		return errors.Join(cause, e)
	}
	if e := a.target.Validate(ctx); e != nil {
		return errors.Join(cause, e)
	}
	if e := a.target.Reload(ctx); e != nil {
		return errors.Join(cause, e)
	}
	if j.Previous != "" {
		if !a.covered(j.Previous) {
			return errors.Join(cause, fmt.Errorf("prior TLS leaf missing binding coverage"))
		}
		if e := a.confirm(ctx, j.Previous); e != nil {
			return errors.Join(cause, e)
		}
	}
	j.Phase = "rolled_back"
	j.Active = j.Previous
	if e := a.save(j); e != nil {
		return errors.Join(cause, e)
	}
	a.ready.Store(j.Active != "" && bindinghttp.Available(a.Manager))
	return cause
}

// Generations are immutable. A crash during staging cannot leave a partial
// directory that a later retry mistakes for a complete deployment.
func (a *Agent) stageGeneration(id string, chain, key []byte) error {
	dir := a.generation(id)
	files := map[string][]byte{"fullchain.pem": chain, "privkey.pem": key, "https.conf": []byte(a.Config.httpsConfig(dir))}
	if st, e := os.Lstat(dir); e == nil {
		if !st.IsDir() {
			return fmt.Errorf("generation must be a real directory")
		}
		for name, want := range files {
			got, e := os.ReadFile(filepath.Join(dir, name))
			if e != nil || !bytes.Equal(got, want) {
				return fmt.Errorf("immutable generation differs: %s", name)
			}
		}
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	staged, e := os.MkdirTemp(filepath.Dir(dir), ".stage-"+id+"-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(staged)
	for name, d := range files {
		if e = store.Atomic(filepath.Join(staged, name), d); e != nil {
			return e
		}
	}
	if e = syncDir(staged); e != nil {
		return e
	}
	if e = os.Rename(staged, dir); e != nil {
		return e
	}
	return syncDir(filepath.Dir(dir))
}

// Deploy reads the configured Certbot lineage once. Control callers cannot
// choose paths. The private key is staged only after verified durable binding.
func (a *Agent) Deploy(ctx context.Context) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if err != nil {
			j := a.journal
			j.LastError = err.Error()
			err = errors.Join(err, a.save(j))
		}
	}()
	chain, e := os.ReadFile(filepath.Join(a.Config.CertbotLineage, "fullchain.pem"))
	if e != nil {
		return e
	}
	key, e := os.ReadFile(filepath.Join(a.Config.CertbotLineage, "privkey.pem"))
	if e != nil {
		return e
	}
	leaf, e := a.verifyPair(chain, key)
	if e != nil {
		return e
	}
	previous, e := a.selected()
	if e != nil {
		return e
	}
	if a.journal.Phase != "committed" && a.journal.Phase != "rolled_back" && a.journal.Phase != "empty" {
		return fmt.Errorf("interrupted deployment requires reconciliation before retry")
	}
	j := Journal{Phase: "prepared", Previous: previous, Active: previous, Candidate: binding.ID(leaf)}
	if e = a.save(j); e != nil {
		return e
	}
	if e = a.Manager.Publish(leaf); e != nil {
		j.Phase = "rolled_back"
		j.LastError = e.Error()
		return errors.Join(e, a.save(j))
	}
	j.Phase = "bound"
	if e = a.save(j); e != nil {
		return e
	}
	if e = a.stageGeneration(j.Candidate, chain, key); e != nil {
		return e
	}
	j.Phase = "activating"
	if e = a.save(j); e != nil {
		return e
	}
	if e = a.selectGeneration(j.Candidate); e != nil {
		return a.rollback(ctx, j, e)
	}
	if e = a.target.Validate(ctx); e != nil {
		return a.rollback(ctx, j, e)
	}
	j.Phase = "reloading"
	if e = a.save(j); e != nil {
		return a.rollback(ctx, j, e)
	}
	if e = a.target.Reload(ctx); e != nil {
		return a.rollback(ctx, j, e)
	}
	j.Phase = "confirming"
	if e = a.save(j); e != nil {
		return a.rollback(ctx, j, e)
	}
	if e = a.confirm(ctx, j.Candidate); e != nil {
		return a.rollback(ctx, j, e)
	}
	j.Phase = "committed"
	j.Active = j.Candidate
	if e = a.save(j); e != nil {
		return a.rollback(ctx, j, e)
	}
	a.ready.Store(bindinghttp.Available(a.Manager))
	return nil
}

// Reconcile compares durable intent, selected files and a fresh verified served
// certificate. It never infers success from a previous reload's exit status.
func (a *Agent) Reconcile(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready.Store(false)
	j := a.journal
	selected, e := a.selected()
	if e != nil {
		return e
	}
	if j.Phase == "empty" {
		if selected != "" {
			return fmt.Errorf("HTTPS include exists without deployment journal")
		}
		return nil
	}
	// Re-publish recovered complete coverage with supplied same-authority QWAC.
	mat, e := a.Config.Material()
	if e != nil {
		return e
	}
	if e = a.Manager.Activate(mat); e != nil {
		return e
	}
	if (j.Phase == "committed" || j.Phase == "rolled_back") && j.Active != "" && selected == j.Active && a.covered(j.Active) {
		if _, err := a.generationLeaf(j.Active); err == nil {
			if served, err := a.target.Probe(ctx); err == nil && binding.ID(served) == j.Active {
				a.ready.Store(bindinghttp.Available(a.Manager))
				return nil
			}
		}
	}
	if j.Candidate != "" && selected == j.Candidate && a.covered(j.Candidate) {
		if _, e = a.generationLeaf(j.Candidate); e == nil {
			if served, err := a.target.Probe(ctx); err == nil && binding.ID(served) == j.Candidate {
				j.Phase = "committed"
				j.Active = j.Candidate
				j.LastError = ""
				if e = a.save(j); e != nil {
					return e
				}
				a.ready.Store(bindinghttp.Available(a.Manager))
				return nil
			}
		}
	}
	prior := j.Active
	if j.Phase != "committed" && j.Phase != "rolled_back" {
		prior = j.Previous
	}
	j.Previous = prior
	cause := fmt.Errorf("reconciling interrupted or unconfirmed %s deployment", j.Phase)
	if e = a.rollback(ctx, j, cause); e != cause {
		return e
	}
	return nil
}

// ReplaceCredentials validates and atomically publishes a same-authority QWAC
// replacement with every retained TLS leaf; it does not reload Nginx.
func (a *Agent) ReplaceCredentials() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	m, e := a.Config.Material()
	if e != nil {
		return e
	}
	return a.Manager.Activate(m)
}
