// Package profile defines and checks the supported 2-QWAC certificate profile.
package profile

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
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
		return errors.New("missing AIA or CRL distribution point")
	}
	if c.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("missing digitalSignature key usage")
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
	for _, ext := range c.Extensions {
		if ext.Id.Equal(QCStatements) {
			var ss []Statement
			rest, e := asn1.Unmarshal(ext.Value, &ss)
			if e != nil || len(rest) > 0 {
				return errors.New("invalid QCStatements")
			}
			for _, s := range ss {
				if s.ID.Equal(Compliance) {
					compliance = true
				}
				if s.ID.Equal(QCType) {
					var types []asn1.ObjectIdentifier
					if _, e = asn1.Unmarshal(s.Info.FullBytes, &types); e != nil {
						return e
					}
					for _, t := range types {
						web = web || t.Equal(WebType)
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
