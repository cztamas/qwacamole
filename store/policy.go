package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cztamas/qwacamole/binding"
)

// WithDefaults validates the operational policy. Explicit legacy lifetimes of
// at least 3s remain supported; omitted lifetime now selects a deployment hour.
func (c Config) WithDefaults() (Config, error) {
	if c.Lifetime == 0 {
		c.Lifetime = time.Hour
	}
	if c.RefreshBefore == 0 {
		c.RefreshBefore = c.Lifetime / 3
	}
	if c.DocumentGrace == 0 {
		c.DocumentGrace = 24 * time.Hour
	}
	if c.MaxCertificates == 0 {
		c.MaxCertificates = 256
	}
	if c.MaxDocumentBytes == 0 {
		c.MaxDocumentBytes = 1 << 20
	}
	if c.Lifetime < 3*time.Second || c.RefreshBefore < time.Second || c.RefreshBefore > c.Lifetime/2 {
		return c, fmt.Errorf("lifetime must be at least 3s and refresh_before must be between 1s and half the lifetime")
	}
	if c.DocumentGrace < time.Second || c.MaxCertificates < 1 || c.MaxDocumentBytes < 1024 {
		return c, fmt.Errorf("document_grace must be at least 1s, max_certificates positive and max_document_bytes at least 1024")
	}
	return c, nil
}

func (m *Manager) nextRefreshLocked() time.Time {
	if m.current.Expires == 0 {
		return time.Now().Add(time.Hour)
	}
	margin := m.cfg.RefreshBefore
	// A certificate may cap validity below the configured usable margin. Do not
	// continuously re-sign an identical near-expiry document; renewal/replacement
	// wakes the worker, otherwise the next attempt fails closed at expiry.
	if m.current.Expires-m.current.Published < 3 {
		return time.Unix(m.current.Expires, 0)
	}
	effective := time.Duration(m.current.Expires-m.current.Published) * time.Second / 3
	if effective < margin {
		margin = effective
	}
	if margin < time.Second {
		margin = time.Second
	}
	return time.Unix(m.current.Expires, 0).Add(-margin)
}

// NextRefresh is computed from the committed signed lifetime, including TLS and
// QWAC expiry caps. Changed wakes a scheduler after a new committed publication.
func (m *Manager) NextRefresh() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nextRefreshLocked()
}

// Changed is a coalescing notification for the single owning scheduler.
func (m *Manager) Changed() <-chan struct{} { return m.changed }
func (m *Manager) notifyLocked() {
	if m.changed != nil {
		select {
		case m.changed <- struct{}{}:
		default:
		}
	}
}

// CollectionInterval bounds collection work independently from refresh rate.
func (m *Manager) CollectionInterval() time.Duration {
	v := m.cfg.DocumentGrace / 4
	if v < time.Second {
		v = time.Second
	}
	if v > time.Hour {
		v = time.Hour
	}
	return v
}

// Collection reports only deleted, authenticated immutable artifacts.
type Collection struct {
	Documents int
	Bytes     int64
}

// Collect serializes with publication and readers. Only hash-named regular files
// authenticated under this host/authority/profile can be removed. The current
// artifact, staged writes, unknown files, and documents within either expiry or
// modification-time grace are preserved. TLS coverage is never collected here.
func (m *Manager) Collect(now time.Time) (Collection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out Collection
	entries, e := os.ReadDir(m.cfg.Dir)
	if e != nil {
		return out, e
	}
	for _, entry := range entries {
		name := entry.Name()
		if !immutablePath("/bindings/"+name) || name == filepath.Base(m.current.URL) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return out, err
		}
		if !info.Mode().IsRegular() || info.Size() > m.cfg.MaxDocumentBytes || now.Before(info.ModTime().Add(m.cfg.DocumentGrace)) {
			continue
		}
		d, err := m.readDocumentLocked(name)
		if err != nil {
			continue
		} // Never infer ownership from filename alone.
		h, err := binding.InspectDocument(string(d), m.cfg.Host, m.material.Chain[len(m.material.Chain)-1], m.cfg.FirefoxCompatibility)
		if err != nil || now.Before(time.Unix(h.Exp, 0).Add(m.cfg.DocumentGrace)) {
			continue
		}
		if err = os.Remove(filepath.Join(m.cfg.Dir, name)); err != nil {
			return out, err
		}
		out.Documents++
		out.Bytes += info.Size()
	}
	return out, nil
}

// Status supplies local diagnostics for logs/tools; it is not browser evidence.
// Storage includes unknown files so unexpected growth remains observable.
type Status struct {
	SignerID             string   `json:"active_qwac_sha256,omitempty"`
	CredentialID         string   `json:"configured_qwac_sha256"`
	ReplacementPending   bool     `json:"replacement_pending"`
	TLSIDs               []string `json:"covered_tls_sha256"`
	QWACExpires          int64    `json:"qwac_expires"`
	QWACChainExpires     int64    `json:"qwac_chain_expires"`
	BindingExpires       int64    `json:"binding_expires"`
	NextRefresh          int64    `json:"next_refresh"`
	RetainedCertificates int      `json:"retained_certificates"`
	MaxCertificates      int      `json:"max_certificates"`
	DocumentBytes        int      `json:"current_document_bytes"`
	Files                int      `json:"storage_files"`
	StorageBytes         int64    `json:"storage_bytes"`
	LastFailure          string   `json:"last_publication_failure,omitempty"`
	MaxDocumentBytes     int64    `json:"max_document_bytes"`
	RefreshFailures      int      `json:"refresh_failures"`
	RetryAt              int64    `json:"refresh_retry_at,omitempty"`
	RefreshFailure       string   `json:"last_refresh_failure,omitempty"`
}

// Maintenance records the single worker's retry state for local diagnostics.
// It does not alter signed state, schedule work or notify the worker.
func (m *Manager) Maintenance(retryAt time.Time, failures int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryAt, m.refreshFailures = retryAt, failures
	m.refreshFailure = ""
	if err != nil {
		m.refreshFailure = err.Error()
	}
}

func (m *Manager) Status() (Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := Status{QWACExpires: m.material.Chain[0].NotAfter.Unix(), BindingExpires: m.current.Expires, NextRefresh: m.nextRefreshLocked().Unix(), RetainedCertificates: len(m.current.Certs), MaxCertificates: m.cfg.MaxCertificates, DocumentBytes: len(m.currentDoc), LastFailure: m.lastFailure}
	s.CredentialID = binding.ID(m.material.Chain[0])
	s.TLSIDs = append([]string(nil), m.current.IDs...)
	s.MaxDocumentBytes = m.cfg.MaxDocumentBytes
	s.RefreshFailures, s.RefreshFailure = m.refreshFailures, m.refreshFailure
	if !m.retryAt.IsZero() {
		s.RetryAt = m.retryAt.Unix()
	}
	chain := m.material.Chain
	if len(m.currentDoc) > 0 {
		h, committed, _, _, err := binding.Decode(string(m.currentDoc))
		if err != nil {
			return s, err
		}
		s.SignerID, chain = h.Kid, committed
		s.QWACExpires = chain[0].NotAfter.Unix()
	}
	s.ReplacementPending = s.SignerID != "" && s.SignerID != s.CredentialID
	s.QWACChainExpires = chain[0].NotAfter.Unix()
	for _, c := range chain {
		if c.NotAfter.Unix() < s.QWACChainExpires {
			s.QWACChainExpires = c.NotAfter.Unix()
		}
	}
	es, e := os.ReadDir(m.cfg.Dir)
	if e != nil {
		return s, e
	}
	for _, entry := range es {
		info, e := entry.Info()
		if e != nil {
			return s, e
		}
		if info.Mode().IsRegular() {
			s.Files++
			s.StorageBytes += info.Size()
		}
	}
	return s, nil
}
