package credentials

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/profile"
)

func issue(t *testing.T, template, parent *x509.Certificate, public *rsa.PublicKey, signer *rsa.PrivateKey) *x509.Certificate {
	t.Helper()
	d, e := x509.CreateCertificate(rand.Reader, template, parent, public, signer)
	if e != nil {
		t.Fatal(e)
	}
	c, e := x509.ParseCertificate(d)
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func qc(statements ...profile.Statement) pkix.Extension {
	d, e := asn1.Marshal(statements)
	if e != nil {
		panic(e)
	}
	return pkix.Extension{Id: profile.QCStatements, Value: d}
}

func webStatement(types ...asn1.ObjectIdentifier) profile.Statement {
	d, e := asn1.Marshal(types)
	if e != nil {
		panic(e)
	}
	return profile.Statement{ID: profile.QCType, Info: asn1.RawValue{FullBytes: d}}
}

func TestCredentialValidationFailuresAndExplicitTrust(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	caKey, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	subject := pkix.Name{CommonName: "Test authority", Country: []string{"HU"}, Organization: []string{"TEST ONLY"}}
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: subject, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	root := issue(t, rootTemplate, rootTemplate, &caKey.PublicKey, caKey)
	intermediateTemplate := *rootTemplate
	intermediateTemplate.SerialNumber = big.NewInt(2)
	intermediateTemplate.Subject.CommonName = "Test intermediate"
	intermediate := issue(t, &intermediateTemplate, root, &caKey.PublicKey, caKey)
	leafTemplate := func() *x509.Certificate {
		return &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "example.test", Country: []string{"HU"}, Organization: []string{"Example TEST"}, ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 97}, Value: "NTRHU-TEST"}}},
			DNSNames: []string{"example.test", "second.example.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
			UnknownExtKeyUsage: []asn1.ObjectIdentifier{profile.BindingEKU}, Policies: []x509.OID{profile.PolicyOID},
			IssuingCertificateURL: []string{"https://example.test/issuer"}, CRLDistributionPoints: []string{"https://example.test/crl"},
			ExtraExtensions: []pkix.Extension{qc(profile.Statement{ID: profile.Compliance}, webStatement(profile.WebType))}}
	}
	for _, tc := range []struct {
		name, want string
		mutate     func(*x509.Certificate)
	}{
		{"valid", "", func(c *x509.Certificate) {}},
		{"TLS QWAC", "only tls-binding EKU", func(c *x509.Certificate) {
			c.UnknownExtKeyUsage = nil
			c.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		}},
		{"additional EKU", "only tls-binding EKU", func(c *x509.Certificate) { c.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth} }},
		{"wrong policy", "QNCP-w-gen", func(c *x509.Certificate) {
			c.Policies = nil
			c.PolicyIdentifiers = []asn1.ObjectIdentifier{{0, 4, 0, 194112, 1, 4}}
		}},
		{"missing statements", "missing QWAC QCStatements", func(c *x509.Certificate) { c.ExtraExtensions = nil }},
		{"missing compliance", "missing QWAC QCStatements", func(c *x509.Certificate) { c.ExtraExtensions = []pkix.Extension{qc(webStatement(profile.WebType))} }},
		{"missing web type", "missing QWAC QCStatements", func(c *x509.Certificate) {
			c.ExtraExtensions = []pkix.Extension{qc(profile.Statement{ID: profile.Compliance})}
		}},
		{"seal", "only website", func(c *x509.Certificate) {
			c.ExtraExtensions = []pkix.Extension{qc(profile.Statement{ID: profile.Compliance}, webStatement(asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 2}))}
		}},
		{"mixed seal and web", "only website", func(c *x509.Certificate) {
			c.ExtraExtensions = []pkix.Extension{qc(profile.Statement{ID: profile.Compliance}, webStatement(profile.WebType, asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 2}))}
		}},
		{"duplicate statement", "duplicate QCStatement", func(c *x509.Certificate) {
			c.ExtraExtensions = []pkix.Extension{qc(profile.Statement{ID: profile.Compliance}, profile.Statement{ID: profile.Compliance}, webStatement(profile.WebType))}
		}},
		{"compliance info", "QcCompliance", func(c *x509.Certificate) {
			c.ExtraExtensions = []pkix.Extension{qc(profile.Statement{ID: profile.Compliance, Info: asn1.RawValue{FullBytes: []byte{5, 0}}}, webStatement(profile.WebType))}
		}},
		{"type trailing data", "invalid QCStatements", func(c *x509.Certificate) {
			s := webStatement(profile.WebType)
			s.Info.FullBytes = append(s.Info.FullBytes, 5, 0)
			c.ExtraExtensions = []pkix.Extension{qc(profile.Statement{ID: profile.Compliance}, s)}
		}},
		{"critical statements", "must not be critical", func(c *x509.Certificate) { c.ExtraExtensions[0].Critical = true }},
		{"malformed statements", "invalid QCStatements", func(c *x509.Certificate) { c.ExtraExtensions[0].Value = []byte{0x30, 1, 0xff} }},
		{"PSD2 incompatible usage", "incompatible 2-QWAC key usage", func(c *x509.Certificate) { c.KeyUsage |= x509.KeyUsageKeyEncipherment }},
		{"missing signing usage", "digitalSignature", func(c *x509.Certificate) { c.KeyUsage = x509.KeyUsageContentCommitment }},
		{"CA leaf", "end-entity", func(c *x509.Certificate) { c.IsCA = true; c.BasicConstraintsValid = true }},
		{"expired", "outside validity", func(c *x509.Certificate) { c.NotAfter = now }},
		{"not yet valid", "outside validity", func(c *x509.Certificate) { c.NotBefore = now.Add(time.Second) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			template := leafTemplate()
			tc.mutate(template)
			leaf := issue(t, template, intermediate, &key.PublicKey, caKey)
			m := &Credentials{Chain: []*x509.Certificate{leaf, intermediate, root}, Key: key}
			err := m.ValidateFor("example.test", root, now)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
	leaf := issue(t, leafTemplate(), intermediate, &key.PublicKey, caKey)
	m := &Credentials{Chain: []*x509.Certificate{leaf, intermediate, root}, Key: key}
	if e = m.ValidateFor("uncovered.example.test", root, now); e == nil || !strings.Contains(e.Error(), "not") {
		t.Fatal("hostname mismatch", e)
	}
	otherTemplate := *rootTemplate
	otherTemplate.SerialNumber = big.NewInt(99)
	untrusted := issue(t, &otherTemplate, &otherTemplate, &key.PublicKey, key)
	if e = m.ValidateFor("example.test", untrusted, now); e == nil || !strings.Contains(e.Error(), "explicit QWAC authority") {
		t.Fatal("untrusted chain", e)
	}
	m.Key = caKey
	if e = m.ValidateFor("example.test", root, now); e == nil || !strings.Contains(e.Error(), "does not match") {
		t.Fatal("key mismatch", e)
	}
	m.Key = key
	m.Chain = []*x509.Certificate{leaf, root}
	if e = m.ValidateFor("example.test", root, now); e == nil {
		t.Fatal("missing intermediate accepted")
	}
	// The root and intermediate deliberately share a public key, so signature
	// checks alone pass; full path building must still reject the issuer mismatch.
	dir := t.TempDir()
	write := func(name string, certs ...*x509.Certificate) string {
		path := filepath.Join(dir, name)
		var data []byte
		for _, c := range certs {
			data = append(data, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
		}
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
		return path
	}
	bundle := write("chain.pem", leaf, intermediate)
	trust := write("root.pem", root)
	private := filepath.Join(dir, "key.pem")
	if e = CreateFile(private, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})); e != nil {
		t.Fatal(e)
	}
	loaded, e := LoadFor([]string{bundle}, private, []string{trust}, []string{"example.test", "second.example.test"}, now)
	if e != nil || len(loaded.Chain) != 3 {
		t.Fatal("omitted root completion", e)
	}
	for _, hosts := range [][]string{nil, {""}, {"example.test", "missing.example.test"}} {
		if _, e = LoadFor([]string{bundle}, private, []string{trust}, hosts, now); e == nil {
			t.Fatal("missing/unchecked hostname accepted", hosts)
		}
	}
	write("chain.pem", leaf, intermediate, root)
	if _, e = LoadFor([]string{bundle}, private, []string{trust}, []string{"example.test"}, now); e != nil {
		t.Fatal("existing full chain compatibility", e)
	}
	if _, e = LoadFor([]string{bundle}, private, nil, []string{"example.test"}, now); e == nil {
		t.Fatal("implicit trust accepted")
	}
	write("root.pem", root, untrusted)
	if _, e = LoadFor([]string{bundle}, private, []string{trust}, []string{"example.test"}, now); e == nil {
		t.Fatal("multiple anchors accepted")
	}
}
