// Package credentials loads explicitly supplied RS256 signing material.
package credentials

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/cztamas/qwacamole/profile"
	"os"
	"time"
)

// Identity contains the signed organization metadata retained in binding state.
type Identity struct{ Host, Organization, Country, Locality, Registration string }

// Credentials is an RSA signing key and a leaf-first chain including its authority.
// Treat the supplied material as immutable while a manager is using it.
type Credentials struct {
	Chain []*x509.Certificate
	Key   *rsa.PrivateKey
}

func ReadCert(path string) (*x509.Certificate, error) {
	d, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	b, _ := pem.Decode(d)
	if b == nil {
		return nil, fmt.Errorf("not PEM: %s", path)
	}
	return x509.ParseCertificate(b.Bytes)
}
func ReadKey(path string) (*rsa.PrivateKey, error) {
	d, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	b, rest := pem.Decode(d)
	if b == nil || !bytes.HasPrefix(bytes.TrimSpace(d), []byte("-----BEGIN ")) {
		return nil, fmt.Errorf("not PEM key")
	}
	if len(bytes.TrimSpace(rest)) != 0 || len(b.Headers) != 0 {
		return nil, fmt.Errorf("one unencrypted RSA key required")
	}
	switch b.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(b.Bytes)
	case "PRIVATE KEY":
		key, e := x509.ParsePKCS8PrivateKey(b.Bytes)
		if e != nil {
			return nil, e
		}
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	return nil, fmt.Errorf("only unencrypted PKCS#1 or PKCS#8 RSA keys supported")
}

// LoadFor is the shared production entry point. ChainFiles is leaf-first PEM;
// the root may be omitted, but must be configured separately in trustFiles.
// One explicit authority is supported by the binding store. No system roots,
// network retrieval, trusted-list qualification or revocation checks are implied.
func LoadFor(chainFiles []string, keyFile string, trustFiles, hosts []string, now time.Time) (*Credentials, error) {
	if len(trustFiles) != 1 {
		return nil, fmt.Errorf("one explicit QWAC trust authority file required")
	}
	anchors, e := ReadCertificates(trustFiles[0])
	if e != nil {
		return nil, fmt.Errorf("QWAC trust: %w", e)
	}
	if len(anchors) != 1 || !anchors[0].IsCA {
		return nil, fmt.Errorf("one QWAC CA authority certificate required")
	}
	m := &Credentials{}
	for _, path := range chainFiles {
		cs, err := ReadCertificates(path)
		if err != nil {
			return nil, fmt.Errorf("QWAC chain: %w", err)
		}
		m.Chain = append(m.Chain, cs...)
	}
	if len(m.Chain) == 0 {
		return nil, fmt.Errorf("QWAC leaf certificate required")
	}
	if !m.Chain[len(m.Chain)-1].Equal(anchors[0]) {
		m.Chain = append(m.Chain, anchors[0])
	}
	m.Key, e = ReadKey(keyFile)
	if e != nil {
		return nil, fmt.Errorf("QWAC signing key: %w", e)
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("at least one binding hostname required")
	}
	for _, host := range hosts {
		if host == "" {
			return nil, fmt.Errorf("empty binding hostname")
		}
		if e = m.ValidateFor(host, anchors[0], now); e != nil {
			return nil, fmt.Errorf("QWAC credential for %s: %w", host, e)
		}
	}
	return m, nil
}

// ReadCertificates reads every certificate in an ordinary PEM bundle.
func ReadCertificates(path string) ([]*x509.Certificate, error) {
	d, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var cs []*x509.Certificate
	for len(bytes.TrimSpace(d)) > 0 {
		if !bytes.HasPrefix(bytes.TrimSpace(d), []byte("-----BEGIN CERTIFICATE-----")) {
			return nil, fmt.Errorf("unexpected PEM data in %s", path)
		}
		b, rest := pem.Decode(d)
		if b == nil || b.Type != "CERTIFICATE" || len(b.Headers) != 0 {
			return nil, fmt.Errorf("invalid certificate PEM in %s", path)
		}
		c, e := x509.ParseCertificate(b.Bytes)
		if e != nil {
			return nil, e
		}
		cs = append(cs, c)
		d = rest
	}
	if len(cs) == 0 {
		return nil, fmt.Errorf("empty certificate bundle: %s", path)
	}
	return cs, nil
}

// Load reads leaf-first certificates (bundles or separate files) and an RSA key.
func Load(chainFiles []string, keyFile string) (*Credentials, error) {
	m := &Credentials{}
	for _, path := range chainFiles {
		cs, e := ReadCertificates(path)
		if e != nil {
			return nil, e
		}
		m.Chain = append(m.Chain, cs...)
	}
	k, e := ReadKey(keyFile)
	if e != nil {
		return nil, e
	}
	m.Key = k
	if e = m.Validate(); e != nil {
		return nil, e
	}
	return m, nil
}

// ValidateFor checks local consistency against a separately supplied authority.
// This does not establish browser trust, revocation status or legal qualification.
func (m *Credentials) ValidateFor(host string, authority *x509.Certificate, now time.Time) error {
	if e := m.Validate(); e != nil {
		return e
	}
	if authority == nil || !authority.IsCA || !m.Chain[len(m.Chain)-1].Equal(authority) {
		return fmt.Errorf("chain must end in the explicit QWAC authority")
	}
	if e := profile.Validate(m.Chain[0]); e != nil {
		return e
	}
	if e := m.Chain[0].VerifyHostname(host); e != nil {
		return e
	}
	for n, c := range m.Chain {
		if now.Before(c.NotBefore) || !now.Before(c.NotAfter) {
			return fmt.Errorf("QWAC chain certificate %d outside validity", n)
		}
		if n+1 < len(m.Chain) {
			if e := c.CheckSignatureFrom(m.Chain[n+1]); e != nil {
				return fmt.Errorf("QWAC chain is not leaf-first: %w", e)
			}
		}
	}
	roots, intermediates := x509.NewCertPool(), x509.NewCertPool()
	roots.AddCert(authority)
	for _, c := range m.Chain[1 : len(m.Chain)-1] {
		intermediates.AddCert(c)
	}
	_, e := m.Chain[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	return e
}

// Validate rejects missing or mismatched signing material before use.
func (m *Credentials) Validate() error {
	if m == nil || m.Key == nil || len(m.Chain) < 2 {
		return fmt.Errorf("signing key and leaf-first QWAC chain including authority required")
	}
	for _, c := range m.Chain {
		if c == nil {
			return fmt.Errorf("nil certificate in signing chain")
		}
	}
	if e := m.Key.Validate(); e != nil {
		return e
	}
	if m.Key.N.BitLen() < 2048 {
		return fmt.Errorf("RS256 signing requires an RSA key of at least 2048 bits")
	}
	pub, ok := m.Chain[0].PublicKey.(*rsa.PublicKey)
	if !ok || !pub.Equal(&m.Key.PublicKey) {
		return fmt.Errorf("signing key does not match RSA QWAC leaf")
	}
	return nil
}
