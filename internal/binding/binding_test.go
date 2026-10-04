package binding

import (
	"crypto/x509"
	"github.com/cztamas/qwacamole/internal/pki"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnnexBAndStrictValidity(t *testing.T) {
	d := t.TempDir()
	if e := pki.Generate(d, pki.Identity{Host: "qwac.example.com", Organization: "Synthetic TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); e != nil {
		t.Fatal(e)
	}
	m, e := pki.Load(d)
	if e != nil {
		t.Fatal(e)
	}
	tls, e := pki.ReadCert(filepath.Join(d, "tls-leaf.pem"))
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Chain[0].IssuingCertificateURL) != 1 || len(m.Chain[0].CRLDistributionPoints) != 1 {
		t.Fatal("missing profile AIA/CDP")
	}
	crlDER, e := os.ReadFile(filepath.Join(d, "qwac-intermediate.crl"))
	if e != nil {
		t.Fatal(e)
	}
	crl, e := x509.ParseRevocationList(crlDER)
	if e != nil {
		t.Fatal(e)
	}
	if e = crl.CheckSignatureFrom(m.Chain[1]); e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	doc, h, e := Sign(m, []*x509.Certificate{tls}, now, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Split(doc, ".")[1] != "" {
		t.Fatal("payload not detached")
	}
	if e = Verify(doc, "qwac.example.com", []*x509.Certificate{tls}, m.Chain[2], now); e != nil {
		t.Fatal(e)
	}
	if TLSHash(tls) == ID(tls) {
		t.Fatal("TLS hash incorrectly hashes raw DER")
	}
	h.Exp = now.Add(-time.Second).Unix()
	bad, _, e := SignHeader(m, h)
	if e != nil {
		t.Fatal(e)
	}
	if e = Verify(bad, "qwac.example.com", []*x509.Certificate{tls}, m.Chain[2], now); e == nil {
		t.Fatal("expired binding accepted")
	}
	h.Exp = now.Add(48 * time.Hour).Unix()
	bad, _, _ = SignHeader(m, h)
	if e = Verify(bad, "qwac.example.com", []*x509.Certificate{tls}, m.Chain[2], now); e == nil {
		t.Fatal("effective expiry cap ignored")
	}
	if e = Verify(doc, "other.example.com", []*x509.Certificate{tls}, m.Chain[2], now); e == nil {
		t.Fatal("wrong hostname accepted")
	}
	if e = Verify(doc, "qwac.example.com", []*x509.Certificate{m.Chain[1]}, m.Chain[2], now); e == nil {
		t.Fatal("unbound TLS accepted")
	}
}
