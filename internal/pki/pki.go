// Package pki creates synthetic test certificates. Nothing here is legally qualified.
package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

var PolicyOID, _ = x509.ParseOID("0.4.0.194112.1.6")

var Policy = asn1.ObjectIdentifier{0, 4, 0, 194112, 1, 6}
var BindingEKU = asn1.ObjectIdentifier{0, 4, 0, 194115, 1, 0}
var QCStatements = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}
var Compliance = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 1}
var QCType = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6}
var WebType = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 3}

type Statement struct {
	ID   asn1.ObjectIdentifier
	Info asn1.RawValue `asn1:"optional"`
}
type Identity struct{ Host, Organization, Country, Locality, Registration string }
type Material struct {
	Chain []*x509.Certificate
	Key   *rsa.PrivateKey
}

func Serial() *big.Int {
	n, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	if e != nil {
		panic(e)
	}
	return n
}
func Template(name string, ca bool) *x509.Certificate {
	t := &x509.Certificate{SerialNumber: Serial(), Subject: pkix.Name{CommonName: name, Organization: []string{"Qwacamole TEST PKI - NOT QUALIFIED"}, Country: []string{"HU"}, ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 97}, Value: "NTRHU-TEST-CA-0001"}}}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(24 * time.Hour), BasicConstraintsValid: true, IsCA: ca, KeyUsage: x509.KeyUsageDigitalSignature, SignatureAlgorithm: x509.SHA256WithRSA}
	if ca {
		t.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
		t.MaxPathLen = 1
	}
	return t
}
func Issue(t, parent *x509.Certificate, pub *rsa.PublicKey, key *rsa.PrivateKey) (*x509.Certificate, error) {
	d, e := x509.CreateCertificate(rand.Reader, t, parent, pub, key)
	if e != nil {
		return nil, e
	}
	return x509.ParseCertificate(d)
}
func WriteCert(path string, c *x509.Certificate) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}), 0600)
}
func WriteKey(path string, k *rsa.PrivateKey) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}), 0600)
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
func Load(dir string) (*Material, error) {
	m := &Material{}
	for _, n := range []string{"qwac-leaf.pem", "qwac-intermediate.pem", "qwac-root.pem"} {
		c, e := ReadCert(filepath.Join(dir, n))
		if e != nil {
			return nil, e
		}
		m.Chain = append(m.Chain, c)
	}
	k, e := ReadKey(filepath.Join(dir, "qwac-leaf-key.pem"))
	m.Key = k
	return m, e
}
func Generate(dir string, id Identity) error {
	return generate(dir, id, nil, nil)
}

func generate(dir string, id Identity, qwacRoot *x509.Certificate, qwacRootKey *rsa.PrivateKey) error {
	if _, e := os.Stat(filepath.Join(dir, "qwac-root.pem")); e == nil {
		return fmt.Errorf("test PKI exists; use scoped clean before regeneration")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	for _, group := range []string{"qwac", "tls", "acme"} {
		root, rk := qwacRoot, qwacRootKey
		var e error
		if group != "qwac" || root == nil {
			rk, e = rsa.GenerateKey(rand.Reader, 3072)
			if e != nil {
				return e
			}
			rt := Template(group+" Qwacamole TEST Root", true)
			root, e = Issue(rt, rt, &rk.PublicKey, rk)
			if e != nil {
				return e
			}
		}
		if e = WriteCert(filepath.Join(dir, group+"-root.pem"), root); e != nil {
			return e
		}
		if e = WriteKey(filepath.Join(dir, group+"-root-key.pem"), rk); e != nil {
			return e
		}
		parent, signer := root, rk
		if group == "qwac" {
			ik, e := rsa.GenerateKey(rand.Reader, 3072)
			if e != nil {
				return e
			}
			it := Template("Qwacamole TEST QWAC Issuing CA", true)
			it.MaxPathLen = 0
			it.MaxPathLenZero = true
			it.Policies = []x509.OID{PolicyOID}
			it.IssuingCertificateURL = []string{"http://qwac.example.com:5002/test-pki/qwac-root.cer"}
			it.CRLDistributionPoints = []string{"http://qwac.example.com:5002/test-pki/qwac-root.crl"}
			ic, e := Issue(it, root, &ik.PublicKey, rk)
			if e != nil {
				return e
			}
			if e = WriteCert(filepath.Join(dir, "qwac-intermediate.pem"), ic); e != nil {
				return e
			}
			if e = WriteKey(filepath.Join(dir, "qwac-intermediate-key.pem"), ik); e != nil {
				return e
			}
			parent, signer = ic, ik
			for name, authority := range map[string]struct {
				cert *x509.Certificate
				key  *rsa.PrivateKey
			}{"qwac-root": {root, rk}, "qwac-intermediate": {ic, ik}} {
				if e = os.WriteFile(filepath.Join(dir, name+".cer"), authority.cert.Raw, 0600); e != nil {
					return e
				}
				crlIssuer := *authority.cert
				if len(crlIssuer.SubjectKeyId) == 0 {
					// The Mozilla fixture omits SKI. Derive the conventional key
					// identifier for CRL AKI without changing the root's DER.
					identifier := sha1.Sum(x509.MarshalPKCS1PublicKey(&authority.key.PublicKey))
					crlIssuer.SubjectKeyId = identifier[:]
				}
				crl, e := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: time.Now().Add(-time.Minute), NextUpdate: time.Now().Add(23 * time.Hour)}, &crlIssuer, authority.key)
				if e != nil {
					return e
				}
				if e = os.WriteFile(filepath.Join(dir, name+".crl"), crl, 0600); e != nil {
					return e
				}
			}

		}
		lk, e := rsa.GenerateKey(rand.Reader, 2048)
		if e != nil {
			return e
		}
		lt := Template(id.Host+" TEST", false)
		lt.DNSNames = []string{id.Host}
		lt.Subject = pkix.Name{CommonName: id.Host, Organization: []string{id.Organization}, Country: []string{id.Country}, Locality: []string{id.Locality}, ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 97}, Value: id.Registration}}}
		if group == "qwac" {
			lt.UnknownExtKeyUsage = []asn1.ObjectIdentifier{BindingEKU}
			lt.IssuingCertificateURL = []string{"http://qwac.example.com:5002/test-pki/qwac-intermediate.cer"}
			lt.CRLDistributionPoints = []string{"http://qwac.example.com:5002/test-pki/qwac-intermediate.crl"}
			lt.Policies = []x509.OID{PolicyOID}
			types, _ := asn1.Marshal([]asn1.ObjectIdentifier{WebType})
			pds, _ := asn1.Marshal([]struct {
				URL      string `asn1:"ia5"`
				Language string `asn1:"printable"`
			}{{"https://qwac.example.com:8443/test-pki-notice", "en"}})
			qc, e := asn1.Marshal([]Statement{{ID: Compliance}, {ID: QCType, Info: asn1.RawValue{FullBytes: types}}, {ID: asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 5}, Info: asn1.RawValue{FullBytes: pds}}})
			if e != nil {
				return e
			}
			lt.ExtraExtensions = []pkix.Extension{{Id: QCStatements, Value: qc}}
		} else {
			lt.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			if group == "acme" {
				lt.DNSNames = []string{"localhost"}
			}
		}
		leaf, e := Issue(lt, parent, &lk.PublicKey, signer)
		if e != nil {
			return e
		}
		if e = WriteCert(filepath.Join(dir, group+"-leaf.pem"), leaf); e != nil {
			return e
		}
		if e = WriteKey(filepath.Join(dir, group+"-leaf-key.pem"), lk); e != nil {
			return e
		}
	}
	return nil
}
