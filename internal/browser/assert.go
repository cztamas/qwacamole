package browser

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

// Assert never treats missing verification events, a timeout, or unrelated TLS
// failures as a successful QWAC negative test.
func Assert(r Result, expect string, root *x509.Certificate) error {
	loaded := false
	valid := false
	errDescription := ""
	certError := float64(0)
	rootSeen := false
	for _, e := range r.Events {
		if e.Type == "CERT_VERIFY_PROC_CHROME_ROOT_STORE_VERSION" {
			if e.Params["version_major"] == float64(RootVersion) {
				loaded = true
			}
		}
		if e.Type == "CERT_VERIFY_PROC_2QWAC_BINDING" && e.Phase == 2 {
			valid = valid || e.Params["is_valid_2qwac_binding"] == true
			if s, ok := e.Params["error_description"].(string); ok {
				errDescription = s
			}
		}
		if e.Type == "CERT_VERIFY_PROC_2QWAC" && e.Phase == 2 {
			if n, ok := e.Params["net_error"].(float64); ok {
				certError = n
			}
			if v, ok := e.Params["verified_cert"].(map[string]any); ok {
				if chain, ok := v["certificates"].([]any); ok {
					for _, raw := range chain {
						if s, ok := raw.(string); ok {
							p, _ := pem.Decode([]byte(s))
							if p != nil && root != nil {
								c, err := x509.ParseCertificate(p.Bytes)
								if err == nil && c.Equal(root) {
									rootSeen = true
								}
							}
						}
					}
				}
			}
		}
	}
	if !loaded {
		return fmt.Errorf("patched PKI Metadata did not load")
	}
	switch expect {
	case "positive":
		if r.NavigationError != "" || !valid || !rootSeen {
			return fmt.Errorf("native acceptance missing: navigation=%s valid=%v authority=%v", r.NavigationError, valid, rootSeen)
		}
	case "signature":
		if r.NavigationError != "" || valid || errDescription != "binding signature invalid" {
			return fmt.Errorf("expected binding signature rejection; got %q %q valid=%v", r.NavigationError, errDescription, valid)
		}
	case "wrong-tls":
		if r.NavigationError != "" || valid || errDescription != "TLS cert not bound" {
			return fmt.Errorf("expected unbound TLS rejection; got %q %q", r.NavigationError, errDescription)
		}
	case "qwac-hostname", "qwac-profile", "no-qwac-trust":
		want := float64(-200)
		if expect == "qwac-profile" {
			want = -207
		}
		if expect == "no-qwac-trust" {
			want = -202
		}
		if r.NavigationError != "" || valid || errDescription != "2-QWAC cert verify failed" || certError != want {
			return fmt.Errorf("expected QWAC cert failure %v; got TLS=%q binding=%q cert=%v valid=%v", want, r.NavigationError, errDescription, certError, valid)
		}
	case "expired-binding":
		if r.NavigationError != "" {
			return fmt.Errorf("unrelated TLS failure: %s", r.NavigationError)
		}
		if valid {
			return fmt.Errorf("BROWSER INCOMPATIBILITY: native verifier accepted expired exp header")
		}
		if errDescription == "" {
			return fmt.Errorf("missing native binding failure")
		}
	case "no-tls-trust", "tls-hostname", "tls-expiry", "ct-required":
		want := "ERR_CERT_AUTHORITY_INVALID"
		switch expect {
		case "tls-hostname":
			want = "ERR_CERT_COMMON_NAME_INVALID"
		case "tls-expiry":
			want = "ERR_CERT_DATE_INVALID"
		case "ct-required":
			want = "ERR_CERTIFICATE_TRANSPARENCY_REQUIRED"
		}
		if !strings.Contains(r.NavigationError, want) || valid {
			return fmt.Errorf("expected %s; got %q valid=%v", want, r.NavigationError, valid)
		}
	default:
		return fmt.Errorf("unknown expectation: %s", expect)
	}
	return nil
}
