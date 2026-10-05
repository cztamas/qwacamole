package pki

import (
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"time"
)

// These are Mozilla's already-public TEST fixtures, not generated credentials.
// Stock Firefox recognizes this root only when its test-anchor pref is enabled.
// Provenance and license are in firefox-fixtures/README.md.
//
//go:embed firefox-fixtures/test-ca.pem
var firefoxRootPEM []byte

//go:embed firefox-fixtures/public-test-ca-key.pem
var firefoxRootKeyPEM []byte

func GenerateFirefox(dir string, id Identity) error {
	block, _ := pem.Decode(firefoxRootPEM)
	if block == nil {
		return fmt.Errorf("Mozilla test root is not PEM")
	}
	root, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	block, _ = pem.Decode(firefoxRootKeyPEM)
	if block == nil {
		return fmt.Errorf("Mozilla public test key is not PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	if err := key.Validate(); err != nil {
		return err
	}
	public, ok := root.PublicKey.(*rsa.PublicKey)
	if !ok || !public.Equal(&key.PublicKey) {
		return fmt.Errorf("Mozilla test root/key mismatch")
	}
	if time.Now().Before(root.NotBefore) || !time.Now().Add(24*time.Hour).Before(root.NotAfter) {
		return fmt.Errorf("bundled Firefox test authority is outside its usable validity window; update the fixture to match a compatible stock Firefox test authority")
	}
	return generate(dir, id, root, key)
}
