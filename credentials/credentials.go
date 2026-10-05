// Package credentials loads explicitly supplied RS256 signing material.
package credentials

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
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
	b, _ := pem.Decode(d)
	if b == nil {
		return nil, fmt.Errorf("not PEM key")
	}
	return x509.ParsePKCS1PrivateKey(b.Bytes)
}

// Load reads a leaf-first chain including its trust anchor and a PKCS#1 RSA key.
func Load(chainFiles []string, keyFile string) (*Credentials, error) {
	m := &Credentials{}
	for _, path := range chainFiles {
		c, e := ReadCert(path)
		if e != nil {
			return nil, e
		}
		m.Chain = append(m.Chain, c)
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
	pub, ok := m.Chain[0].PublicKey.(*rsa.PublicKey)
	if !ok || !pub.Equal(&m.Key.PublicKey) {
		return fmt.Errorf("signing key does not match RSA QWAC leaf")
	}
	return nil
}
