package agent

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cztamas/qwacamole/profile"
)

func setupFixture(t *testing.T) SetupOptions {
	t.Helper()
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
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "authority", Country: []string{"HU"}, Organization: []string{"TEST ONLY"}}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	ca := issue(caTemplate, caTemplate)
	web, _ := asn1.Marshal([]asn1.ObjectIdentifier{profile.WebType})
	qc, _ := asn1.Marshal([]profile.Statement{{ID: profile.Compliance}, {ID: profile.QCType, Info: asn1.RawValue{FullBytes: web}}})
	leaf := issue(&x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "example.test", Country: []string{"HU"}, Organization: []string{"TEST ONLY"}, ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 97}, Value: "NTRHU-TEST"}}}, DNSNames: []string{"example.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, UnknownExtKeyUsage: []asn1.ObjectIdentifier{profile.BindingEKU}, Policies: []x509.OID{profile.PolicyOID}, IssuingCertificateURL: []string{"https://example.test/ca"}, CRLDistributionPoints: []string{"https://example.test/crl"}, ExtraExtensions: []pkix.Extension{{Id: profile.QCStatements, Value: qc}}}, ca)
	dir := t.TempDir()
	chain := filepath.Join(dir, "chain.pem")
	private := filepath.Join(dir, "key.pem")
	authority := filepath.Join(dir, "authority.pem")
	for path, data := range map[string][]byte{chain: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}), authority: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}), private: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})} {
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	return SetupOptions{Hostname: "example.test", Application: "http://127.0.0.1:8080", Chain: chain, Key: private, Authority: authority, Email: "operator@example.test", DryRun: true}
}
func TestSetupPlanKeepsStrictDedicatedDeployment(t *testing.T) {
	o := setupFixture(t)
	p, e := buildSetupPlan(o, "/opt/qwacamole/bin/qwacamole-agent")
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Config.Validate(); e != nil {
		t.Fatal(e)
	}
	if p.Config.FirefoxCompatibility {
		t.Fatal("compatibility enabled implicitly")
	}
	var unit, hook string
	for _, file := range p.Files {
		if strings.HasSuffix(file.Path, "agent.service") {
			unit = string(file.Data)
		}
		if strings.HasSuffix(file.Path, "deploy-hook") {
			hook = string(file.Data)
		}
		if strings.Contains(file.Path, "nginx/nginx.conf") {
			t.Fatal("shared Nginx config targeted")
		}
	}
	if !strings.Contains(unit, "RuntimeDirectoryMode=0700") || !strings.Contains(unit, "Requires=qwacamole-nginx.service") || !strings.Contains(hook, "certbot-hook -config /etc/qwacamole/agent.json") {
		t.Fatal(unit, hook)
	}
	if !strings.Contains(strings.Join(p.Commands, "\n"), " verify -config ") {
		t.Fatal("verification missing")
	}
}
func TestSetupRejectsOperatorInputBeforeGeneratingFiles(t *testing.T) {
	o := setupFixture(t)
	for _, tc := range []struct {
		name   string
		change func(*SetupOptions)
	}{
		{"wrong hostname", func(o *SetupOptions) { o.Hostname = "wrong.test" }},
		{"upstream credentials", func(o *SetupOptions) { o.Application = "http://user:password@example.test" }},
		{"ACME contact injection", func(o *SetupOptions) { o.Email = "bad\nexec" }},
		{"ACME server injection", func(o *SetupOptions) { o.ACMEServer = "https://example.test/'injection" }},
		{"wrong authority", func(o *SetupOptions) { o.Authority = o.Chain }},
		{"bad key", func(o *SetupOptions) { o.Key = o.Chain }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := o
			tc.change(&copy)
			if p, e := buildSetupPlan(copy, "/usr/local/bin/qwacamole-agent"); e == nil || len(p.Files) != 0 {
				t.Fatal(p, e)
			}
		})
	}
	os.Chmod(o.Key, 0644)
	if _, e := buildSetupPlan(o, "/usr/local/bin/qwacamole-agent"); e == nil {
		t.Fatal("public signing key accepted")
	}
	os.Chmod(o.Key, 0600)
	link := filepath.Join(filepath.Dir(o.Key), "link.pem")
	os.Symlink(o.Key, link)
	o.Key = link
	if _, e := buildSetupPlan(o, "/usr/local/bin/qwacamole-agent"); e == nil {
		t.Fatal("symlink input accepted")
	}
}

func TestSetupSelectsWorkerIdentityAndDoesNotChangeLegacyRendering(t *testing.T) {
	o := setupFixture(t)
	p, e := buildSetupPlan(o, "/usr/local/bin/qwacamole-agent")
	if e != nil {
		t.Fatal(e)
	}
	if p.Config.NginxWorkerUser != "www-data" || !strings.Contains(RenderNginx(p.Config), "user www-data;\n") {
		t.Fatal("setup worker identity does not match directory ownership")
	}
	if strings.Contains(RenderNginx(validConfig()), "user www-data;") {
		t.Fatal("legacy manual configuration changed")
	}
	c := validConfig()
	c.NginxWorkerUser = "root"
	if e = c.Validate(); e == nil {
		t.Fatal("root worker accepted")
	}
	c.NginxWorkerUser = "user; include malicious"
	if e = c.Validate(); e == nil {
		t.Fatal("worker user injection accepted")
	}
}
