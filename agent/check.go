package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

// Check verifies a deployed binding against the actual leaf of a fresh normally
// verified connection. This diagnostic is not native-browser acceptance.
func Check(ctx context.Context, c Config) error {
	roots, e := c.roots()
	if e != nil {
		return e
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{ServerName: c.Host, RootCAs: roots, MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, c.ProbeAddress)
	}}
	defer tr.CloseIdleConnections()
	req, e := http.NewRequestWithContext(ctx, "GET", "https://"+c.Host+store.DiscoveryURL, nil)
	if e != nil {
		return e
	}
	resp, e := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.TLS == nil {
		return fmt.Errorf("binding response unavailable: HTTP %d", resp.StatusCode)
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, 1<<21))
	if e != nil {
		return e
	}
	root, e := credentials.ReadCert(c.TrustFile)
	if e != nil {
		return e
	}
	verify := binding.Verify
	if c.FirefoxCompatibility {
		verify = binding.VerifyFirefox
	}
	return verify(string(body), c.Host, []*x509.Certificate{resp.TLS.PeerCertificates[0]}, root, time.Now())
}
