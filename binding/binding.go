// Package binding implements ETSI TS 119 411-5 V2.1.1 Annex B (RS256/S256).
package binding

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/profile"
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
	Crit       []string `json:"crit,omitempty"`
}

func ID(c *x509.Certificate) string { s := sha256.Sum256(c.Raw); return hex.EncodeToString(s[:]) }
func TLSHash(c *x509.Certificate) string {
	s := sha256.Sum256([]byte(base64.RawURLEncoding.EncodeToString(c.Raw)))
	return base64.RawURLEncoding.EncodeToString(s[:])
}
func Sign(m *credentials.Credentials, certs []*x509.Certificate, now time.Time, lifetime time.Duration) (string, Header, error) {
	return sign(m, certs, now, lifetime, false)
}

// SignFirefox is an explicit compatibility variant: Firefox 157 rejects crit.
// The standard Chrome/JAdES path continues to mark sigD critical.
func SignFirefox(m *credentials.Credentials, certs []*x509.Certificate, now time.Time, lifetime time.Duration) (string, Header, error) {
	return sign(m, certs, now, lifetime, true)
}

func sign(m *credentials.Credentials, certs []*x509.Certificate, now time.Time, lifetime time.Duration, firefox bool) (string, Header, error) {
	if e := m.Validate(); e != nil {
		return "", Header{}, e
	}
	if len(certs) == 0 {
		return "", Header{}, errors.New("empty TLS certificate set")
	}
	h := Header{Alg: "RS256", Kid: ID(m.Chain[0]), Cty: "TLS-Certificate-Binding-v1", Iat: now.Unix(), Exp: now.Add(lifetime).Unix(), Crit: []string{"sigD"}, SigD: SigD{MID: "http://uri.etsi.org/19182/ObjectIdByURIHash", HashM: "S256"}}
	if firefox {
		h.Crit = nil
	}
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
func SignHeader(m *credentials.Credentials, h Header) (string, Header, error) {
	if e := m.Validate(); e != nil {
		return "", h, e
	}
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
	return verify(doc, host, certs, root, now, false, false)
}

func VerifyFirefox(doc, host string, certs []*x509.Certificate, root *x509.Certificate, now time.Time) error {
	return verify(doc, host, certs, root, now, true, false)
}

// InspectDocument authenticates a stored document's signer, profile, hostname,
// signature and hash-list structure at its issuance time. It does NOT establish
// TLS certificate coverage or current validity; use Verify for acceptance. This
// permits storage collection to recognize orphaned, unpublished candidates.
func InspectDocument(doc, host string, root *x509.Certificate, firefox bool) (Header, error) {
	h, _, _, _, e := Decode(doc)
	if e == nil {
		e = verify(doc, host, nil, root, time.Unix(h.Iat, 0), firefox, true)
	}
	return h, e
}

func verify(doc, host string, certs []*x509.Certificate, root *x509.Certificate, now time.Time, firefox, inspect bool) error {
	h, cs, sig, input, e := Decode(doc)
	if e != nil {
		return e
	}
	if len(cs) < 2 {
		return errors.New("missing QWAC chain")
	}
	critOK := len(h.Crit) == 1 && h.Crit[0] == "sigD"
	if firefox {
		critOK = len(h.Crit) == 0
	}
	if h.Alg != "RS256" || h.Cty != "TLS-Certificate-Binding-v1" || !critOK {
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
	if h.SigD.MID != "http://uri.etsi.org/19182/ObjectIdByURIHash" || h.SigD.HashM != "S256" || len(h.SigD.Pars) != len(h.SigD.HashV) || len(h.SigD.Pars) == 0 {
		return errors.New("invalid sigD")
	}
	seen := map[string]bool{}
	for n, p := range h.SigD.Pars {
		if len(p) != 68 || !strings.HasSuffix(p, ".cer") || p != strings.ToLower(p) || seen[p] {
			return errors.New("invalid or duplicate sigD identifier")
		}
		seen[p] = true
		if _, e = hex.DecodeString(p[:64]); e != nil {
			return errors.New("invalid sigD identifier")
		}
		hash, err := base64.RawURLEncoding.DecodeString(h.SigD.HashV[n])
		if err != nil || len(hash) != sha256.Size {
			return errors.New("invalid sigD hash")
		}
	}
	if h.Exp > cs[0].NotAfter.Unix() {
		return errors.New("expiry exceeds QWAC maximum")
	}
	if inspect {
		return nil
	}
	if len(certs) == 0 {
		return errors.New("empty TLS certificate set")
	}
	latest := certs[0].NotAfter
	for _, c := range certs {
		if c.NotAfter.After(latest) {
			latest = c.NotAfter
		}
		found := false
		for n, v := range h.SigD.HashV {
			if v == TLSHash(c) && h.SigD.Pars[n] == ID(c)+".cer" {
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

// Profile checks the supported 2-QWAC certificate profile.
func Profile(c *x509.Certificate) error { return profile.Validate(c) }
