package credentials

import (
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
