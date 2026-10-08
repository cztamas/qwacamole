package credentials

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadExplicitPathsAndRejectMismatchedKey(t *testing.T) {
	dir := t.TempDir()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "loader test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	cert := filepath.Join(dir, "arbitrary-chain-name.pem")
	private := filepath.Join(dir, "arbitrary-key-name.pem")
	if e = os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(private, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); e != nil {
		t.Fatal(e)
	}
	m, e := Load([]string{cert, cert}, private)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Chain) != 2 || !m.Key.Equal(key) {
		t.Fatal("explicit material not loaded")
	}
	other, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	m.Key = other
	if e = m.Validate(); e == nil {
		t.Fatal("mismatched key accepted")
	}
	if _, e = Load([]string{cert}, private); e == nil {
		t.Fatal("missing authority accepted")
	}
	if _, e = Load([]string{cert, cert}, cert); e == nil {
		t.Fatal("certificate accepted as key")
	}
	if e = (*Credentials)(nil).Validate(); e == nil {
		t.Fatal("nil material accepted")
	}
}

func TestBundlesPKCS8AndStrictPEM(t *testing.T) {
	dir := t.TempDir()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "PEM test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	bundle := filepath.Join(dir, "bundle.pem")
	private := filepath.Join(dir, "key.pem")
	os.WriteFile(bundle, append(cert, cert...), 0600)
	encoded, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(private, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600)
	mat, e := Load([]string{bundle}, private)
	if e != nil || len(mat.Chain) != 2 || !mat.Key.Equal(key) {
		t.Fatal("bundle/PKCS8", e)
	}
	for _, data := range [][]byte{append(cert, []byte("garbage")...), []byte("garbage"), nil} {
		os.WriteFile(bundle, data, 0600)
		if _, e := ReadCertificates(bundle); e == nil {
			t.Fatal("invalid PEM accepted")
		}
	}
	os.WriteFile(private, pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: encoded}), 0600)
	if _, e := ReadKey(private); e == nil {
		t.Fatal("encrypted key accepted")
	}
	ec, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	ecDER, e := x509.MarshalPKCS8PrivateKey(ec)
	if e != nil {
		t.Fatal(e)
	}
	validPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	for _, data := range [][]byte{pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecDER}), encoded, append(validPEM, []byte("garbage")...), append([]byte("-----BEGIN BAD-----\ninvalid\n"), validPEM...)} {
		if e = os.WriteFile(private, data, 0600); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadKey(private); e == nil {
			t.Fatal("unsupported or malformed key accepted")
		}
	}
}
