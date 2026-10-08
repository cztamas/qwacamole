package credentials

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalKeyAndCSRKeepKeyAndRefuseOverwrite(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signer.pem")
	if e := GenerateKey(keyPath, 2048); e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(keyPath)
	if e != nil {
		t.Fatal(e)
	}
	st, e := os.Stat(keyPath)
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("key permissions", e)
	}
	if e = GenerateKey(keyPath, 2048); e == nil {
		t.Fatal("overwrote key")
	}
	if e = GenerateKey(filepath.Join(dir, "weak.pem"), 1024); e == nil {
		t.Fatal("generated weak key")
	}
	link := filepath.Join(dir, "link.pem")
	if e = os.Symlink(keyPath, link); e != nil {
		t.Fatal(e)
	}
	if e = GenerateKey(link, 2048); e == nil {
		t.Fatal("followed symlink")
	}
	key, e := ReadKey(keyPath)
	if e != nil {
		t.Fatal(e)
	}
	r := Request{Country: "HU", Organization: "Example TEST", Registration: "NTRHU-TEST", CommonName: "example.test", Locality: "Budapest", Province: "Pest", Street: "Test street", PostalCode: "1000", Hosts: []string{"example.test", "second.example.test"}}
	d, e := CSR(key, r)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := pem.Decode(d)
	csr, e := x509.ParseCertificateRequest(b.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	if e = csr.CheckSignature(); e != nil {
		t.Fatal(e)
	}
	if !key.PublicKey.Equal(csr.PublicKey) || len(csr.DNSNames) != 2 || csr.Subject.Organization[0] != r.Organization || csr.Subject.Country[0] != r.Country || csr.Subject.Locality[0] != r.Locality || csr.Subject.Province[0] != r.Province || csr.Subject.StreetAddress[0] != r.Street || csr.Subject.PostalCode[0] != r.PostalCode {
		t.Fatal("CSR lost requested fields")
	}
	found := false
	for _, n := range csr.Subject.Names {
		if n.Type.String() == "2.5.4.97" && n.Value == r.Registration {
			found = true
		}
	}
	if !found {
		t.Fatal("missing organizationIdentifier")
	}
	for _, oid := range []string{"2.5.29.15", "2.5.29.17", "2.5.29.32", "2.5.29.37"} {
		found = false
		for _, ext := range csr.Extensions {
			if ext.Id.String() == oid {
				found = true
			}
		}
		if !found {
			t.Fatal("missing requested extension", oid)
		}
	}
	csrPath := filepath.Join(dir, "request.pem")
	if e = CreateFile(csrPath, d); e != nil {
		t.Fatal(e)
	}
	if e = CreateFile(csrPath, []byte("overwrite")); e == nil {
		t.Fatal("overwrote CSR")
	}
	actual, _ := os.ReadFile(keyPath)
	if !bytes.Equal(actual, original) {
		t.Fatal("CSR or failed creation changed private key")
	}
	for _, hosts := range [][]string{nil, {"*.example.test"}, {"example.test:443"}, {"127.0.0.1"}, {"UPPER.test"}} {
		r.Hosts = hosts
		if _, e = CSR(key, r); e == nil {
			t.Fatal("invalid names", hosts)
		}
	}
	if _, e = CSR(nil, r); e == nil {
		t.Fatal("nil signer accepted")
	}
}
