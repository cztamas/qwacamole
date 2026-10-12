package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cztamas/qwacamole/verification"
)

// Verify retains the existing control readiness and shared cryptographic checks.
// Check remains available for callers that require only its historical result.
func Verify(ctx context.Context, c Config) (verification.Report, error) {
	r := verification.Report{Hostname: c.Host}
	data, e := Control(ctx, c, "status")
	if e != nil {
		e = r.Fail("Nginx deployment", e, "start the dedicated Nginx and agent services; inspect their logs")
		return r, e
	}
	var status Status
	if e = json.Unmarshal(data, &status); e != nil {
		e = r.Fail("Nginx deployment", e, "check agent version and private control socket")
		return r, e
	}
	if !status.Ready || status.Deployment.LastError != "" {
		e = r.Fail("Nginx deployment", fmt.Errorf("deployment is not ready: %s", status.Deployment.LastError), "finish Certbot issuance or repair the reported deployment error; run reconcile after recovery")
		return r, e
	}
	roots, e := c.roots()
	if e != nil {
		e = r.Fail("HTTPS", e, "check the configured ordinary TLS roots")
		return r, e
	}
	state, e := c.StoreConfig()
	if e != nil {
		e = r.Fail("configuration", e, "check the agent binding configuration")
		return r, e
	}
	r, e = verification.Verify(ctx, verification.Options{Hostname: c.Host, Authority: c.TrustFile, Address: c.ProbeAddress, HTTPSAuthority: c.PublicAuthority(), TLSRoots: roots, StateDirectory: state.Dir, FirefoxCompatibility: c.FirefoxCompatibility})
	if e == nil {
		r.Pass("Nginx deployment", "dedicated deployment is ready")
	}
	return r, e
}
