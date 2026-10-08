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
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Dir      string        `json:"directory"`
	Host     string        `json:"hostname"`
	Lifetime time.Duration `json:"lifetime"`
	// RefreshBefore is the advance margin, reduced proportionally when TLS or
	// QWAC expiry caps the effective lifetime. Zero selects Lifetime/3.
	RefreshBefore time.Duration `json:"refresh_before,omitempty"`
	// DocumentGrace protects historical and orphaned documents until both their
	// expiry and filesystem modification time are at least this far in the past.
	DocumentGrace time.Duration `json:"document_grace,omitempty"`
	// MaxCertificates refuses additions beyond this count; it never prunes
	// certificates needed by authenticated connections, even after NotAfter.
	MaxCertificates int `json:"max_certificates,omitempty"`
	// MaxDocumentBytes refuses oversized publications before writing any state.
	MaxDocumentBytes     int64 `json:"max_document_bytes,omitempty"`
	FirefoxCompatibility bool  `json:"firefox_compatibility,omitempty"`
}
type State struct {
	Owner          credentials.Identity `json:"synthetic_owner"`
	URL            string               `json:"url"`
	DiscoveryURL   string               `json:"discovery_url,omitempty"`
	Certs          []string             `json:"certificates_der"`
	IDs            []string             `json:"tls_ids"`
	Expires        int64                `json:"expires"`
	Published      int64                `json:"published"`
	BindingProfile string               `json:"binding_profile,omitempty"`
}
type Manager struct {
	mu              sync.RWMutex
	cfg             Config
	material        *credentials.Credentials
	operations      Operations
	current         State
	currentDoc      []byte
	changed         chan struct{}
	lastFailure     string
	retryAt         time.Time
	refreshFailures int
	refreshFailure  string
}

// DiscoveryURL is stable across publication, renewal and credential replacement.
// State.URL continues to identify an immutable recovery artifact.
const DiscoveryURL = "/bindings/current.jose"

// Operations overrides signing, document-write or manifest-commit boundaries. Nil functions
// use the standard implementation. A new certificate is never returned to the
// issuer until both succeed; every candidate is verified before publication.
// Custom writers must durably write supplied bytes to the supplied path before
// returning nil. The defaults use Atomic, including directory fsync. A returned
// manifest error restores prior bytes if the failing writer already renamed the
// candidate. An unsuccessful rollback is reported explicitly; storage repair is
// then required before restart. Process crashes recover a verified old/new commit.
type Operations struct {
	Sign           func(*credentials.Credentials, []*x509.Certificate, time.Time, time.Duration, bool) (string, binding.Header, error)
	WriteDocument  func(string, []byte) error
	CommitManifest func(string, []byte, bool) error
}

func Open(cfg Config, mat *credentials.Credentials, ops Operations) (*Manager, error) {
	return open(cfg, mat, ops, true)
}

// Recover validates durable state without publishing or switching credentials.
// Integrations use this when provisioning a configuration that may be rejected.
func Recover(cfg Config, mat *credentials.Credentials, ops Operations) (*Manager, error) {
	return open(cfg, mat, ops, false)
}

func open(cfg Config, mat *credentials.Credentials, ops Operations, refresh bool) (*Manager, error) {
	if e := mat.Validate(); e != nil {
		return nil, e
	}
	var err error
	cfg, err = cfg.WithDefaults()
	if err != nil {
		return nil, err
	}
	if e := os.MkdirAll(cfg.Dir, 0700); e != nil {
		return nil, e
	}
	m := &Manager{cfg: cfg, material: mat, operations: ops, changed: make(chan struct{}, 1)}
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
	if !immutablePath(m.current.URL) {
		return nil, errors.New("recovery requires an immutable binding URL")
	}
	doc, e := m.readDocumentLocked(filepath.Base(m.current.URL))
	if e != nil {
		return nil, e
	}
	h, _, _, _, e := binding.Decode(string(doc))
	if e != nil {
		return nil, e
	}
	if h.Exp != m.current.Expires || h.Iat != m.current.Published {
		return nil, errors.New("recovery signed time mismatch")
	}
	sum := sha256.Sum256(doc)
	if filepath.Base(m.current.URL) != hex.EncodeToString(sum[:])+".jose" {
		return nil, errors.New("recovery document hash mismatch")
	}
	var all []*x509.Certificate
	if len(m.current.Certs) != len(m.current.IDs) || len(m.current.IDs) != len(h.SigD.HashV) {
		return nil, errors.New("recovery certificate identifier count mismatch")
	}
	seen := map[string]bool{}
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
		if seen[m.current.IDs[n]] {
			return nil, errors.New("duplicate recovery TLS certificate")
		}
		seen[m.current.IDs[n]] = true
		all = append(all, c)
	}
	if err := m.verify(string(doc), all, time.Unix(h.Iat, 0)); err != nil {
		return nil, fmt.Errorf("recovery binding: %w", err)
	}
	m.currentDoc = append([]byte(nil), doc...)
	m.current.DiscoveryURL = DiscoveryURL

	cs, e := m.certsLocked()
	if e != nil {
		return nil, e
	}
	if refresh && hasLiveCertificate(cs, time.Now()) { // refresh also re-verifies an expired document before it is advertised
		if e = m.publishLocked(cs, time.Now()); e != nil {
			return nil, fmt.Errorf("recovery publication: %w", e)
		}
	}
	return m, nil
}

// PrepareReplacement checks a same-authority candidate without changing files or
// the live manager. Replacement rebuilds coverage under the same lock at commit.
func (m *Manager) PrepareReplacement(mat *credentials.Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prepareReplacementLocked(mat)
}

func (m *Manager) prepareReplacementLocked(mat *credentials.Credentials) error {
	if e := mat.Validate(); e != nil {
		return e
	}
	if !mat.Chain[len(mat.Chain)-1].Equal(m.material.Chain[len(m.material.Chain)-1]) {
		return errors.New("QWAC authority migration is not supported")
	}
	cs, e := m.certsLocked()
	if e != nil {
		return e
	}
	if !hasLiveCertificate(cs, time.Now()) {
		return nil
	}
	sign := binding.Sign
	if m.cfg.FirefoxCompatibility {
		sign = binding.SignFirefox
	}
	doc, _, e := sign(mat, cs, time.Now(), m.cfg.Lifetime)
	if e != nil {
		return e
	}
	candidate := &Manager{cfg: m.cfg, material: mat, operations: m.operations, current: m.current}
	return candidate.verify(doc, cs, time.Now())
}

// Replace atomically replaces credentials while preserving retained TLS coverage.
func (m *Manager) Replace(mat *credentials.Credentials) error {
	return m.Activate(mat)
}

// Activate prepares candidate credentials and the complete retained/additional
// TLS set together. One signed, verified, durable publication precedes any live
// signer/state change. Additional leaves include Caddy's stored or newly issued
// certificate, which may be absent from the recovered manifest.
func (m *Manager) Activate(mat *credentials.Credentials, additional ...*x509.Certificate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.prepareReplacementLocked(mat); e != nil {
		return e
	}
	cs, e := m.certsLocked()
	if e != nil {
		return e
	}
	cs, added := unionCertificates(cs, additional)
	if len(cs) > 0 {
		candidate := &Manager{cfg: m.cfg, material: mat, operations: m.operations, current: m.current, currentDoc: m.currentDoc}
		if e = candidate.publishCandidateLocked(cs, time.Now(), added); e != nil {
			m.lastFailure = e.Error()
			return e
		}
		m.current = candidate.current
		m.currentDoc = candidate.currentDoc
		m.lastFailure = ""
		m.notifyLocked()
	}
	m.material = mat
	m.notifyLocked()
	return nil
}

func unionCertificates(retained, additional []*x509.Certificate) ([]*x509.Certificate, bool) {
	seen := make(map[string]bool, len(retained))
	for _, c := range retained {
		seen[binding.ID(c)] = true
	}
	added := false
	for _, c := range additional {
		if c != nil && !seen[binding.ID(c)] {
			retained = append(retained, c)
			seen[binding.ID(c)] = true
			added = true
		}
	}
	return retained, added
}

// CurrentDocument returns a verified copy of exactly one committed snapshot.
// The stable endpoint never selects a staged artifact or extends signed expiry.
func (m *Manager) CurrentDocument() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.currentDoc) == 0 || m.current.Expires <= time.Now().Unix() {
		return nil, errors.New("current binding unavailable or expired")
	}
	cs, e := m.certsLocked()
	if e != nil {
		return nil, e
	}
	if e = m.verify(string(m.currentDoc), cs, time.Now()); e != nil {
		return nil, e
	}
	return append([]byte(nil), m.currentDoc...), nil
}
func (m *Manager) Snapshot() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.current
	s.Certs = append([]string(nil), s.Certs...)
	s.IDs = append([]string(nil), s.IDs...)
	return s
}

// Document serves only retained, content-addressed, independently verified
// bindings. Historical documents may be expired; their signed content is immutable.
func (m *Manager) Document(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !immutablePath(path) {
		return nil, errors.New("invalid binding path")
	}
	name := strings.TrimPrefix(path, "/bindings/")
	d, e := m.readDocumentLocked(name)
	if e != nil {
		return nil, e
	}
	h, _, _, _, e := binding.Decode(string(d))
	if e != nil {
		return nil, e
	}
	all, e := m.certsLocked()
	if e != nil {
		return nil, e
	}
	var cs []*x509.Certificate
	for _, c := range all {
		for _, hash := range h.SigD.HashV {
			if hash == binding.TLSHash(c) {
				cs = append(cs, c)
				break
			}
		}
	}
	if len(cs) != len(h.SigD.HashV) {
		return nil, errors.New("document contains unretained certificates")
	}
	if e = m.verify(string(d), cs, time.Unix(h.Iat, 0)); e != nil {
		return nil, e
	}
	return d, nil
}

func immutablePath(path string) bool {
	name := strings.TrimPrefix(path, "/bindings/")
	if path != "/bindings/"+name || len(name) != 69 || !strings.HasSuffix(name, ".jose") || name != strings.ToLower(name) {
		return false
	}
	_, e := hex.DecodeString(name[:64])
	return e == nil
}

func (m *Manager) readDocumentLocked(name string) ([]byte, error) {
	p := filepath.Join(m.cfg.Dir, name)
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("binding must be a regular file")
	}
	d, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(d)
	if name != hex.EncodeToString(sum[:])+".jose" {
		return nil, errors.New("binding content hash mismatch")
	}
	return d, nil
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
			if now.Before(m.nextRefreshLocked()) {
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
	if now.Before(m.nextRefreshLocked()) {
		return nil
	}
	cs, e := m.certsLocked()
	if e != nil {
		return e
	}
	if len(cs) == 0 {
		return nil
	}
	return m.publishLocked(cs, now)
}
func (m *Manager) publishLocked(cs []*x509.Certificate, now time.Time) error {
	return m.publishCandidateLocked(cs, now, false)
}
func (m *Manager) publishCandidateLocked(cs []*x509.Certificate, now time.Time, newCertificate bool) (err error) {
	defer func() {
		if err != nil {
			m.lastFailure = err.Error()
		} else {
			m.lastFailure = ""
		}
	}()
	if len(cs) > m.cfg.MaxCertificates && len(cs) > len(m.current.Certs) {
		return fmt.Errorf("retained TLS certificate capacity %d reached: drain all connections and retire state offline before adding certificates", m.cfg.MaxCertificates)
	}
	sign := binding.Sign
	if m.cfg.FirefoxCompatibility {
		sign = binding.SignFirefox
	}
	var doc string
	var h binding.Header
	var e error
	if m.operations.Sign != nil {
		doc, h, e = m.operations.Sign(m.material, cs, now, m.cfg.Lifetime, newCertificate)
	} else {
		doc, h, e = sign(m.material, cs, now, m.cfg.Lifetime)
	}
	if e != nil {
		return e
	}
	if e = m.verify(doc, cs, now); e != nil {
		return e
	}
	// Derive manifest metadata from the independently verified signed bytes,
	// never from an injected signer's separately returned header.
	h, _, _, _, e = binding.Decode(doc)
	if e != nil {
		return e
	}
	if int64(len(doc)) > m.cfg.MaxDocumentBytes {
		return fmt.Errorf("binding document exceeds max_document_bytes %d; drain and retire TLS coverage offline or raise capacity", m.cfg.MaxDocumentBytes)
	}
	sum := sha256.Sum256([]byte(doc))
	name := hex.EncodeToString(sum[:]) + ".jose"
	subject := m.material.Chain[0].Subject
	owner := credentials.Identity{Host: m.cfg.Host}
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
	state := State{Owner: owner, URL: "/bindings/" + name, DiscoveryURL: DiscoveryURL, Expires: h.Exp, Published: h.Iat}
	if m.cfg.FirefoxCompatibility {
		state.BindingProfile = "firefox-compatibility-without-crit"
	}
	for _, c := range cs {
		state.Certs = append(state.Certs, base64.StdEncoding.EncodeToString(c.Raw))
		state.IDs = append(state.IDs, binding.ID(c))
	}
	write := m.operations.WriteDocument
	if write == nil {
		write = Atomic
	}
	if e = write(filepath.Join(m.cfg.Dir, name), []byte(doc)); e != nil {
		return e
	}
	if persisted, err := m.readDocumentLocked(name); err != nil || string(persisted) != doc {
		return fmt.Errorf("candidate document not durably readable: %v", err)
	}
	d, e := json.MarshalIndent(state, "", "  ")
	if e != nil {
		return e
	}
	commit := m.operations.CommitManifest
	if commit == nil {
		commit = func(path string, d []byte, _ bool) error { return Atomic(path, d) }
	}
	manifestPath := filepath.Join(m.cfg.Dir, "current.json")
	prior, priorErr := os.ReadFile(manifestPath)
	if priorErr != nil && !os.IsNotExist(priorErr) {
		return priorErr
	}
	if e = commit(manifestPath, d, newCertificate); e != nil {
		// Atomic rename may have succeeded before a directory sync error was
		// reported. Preserve ordinary error semantics by restoring the old commit.
		actual, readErr := os.ReadFile(manifestPath)
		unchanged := readErr == nil && string(actual) == string(prior) || os.IsNotExist(readErr) && os.IsNotExist(priorErr)
		if !unchanged {
			var rollback error
			if os.IsNotExist(priorErr) {
				rollback = os.Remove(manifestPath)
				if rollback == nil {
					rollback = syncDirectory(m.cfg.Dir)
				}
			} else {
				rollback = Atomic(manifestPath, prior)
			}
			if rollback != nil {
				return fmt.Errorf("manifest commit failed: %v; prior manifest rollback failed, repair storage before restart: %w", e, rollback)
			}
		}
		return e
	}
	m.current = state
	m.currentDoc = []byte(doc)
	m.notifyLocked()
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
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	d, e := os.Open(path)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
