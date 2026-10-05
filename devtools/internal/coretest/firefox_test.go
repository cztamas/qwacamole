package coretest

import (
	"crypto/x509"
	"encoding/base64"
	. "github.com/cztamas/qwacamole/binding"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/devtools/internal/pki"
)

func TestFirefoxCompatibilityKeepsCryptoChecksAndStrictProfileSeparate(t *testing.T) {
	dir := t.TempDir()
	id := pki.Identity{Host: "qwac.example.com", Organization: "Firefox Fixture TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}
	if err := pki.GenerateFirefox(dir, id); err != nil {
		t.Fatal(err)
	}
	m, err := pki.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := pki.ReadCert(filepath.Join(dir, "tls-leaf.pem"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cs := []*x509.Certificate{leaf}
	root := m.Chain[len(m.Chain)-1]
	doc, h, err := SignFirefox(m, cs, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Crit) != 0 || m.Chain[0].Subject.Organization[0] != id.Organization {
		t.Fatal("compatibility header or signed owner differs")
	}
	if err := VerifyFirefox(doc, id.Host, cs, root, now); err != nil {
		t.Fatal(err)
	}
	if err := Verify(doc, id.Host, cs, root, now); err == nil {
		t.Fatal("Firefox variant accepted by the strict profile")
	}
	if err := VerifyFirefox(doc, "other.example.com", cs, root, now); err == nil {
		t.Fatal("hostname check bypassed")
	}
	if err := VerifyFirefox(doc, id.Host, []*x509.Certificate{m.Chain[1]}, root, now); err == nil {
		t.Fatal("binding hash check bypassed")
	}
	if err := VerifyFirefox(doc, id.Host, cs, root, now.Add(2*time.Minute)); err == nil {
		t.Fatal("binding expiry bypassed")
	}
	parts := strings.Split(doc, ".")
	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	signature[0] ^= 1
	parts[2] = base64.RawURLEncoding.EncodeToString(signature)
	if err := VerifyFirefox(strings.Join(parts, "."), id.Host, cs, root, now); err == nil {
		t.Fatal("signature check bypassed")
	}
	standard, strict, err := Sign(m, cs, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict.Crit) != 1 || strict.Crit[0] != "sigD" {
		t.Fatal("standard critical header changed")
	}
	if err := Verify(standard, id.Host, cs, root, now); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFirefox(standard, id.Host, cs, root, now); err == nil {
		t.Fatal("Firefox profile unexpectedly accepts crit")
	}
	for _, name := range []string{"qwac-root", "qwac-intermediate"} {
		data, err := os.ReadFile(filepath.Join(dir, name+".crl"))
		if err != nil {
			t.Fatal(err)
		}
		crl, err := x509.ParseRevocationList(data)
		if err != nil {
			t.Fatal(err)
		}
		issuer, err := pki.ReadCert(filepath.Join(dir, name+".pem"))
		if err != nil {
			t.Fatal(err)
		}
		if err := crl.CheckSignatureFrom(issuer); err != nil {
			t.Fatal(err)
		}
	}
}
