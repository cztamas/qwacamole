package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/profile"
)

func TestCredentialCommandsPrepareInspectImportWithoutDeployment(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.pem")
	var out bytes.Buffer
	if e := credentialCommand("keygen", []string{"-out", keyPath, "-bits", "2048"}, &out); e != nil {
		t.Fatal(e)
	}
	keyBytes, e := os.ReadFile(keyPath)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out.String(), "PRIVATE KEY") {
		t.Fatal("key printed")
	}
	csrPath := filepath.Join(dir, "csr.pem")
	if e = credentialCommand("csr", []string{"-key", keyPath, "-out", csrPath, "-country", "HU", "-organization", "TEST operator", "-registration", "NTRHU-TEST", "-common-name", "example.test", "-hostname", "example.test", "-hostname", "second.example.test"}, &out); e != nil {
		t.Fatal(e)
	}
	csrBytes, e := os.ReadFile(csrPath)
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode(csrBytes)
	csr, e := x509.ParseCertificateRequest(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	if e = csr.CheckSignature(); e != nil {
		t.Fatal(e)
	}
	key, e := credentials.ReadKey(keyPath)
	if e != nil {
		t.Fatal(e)
	}
	caKey, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TEST CA", Country: []string{"HU"}, Organization: []string{"TEST CA"}}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	rootDER, e := x509.CreateCertificate(rand.Reader, root, root, &caKey.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	root, e = x509.ParseCertificate(rootDER)
	if e != nil {
		t.Fatal(e)
	}
	types, _ := asn1.Marshal([]asn1.ObjectIdentifier{profile.WebType})
	statements, _ := asn1.Marshal([]profile.Statement{{ID: profile.Compliance}, {ID: profile.QCType, Info: asn1.RawValue{FullBytes: types}}})
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: csr.Subject, DNSNames: csr.DNSNames, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, UnknownExtKeyUsage: []asn1.ObjectIdentifier{profile.BindingEKU}, Policies: []x509.OID{profile.PolicyOID}, IssuingCertificateURL: []string{"https://example.test/issuer"}, CRLDistributionPoints: []string{"https://example.test/crl"}, ExtraExtensions: []pkix.Extension{{Id: profile.QCStatements, Value: statements}}}
	leaf.Subject.ExtraNames = csr.Subject.Names
	leafDER, e := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	chainPath, trustPath := filepath.Join(dir, "leaf.pem"), filepath.Join(dir, "root.pem")
	for path, data := range map[string][]byte{chainPath: leafDER, trustPath: rootDER} {
		if e = credentials.CreateFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: data})); e != nil {
			t.Fatal(e)
		}
	}
	args := []string{"-chain", chainPath, "-key", keyPath, "-trust", trustPath, "-hostname", "example.test", "-hostname", "second.example.test"}
	out.Reset()
	if e = credentialCommand("inspect-qwac", args, &out); e != nil {
		t.Fatal(e)
	}
	var report map[string]any
	if e = json.Unmarshal(out.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	if report["valid"] != true || report["chain_certificates"] != float64(2) || !strings.Contains(report["validation_scope"].(string), "not established") {
		t.Fatal(report)
	}
	importDir := filepath.Join(dir, "imported")
	out.Reset()
	if e = credentialCommand("import-qwac", append(args, "-out", importDir), &out); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out.String(), "PRIVATE KEY") {
		t.Fatal("import printed key")
	}
	importedKey := filepath.Join(importDir, "key.pem")
	loaded, e := credentials.LoadFor([]string{filepath.Join(importDir, "chain.pem")}, importedKey, []string{trustPath}, csr.DNSNames, now)
	if e != nil || !loaded.Key.Equal(key) {
		t.Fatal("import changed credential", e)
	}
	for _, path := range []string{importDir, importedKey, filepath.Join(importDir, "chain.pem")} {
		st, e := os.Stat(path)
		if e != nil || st.Mode().Perm()&0077 != 0 {
			t.Fatal("public import permissions", path, e)
		}
	}
	before, e := os.ReadFile(importedKey)
	if e != nil {
		t.Fatal(e)
	}
	if e = credentialCommand("import-qwac", append(args, "-out", importDir), &out); e == nil {
		t.Fatal("overwrote imported credential")
	}
	after, _ := os.ReadFile(importedKey)
	if !bytes.Equal(before, after) {
		t.Fatal("failed import changed prior key")
	}
	invalidDir := filepath.Join(dir, "rejected")
	out.Reset()
	if e = credentialCommand("import-qwac", append(args, "-hostname", "missing.example.test", "-out", invalidDir), &out); e == nil {
		t.Fatal("imported invalid candidate")
	}
	if _, e = os.Stat(invalidDir); !os.IsNotExist(e) {
		t.Fatal("invalid import left output", e)
	}
	if out.Len() != 0 {
		t.Fatal("rejected credential printed success")
	}
	if e = credentialCommand("inspect-qwac", args[:len(args)-2], &out); e != nil {
		t.Fatal(e)
	}
	if e = credentialCommand("inspect-qwac", []string{"-chain", chainPath, "-key", keyPath, "-hostname", "example.test"}, &out); e == nil {
		t.Fatal("implicit trust accepted")
	}
	actual, _ := os.ReadFile(keyPath)
	if !bytes.Equal(actual, keyBytes) {
		t.Fatal("source key modified")
	}
	actual, _ = os.ReadFile(trustPath)
	if !bytes.Equal(actual, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})) {
		t.Fatal("trust modified")
	}
}
