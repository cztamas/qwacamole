package verification

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/profile"
)

func TestLiveVerificationAndFailedChecks(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Now()
	issue := func(c, parent *x509.Certificate) *x509.Certificate {
		der, e := x509.CreateCertificate(rand.Reader, c, parent, &key.PublicKey, key)
		if e != nil {
			t.Fatal(e)
		}
		cert, e := x509.ParseCertificate(der)
		if e != nil {
			t.Fatal(e)
		}
		return cert
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test", Organization: []string{"TEST ONLY"}, Country: []string{"HU"}}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	ca := issue(caTemplate, caTemplate)
	web, _ := asn1.Marshal([]asn1.ObjectIdentifier{profile.WebType})
	qc, _ := asn1.Marshal([]profile.Statement{{ID: profile.Compliance}, {ID: profile.QCType, Info: asn1.RawValue{FullBytes: web}}})
	q := issue(&x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "example.test", Country: []string{"HU"}, Organization: []string{"TEST ONLY"}, ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 97}, Value: "NTRHU-TEST"}}}, DNSNames: []string{"example.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, UnknownExtKeyUsage: []asn1.ObjectIdentifier{profile.BindingEKU}, Policies: []x509.OID{profile.PolicyOID}, IssuingCertificateURL: []string{"https://example.test/ca"}, CRLDistributionPoints: []string{"https://example.test/crl"}, ExtraExtensions: []pkix.Extension{{Id: profile.QCStatements, Value: qc}}}, ca)
	leaf := issue(&x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "example.test"}, DNSNames: []string{"example.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(20 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}, ca)
	material := &credentials.Credentials{Chain: []*x509.Certificate{q, ca}, Key: key}
	doc, _, e := binding.Sign(material, []*x509.Certificate{leaf}, now, time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	mode := "valid"
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if mode == "unready" {
				w.WriteHeader(503)
				return
			}
			if mode != "missing-link" {
				w.Header().Set("Link", `</bindings/current.jose>; rel="tls-certificate-binding"`)
			}
			return
		}
		w.Header().Set("Content-Type", "application/jose")
		w.Header().Set("Cache-Control", "no-store")
		if mode == "oversize" {
			w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
			return
		}
		if mode == "bad-binding" {
			w.Write([]byte("invalid"))
			return
		}
		w.Write([]byte(doc))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leaf.Raw}, PrivateKey: key}}}
	server.StartTLS()
	defer server.Close()
	dir := t.TempDir()
	authority := filepath.Join(dir, "ca.pem")
	os.WriteFile(authority, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}), 0600)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	options := Options{Hostname: "example.test", Authority: authority, Address: server.Listener.Addr().String(), TLSRoots: roots}
	for _, tc := range []struct{ mode, failed string }{{"valid", ""}, {"missing-link", "discovery"}, {"unready", "HTTPS"}, {"oversize", "discovery"}, {"bad-binding", "signed binding"}} {
		mode = tc.mode
		report, e := Verify(context.Background(), options)
		if tc.failed == "" {
			if e != nil || !report.Ready || len(report.Checks) != 4 {
				t.Fatal(report, e)
			}
		} else {
			if e == nil || report.Ready || len(report.Checks) == 0 || report.Checks[len(report.Checks)-1].Name != tc.failed || report.Checks[len(report.Checks)-1].Action == "" {
				t.Fatal(report, e)
			}
		}
	}
	mode = "valid"
	options.TLSRoots = nil
	report, e := Verify(context.Background(), options)
	if e == nil || report.Ready || report.Checks[0].Name != "HTTPS" {
		t.Fatal(report, e)
	}
	options.TLSRoots = roots
	options.Hostname = "wrong.test"
	if _, e = Verify(context.Background(), options); e == nil {
		t.Fatal("wrong hostname accepted")
	}
	options.Hostname = "example.test"
	options.FirefoxCompatibility = true
	if _, e = Verify(context.Background(), options); e == nil {
		t.Fatal("strict binding accepted as compatibility")
	}
}
