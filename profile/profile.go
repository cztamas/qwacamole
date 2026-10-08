// Package profile defines and checks the supported 2-QWAC certificate profile.
package profile

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"strings"
)

var PolicyOID, _ = x509.ParseOID("0.4.0.194112.1.6")

var Policy = asn1.ObjectIdentifier{0, 4, 0, 194112, 1, 6}
var BindingEKU = asn1.ObjectIdentifier{0, 4, 0, 194115, 1, 0}
var QCStatements = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}
var Compliance = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 1}
var QCType = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6}
var WebType = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 3}

type Statement struct {
	ID   asn1.ObjectIdentifier
	Info asn1.RawValue `asn1:"optional"`
}

func Validate(c *x509.Certificate) error {
	if c == nil || c.IsCA {
		return errors.New("2-QWAC must be an end-entity certificate")
	}
	if len(c.Subject.Country) != 1 || len(c.Subject.Country[0]) != 2 || len(c.Subject.Organization) != 1 || strings.TrimSpace(c.Subject.Organization[0]) == "" || c.Subject.CommonName == "" || len(c.DNSNames) == 0 {
		return errors.New("missing QWAC subject or SAN")
	}
	orgID := 0
	for _, name := range c.Subject.Names {
		if name.Type.String() == "2.5.4.97" {
			value, ok := name.Value.(string)
			if !ok || strings.TrimSpace(value) == "" || value == c.Subject.Organization[0] {
				return errors.New("invalid legal person organizationIdentifier")
			}
			orgID++
		}
	}
	if orgID != 1 {
		return errors.New("missing legal person organizationIdentifier")
	}
	if len(c.Issuer.Country) != 1 || len(c.Issuer.Organization) != 1 || c.Issuer.CommonName == "" {
		return errors.New("missing QWAC issuer identity")
	}
	if len(c.IssuingCertificateURL) == 0 || len(c.CRLDistributionPoints) == 0 {
		return errors.New("missing AIA or CRL distribution point")
	}
	if c.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("missing digitalSignature key usage")
	}
	if c.KeyUsage & ^(x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment) != 0 {
		return errors.New("incompatible 2-QWAC key usage: signing only required")
	}

	if len(c.ExtKeyUsage) != 0 || len(c.UnknownExtKeyUsage) != 1 || !c.UnknownExtKeyUsage[0].Equal(BindingEKU) {
		return errors.New("QWAC must contain only tls-binding EKU")
	}
	policy := false
	for _, o := range c.PolicyIdentifiers {
		policy = policy || o.Equal(Policy)
	}
	if !policy {
		return errors.New("missing QNCP-w-gen policy")
	}
	compliance, web := false, false
	seen := map[string]bool{}
	for _, ext := range c.Extensions {
		if ext.Id.Equal(QCStatements) {
			if ext.Critical {
				return errors.New("QCStatements must not be critical")
			}
			var ss []Statement
			rest, e := asn1.Unmarshal(ext.Value, &ss)
			if e != nil || len(rest) > 0 {
				return errors.New("invalid QCStatements")
			}
			canonical, e := asn1.Marshal(ss)
			if e != nil || !bytes.Equal(canonical, ext.Value) {
				return errors.New("invalid QCStatements encoding or trailing fields")
			}
			for _, s := range ss {
				if seen[s.ID.String()] {
					return errors.New("duplicate QCStatement")
				}
				seen[s.ID.String()] = true
				if s.ID.Equal(Compliance) {
					if len(s.Info.FullBytes) != 0 {
						return errors.New("invalid QcCompliance statementInfo")
					}
					compliance = true
				}
				if s.ID.Equal(asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 7}) {
					return errors.New("QcCClegislation is incompatible with EU qualified certificates")
				}
				if s.ID.Equal(QCType) {
					var types []asn1.ObjectIdentifier
					rest, e = asn1.Unmarshal(s.Info.FullBytes, &types)
					if e != nil || len(rest) != 0 {
						return errors.New("invalid QcType statementInfo")
					}
					if len(types) != 1 || !types[0].Equal(WebType) {
						return errors.New("QcType must identify only website authentication")
					}
					web = true
				}
			}
		}
	}
	if !compliance || !web {
		return errors.New("missing QWAC QCStatements")
	}
	return nil
}
