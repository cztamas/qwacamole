// Package binding implements ETSI TS 119 411-5 V2.1.1 Annex B (RS256/S256).
package binding

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cztamas/qwacamole/internal/pki"
	"strings"
	"time"
)

type SigD struct {
	MID   string   `json:"mId"`
	Pars  []string `json:"pars"`
	HashM string   `json:"hashM"`
	HashV []string `json:"hashV"`
}
type Header struct {
	Alg        string   `json:"alg"`
	Kid        string   `json:"kid"`
	Cty        string   `json:"cty"`
	Thumbprint string   `json:"x5t#S256"`
	Chain      []string `json:"x5c"`
	Iat        int64    `json:"iat"`
	Exp        int64    `json:"exp"`
	SigD       SigD     `json:"sigD"`
	Crit       []string `json:"crit"`
}

func ID(c *x509.Certificate) string { s := sha256.Sum256(c.Raw); return hex.EncodeToString(s[:]) }
func TLSHash(c *x509.Certificate) string {
	s := sha256.Sum256([]byte(base64.RawURLEncoding.EncodeToString(c.Raw)))
	return base64.RawURLEncoding.EncodeToString(s[:])
}
func Sign(m *pki.Material, certs []*x509.Certificate, now time.Time, lifetime time.Duration) (string, Header, error) {
	if len(certs) == 0 {
		return "", Header{}, errors.New("empty TLS certificate set")
	}
	h := Header{Alg: "RS256", Kid: ID(m.Chain[0]), Cty: "TLS-Certificate-Binding-v1", Iat: now.Unix(), Exp: now.Add(lifetime).Unix(), Crit: []string{"sigD"}, SigD: SigD{MID: "http://uri.etsi.org/19182/ObjectIdByURIHash", HashM: "S256"}}
	s := sha256.Sum256(m.Chain[0].Raw)
	h.Thumbprint = base64.RawURLEncoding.EncodeToString(s[:])
	for _, c := range m.Chain {
		h.Chain = append(h.Chain, base64.StdEncoding.EncodeToString(c.Raw))
	}
	latest := certs[0].NotAfter
	for _, c := range certs {
		h.SigD.Pars = append(h.SigD.Pars, ID(c)+".cer")
		h.SigD.HashV = append(h.SigD.HashV, TLSHash(c))
		if c.NotAfter.After(latest) {
			latest = c.NotAfter
		}
	}
	for _, end := range []time.Time{latest, m.Chain[0].NotAfter} {
		if h.Exp > end.Unix() {
			h.Exp = end.Unix()
		}
	}
	if h.Exp <= now.Unix() {
		return "", h, errors.New("effective binding expiry has passed")
	}
	return SignHeader(m, h)
}

// SignHeader is also used to construct controlled negative test documents.
func SignHeader(m *pki.Material, h Header) (string, Header, error) {
	d, e := json.Marshal(h)
	if e != nil {
		return "", h, e
	}
	input := base64.RawURLEncoding.EncodeToString(d) + "."
	sum := sha256.Sum256([]byte(input))
	sig, e := rsa.SignPKCS1v15(rand.Reader, m.Key, crypto.SHA256, sum[:])
	return input + "." + base64.RawURLEncoding.EncodeToString(sig), h, e
}
func Decode(doc string) (Header, []*x509.Certificate, []byte, string, error) {
	var h Header
	p := strings.Split(doc, ".")
	if len(p) != 3 || p[1] != "" {
		return h, nil, nil, "", errors.New("expected compact detached JAdES")
	}
	d, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return h, nil, nil, "", e
	}
	dec := json.NewDecoder(strings.NewReader(string(d)))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&h); e != nil {
		return h, nil, nil, "", e
	}
	var cs []*x509.Certificate
	for _, s := range h.Chain {
		d, e := base64.StdEncoding.DecodeString(s)
		if e != nil {
			return h, nil, nil, "", e
		}
		c, e := x509.ParseCertificate(d)
		if e != nil {
			return h, nil, nil, "", e
		}
		cs = append(cs, c)
	}
	sig, e := base64.RawURLEncoding.DecodeString(p[2])
	return h, cs, sig, p[0] + ".", e
}
func Verify(doc, host string, certs []*x509.Certificate, root *x509.Certificate, now time.Time) error {
	h, cs, sig, input, e := Decode(doc)
	if e != nil {
		return e
	}
	if len(cs) < 2 {
		return errors.New("missing QWAC chain")
	}
	if h.Alg != "RS256" || h.Cty != "TLS-Certificate-Binding-v1" || len(h.Crit) != 1 || h.Crit[0] != "sigD" {
		return errors.New("invalid JAdES header")
	}
	if h.Iat > now.Unix() || h.Exp <= now.Unix() {
		return errors.New("binding outside validity")
	}
	sum := sha256.Sum256(cs[0].Raw)
	if h.Thumbprint != base64.RawURLEncoding.EncodeToString(sum[:]) || h.Kid != ID(cs[0]) {
		return errors.New("signer identifier mismatch")
	}
	pub, ok := cs[0].PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("RSA signer required")
	}
	sum = sha256.Sum256([]byte(input))
	if e = rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); e != nil {
		return fmt.Errorf("signature: %w", e)
	}
	if e = Profile(cs[0]); e != nil {
		return e
	}
	if e = cs[0].VerifyHostname(host); e != nil {
		return e
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	inter := x509.NewCertPool()
	for _, c := range cs[1:] {
		inter.AddCert(c)
	}
	if _, e = cs[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); e != nil {
		return e
	}
	if !cs[len(cs)-1].Equal(root) {
		return errors.New("chain does not end in configured QWAC authority")
	}
	if h.SigD.MID != "http://uri.etsi.org/19182/ObjectIdByURIHash" || h.SigD.HashM != "S256" || len(h.SigD.Pars) != len(h.SigD.HashV) || len(certs) == 0 {
		return errors.New("invalid sigD")
	}
	latest := certs[0].NotAfter
	for _, c := range certs {
		if c.NotAfter.After(latest) {
			latest = c.NotAfter
		}
		found := false
		for _, v := range h.SigD.HashV {
			if v == TLSHash(c) {
				found = true
			}
		}
		if !found {
			return errors.New("TLS certificate not bound")
		}
	}
	if h.Exp > latest.Unix() || h.Exp > cs[0].NotAfter.Unix() {
		return errors.New("expiry exceeds effective maximum")
	}
	return nil
}
func Profile(c *x509.Certificate) error {
	if len(c.Subject.Country) != 1 || len(c.Subject.Organization) != 1 || c.Subject.CommonName == "" || len(c.DNSNames) == 0 {
		return errors.New("missing QWAC subject or SAN")
	}
	orgID := false
	for _, name := range c.Subject.Names {
		if name.Type.String() == "2.5.4.97" {
			orgID = true
		}
	}
	if !orgID {
		return errors.New("missing legal person organizationIdentifier")
	}
	if len(c.Issuer.Country) != 1 || len(c.Issuer.Organization) != 1 || c.Issuer.CommonName == "" {
		return errors.New("missing QWAC issuer identity")
	}
	if len(c.IssuingCertificateURL) == 0 || len(c.CRLDistributionPoints) == 0 {
		return errors.New("missing test AIA or CRL distribution point")
	}
	if c.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("missing digitalSignature key usage")
	}

	if len(c.ExtKeyUsage) != 0 || len(c.UnknownExtKeyUsage) != 1 || !c.UnknownExtKeyUsage[0].Equal(pki.BindingEKU) {
		return errors.New("QWAC must contain only tls-binding EKU")
	}
	policy := false
	for _, o := range c.PolicyIdentifiers {
		policy = policy || o.Equal(pki.Policy)
	}
	if !policy {
		return errors.New("missing QNCP-w-gen policy")
	}
	compliance, web := false, false
	for _, ext := range c.Extensions {
		if ext.Id.Equal(pki.QCStatements) {
			var ss []pki.Statement
			rest, e := asn1.Unmarshal(ext.Value, &ss)
			if e != nil || len(rest) > 0 {
				return errors.New("invalid QCStatements")
			}
			for _, s := range ss {
				if s.ID.Equal(pki.Compliance) {
					compliance = true
				}
				if s.ID.Equal(pki.QCType) {
					var types []asn1.ObjectIdentifier
					if _, e = asn1.Unmarshal(s.Info.FullBytes, &types); e != nil {
						return e
					}
					for _, t := range types {
						web = web || t.Equal(pki.WebType)
					}
				}
			}
		}
	}
	if !compliance || !web {
		return errors.New("missing QWAC QCStatements")
	}
	return nil
}
