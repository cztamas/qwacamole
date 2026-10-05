// Package store publishes immutable binding documents before atomically switching
// the current manifest. Old documents remain available across rotation/restart.
package store

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cztamas/qwacamole/internal/binding"
	"github.com/cztamas/qwacamole/internal/pki"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	Dir                  string        `json:"directory"`
	PKI                  string        `json:"pki"`
	Host                 string        `json:"hostname"`
	Lifetime             time.Duration `json:"lifetime"`
	FirefoxCompatibility bool          `json:"firefox_compatibility,omitempty"`
}
type State struct {
	Owner          pki.Identity `json:"synthetic_owner"`
	URL            string       `json:"url"`
	Certs          []string     `json:"certificates_der"`
	IDs            []string     `json:"tls_ids"`
	Expires        int64        `json:"expires"`
	Published      int64        `json:"published"`
	BindingProfile string       `json:"binding_profile,omitempty"`
}
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	material *pki.Material
	current  State
}

var registry = struct {
	sync.Mutex
	m map[string]*Manager
}{m: map[string]*Manager{}}

func Shared(cfg Config) (*Manager, error) {
	registry.Lock()
	defer registry.Unlock()
	dir, e := filepath.Abs(cfg.Dir)
	if e != nil {
		return nil, e
	}
	cfg.Dir = dir
	if cfg.Lifetime == 0 {
		cfg.Lifetime = 45 * time.Second
	}
	if m := registry.m[dir]; m != nil {
		if m.cfg != cfg {
			return nil, fmt.Errorf("binding manager config mismatch")
		}
		return m, nil
	}
	m, e := Open(cfg)
	if e == nil {
		registry.m[dir] = m
	}
	return m, e
}
func Open(cfg Config) (*Manager, error) {
	if e := os.MkdirAll(cfg.Dir, 0700); e != nil {
		return nil, e
	}
	mat, e := pki.Load(cfg.PKI)
	if e != nil {
		return nil, e
	}
	m := &Manager{cfg: cfg, material: mat}
	d, e := os.ReadFile(filepath.Join(cfg.Dir, "current.json"))
	if os.IsNotExist(e) {
		return m, nil
	}
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(d, &m.current); e != nil {
		return nil, e
	}
	doc, e := os.ReadFile(filepath.Join(cfg.Dir, filepath.Base(m.current.URL)))
	if e != nil {
		return nil, e
	}
	h, _, _, _, e := binding.Decode(string(doc))
	if e != nil {
		return nil, e
	}
	if h.Exp != m.current.Expires {
		return nil, errors.New("recovery expiry mismatch")
	}
	sum := sha256.Sum256(doc)
	if filepath.Base(m.current.URL) != hex.EncodeToString(sum[:])+".jose" {
		return nil, errors.New("recovery document hash mismatch")
	}
	var all []*x509.Certificate
	for n, raw := range m.current.Certs {
		der, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, err
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, err
		}
		if n >= len(m.current.IDs) || m.current.IDs[n] != binding.ID(c) {
			return nil, errors.New("recovery certificate identifier mismatch")
		}
		all = append(all, c)
	}
	if err := m.verify(string(doc), all, time.Unix(h.Iat, 0)); err != nil {
		return nil, fmt.Errorf("recovery binding: %w", err)
	}

	cs, e := m.certsLocked()
	if e != nil {
		return nil, e
	}
	if hasLiveCertificate(cs, time.Now()) { // refresh also re-verifies an expired document before it is advertised
		if e = m.publishLocked(cs, time.Now()); e != nil {
			return nil, fmt.Errorf("recovery publication: %w", e)
		}
	}
	return m, nil
}
func (m *Manager) Snapshot() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.current
	s.Certs = append([]string(nil), s.Certs...)
	s.IDs = append([]string(nil), s.IDs...)
	return s
}
func (m *Manager) certsLocked() ([]*x509.Certificate, error) {
	var cs []*x509.Certificate
	for _, s := range m.current.Certs {
		d, e := base64.StdEncoding.DecodeString(s)
		if e != nil {
			return nil, e
		}
		c, e := x509.ParseCertificate(d)
		if e != nil {
			return nil, e
		}
		// A browser may keep an authenticated HTTP/2 connection beyond this
		// certificate's expiry. Retain its binding alongside renewed certificates
		// for the lifetime of this local state; pruning by NotAfter breaks those
		// connections. This does not change TLS verification for new handshakes.
		cs = append(cs, c)
	}
	return cs, nil
}
func hasLiveCertificate(cs []*x509.Certificate, now time.Time) bool {
	for _, c := range cs {
		if c.NotAfter.After(now) {
			return true
		}
	}
	return false
}
func (m *Manager) Publish(c *x509.Certificate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	cs, e := m.certsLocked()
	if e != nil {
		return e
	}
	for _, old := range cs {
		if old.Equal(c) {
			if m.current.Expires > now.Add(10*time.Second).Unix() {
				return nil
			}
			return m.publishLocked(cs, now)
		}
	}
	cs = append(cs, c)
	return m.publishCandidateLocked(cs, now, true)
}
func (m *Manager) Refresh() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.current.Expires > now.Add(15*time.Second).Unix() {
		return nil
	}
	cs, e := m.certsLocked()
	if e != nil {
		return e
	}
	if !hasLiveCertificate(cs, now) {
		return nil
	}
	return m.publishLocked(cs, now)
}
func (m *Manager) publishLocked(cs []*x509.Certificate, now time.Time) error {
	return m.publishCandidateLocked(cs, now, false)
}
func (m *Manager) publishCandidateLocked(cs []*x509.Certificate, now time.Time, newCertificate bool) error {
	if newCertificate {
		if _, err := os.Stat(filepath.Join(m.cfg.Dir, "fail-sign")); err == nil {
			return fmt.Errorf("injected binding fail-sign failure for new TLS certificate")
		}
	}
	sign := binding.Sign
	if m.cfg.FirefoxCompatibility {
		sign = binding.SignFirefox
	}
	doc, h, e := sign(m.material, cs, now, m.cfg.Lifetime)
	if e != nil {
		return e
	}
	if e = m.verify(doc, cs, now); e != nil {
		return e
	}
	sum := sha256.Sum256([]byte(doc))
	name := hex.EncodeToString(sum[:]) + ".jose"
	subject := m.material.Chain[0].Subject
	owner := pki.Identity{Host: m.cfg.Host}
	if len(subject.Organization) > 0 {
		owner.Organization = subject.Organization[0]
	}
	if len(subject.Country) > 0 {
		owner.Country = subject.Country[0]
	}
	if len(subject.Locality) > 0 {
		owner.Locality = subject.Locality[0]
	}
	for _, name := range subject.Names {
		if name.Type.String() == "2.5.4.97" {
			owner.Registration = fmt.Sprint(name.Value)
		}
	}
	state := State{Owner: owner, URL: "/bindings/" + name, Expires: h.Exp, Published: now.Unix()}
	if m.cfg.FirefoxCompatibility {
		state.BindingProfile = "firefox-compatibility-without-crit"
	}
	for _, c := range cs {
		state.Certs = append(state.Certs, base64.StdEncoding.EncodeToString(c.Raw))
		state.IDs = append(state.IDs, binding.ID(c))
	}
	if e = Atomic(filepath.Join(m.cfg.Dir, name), []byte(doc)); e != nil {
		return e
	}
	// Exercise failure after the immutable document is durable, before its
	// manifest becomes current. An unadvertised document cannot activate TLS.
	if newCertificate {
		if _, err := os.Stat(filepath.Join(m.cfg.Dir, "fail-publish")); err == nil {
			return fmt.Errorf("injected binding fail-publish failure for new TLS certificate")
		}
	}
	d, e := json.MarshalIndent(state, "", "  ")
	if e != nil {
		return e
	}
	if e = Atomic(filepath.Join(m.cfg.Dir, "current.json"), d); e != nil {
		return e
	}
	m.current = state
	return nil
}

func (m *Manager) verify(doc string, cs []*x509.Certificate, now time.Time) error {
	verify := binding.Verify
	if m.cfg.FirefoxCompatibility {
		verify = binding.VerifyFirefox
	}
	return verify(doc, m.cfg.Host, cs, m.material.Chain[len(m.material.Chain)-1], now)
}
func Atomic(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".atomic-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return e
	}
	d, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
