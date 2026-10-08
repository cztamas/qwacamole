package credentials

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/cztamas/qwacamole/profile"
)

// CreateFile creates private, synced output without following an existing
// symlink or overwriting an existing file. Parent directories must be private.
func CreateFile(path string, data []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

// GenerateKey creates a dedicated local RS256 key, encoded as PKCS#8 PEM.
func GenerateKey(path string, bits int) error {
	if bits != 2048 && bits != 3072 && bits != 4096 {
		return fmt.Errorf("RSA key size must be 2048, 3072 or 4096 bits")
	}
	key, e := rsa.GenerateKey(rand.Reader, bits)
	if e != nil {
		return e
	}
	d, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		return e
	}
	return CreateFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: d}))
}

// Request contains the legal-person subject and DNS SANs requested from an
// issuer. These requests cannot establish qualification or guarantee issuance.
type Request struct {
	Country, Organization, Registration, CommonName string
	Locality, Province, Street, PostalCode          string
	Hosts                                           []string
}

// CSR uses a caller-owned signer and verifies the resulting PKCS#10 signature.
// The private key is neither exported nor changed. Only RSA/RS256 is supported.
func CSR(signer crypto.Signer, r Request) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("signing key required")
	}
	pub, ok := signer.Public().(*rsa.PublicKey)
	if !ok || pub.N.BitLen() < 2048 {
		return nil, fmt.Errorf("CSR requires an RSA signing key of at least 2048 bits")
	}
	if len(r.Country) != 2 || r.Country != strings.ToUpper(r.Country) || strings.TrimSpace(r.Organization) == "" || strings.TrimSpace(r.Registration) == "" || r.Registration == r.Organization || strings.TrimSpace(r.CommonName) == "" || len(r.Hosts) == 0 {
		return nil, fmt.Errorf("two-letter country, organization, distinct registration identifier, common name and DNS hostnames required")
	}
	for _, host := range r.Hosts {
		if !validDNS(host) {
			return nil, fmt.Errorf("CSR hostname must be a lowercase literal DNS name: %q", host)
		}
	}
	subject := pkix.Name{Country: []string{r.Country}, Organization: []string{r.Organization}, CommonName: r.CommonName,
		ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 97}, Value: r.Registration}}}
	for _, field := range []struct {
		value string
		dst   *[]string
	}{{r.Locality, &subject.Locality}, {r.Province, &subject.Province}, {r.Street, &subject.StreetAddress}, {r.PostalCode, &subject.PostalCode}} {
		if field.value != "" {
			*field.dst = []string{field.value}
		}
	}
	eku, _ := asn1.Marshal([]asn1.ObjectIdentifier{profile.BindingEKU})
	policies, _ := asn1.Marshal([]struct{ Policy asn1.ObjectIdentifier }{{profile.Policy}})
	ku, _ := asn1.Marshal(asn1.BitString{Bytes: []byte{0x80}, BitLength: 1})
	template := &x509.CertificateRequest{Subject: subject, DNSNames: r.Hosts, SignatureAlgorithm: x509.SHA256WithRSA,
		ExtraExtensions: []pkix.Extension{
			{Id: asn1.ObjectIdentifier{2, 5, 29, 15}, Critical: true, Value: ku},
			{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Value: eku},
			{Id: asn1.ObjectIdentifier{2, 5, 29, 32}, Value: policies},
		}}
	d, e := x509.CreateCertificateRequest(rand.Reader, template, signer)
	if e != nil {
		return nil, e
	}
	request, e := x509.ParseCertificateRequest(d)
	if e != nil {
		return nil, e
	}
	if e = request.CheckSignature(); e != nil {
		return nil, fmt.Errorf("generated CSR signature: %w", e)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: d}), nil
}

func validDNS(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
