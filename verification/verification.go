// Package verification reports server readiness without changing trust or state.
package verification

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Action string `json:"next_action,omitempty"`
}
type Report struct {
	Hostname string  `json:"hostname"`
	Ready    bool    `json:"ready"`
	Checks   []Check `json:"checks"`
}
type Options struct {
	Hostname, Authority, Address, HTTPSAuthority string
	TLSRoots                                     *x509.CertPool
	FirefoxCompatibility                         bool
}

func (r *Report) Fail(name string, err error, action string) error {
	r.Ready = false
	r.Checks = append(r.Checks, Check{Name: name, Detail: err.Error(), Action: action})
	return fmt.Errorf("%s: %w; %s", name, err, action)
}
func (r *Report) failure(name string, err error, action string) (Report, error) {
	e := r.Fail(name, err, action)
	return *r, e
}
func (r *Report) Pass(name, detail string) {
	r.Checks = append(r.Checks, Check{Name: name, OK: true, Detail: detail})
}
func (r Report) Write(w io.Writer) {
	for _, c := range r.Checks {
		if c.OK {
			fmt.Fprintf(w, "PASS %s: %s\n", c.Name, c.Detail)
		} else {
			fmt.Fprintf(w, "FAIL %s: %s\nNext: %s\n", c.Name, c.Detail, c.Action)
		}
	}
	if r.Ready {
		fmt.Fprintf(w, "Ready: %s serves verified HTTPS and a current signed TLS binding.\n", r.Hostname)
	}
	fmt.Fprintln(w, "Browser acceptance and legal qualification are separate checks.")
}

// Verify uses a fresh normal TLS connection, stable discovery, the explicitly
// selected QWAC authority and the existing binding verifier. It never retrieves
// trust anchors or follows redirects, and bounds both requests and body reads.
func Verify(ctx context.Context, o Options) (Report, error) {
	r := Report{Hostname: o.Hostname}
	if o.Hostname == "" || strings.ContainsAny(o.Hostname, "/:*?@ \r\n") || net.ParseIP(o.Hostname) != nil {
		return r.failure("configuration", fmt.Errorf("one literal DNS hostname required"), "supply the configured hostname")
	}
	roots, e := credentials.ReadCertificates(o.Authority)
	if e != nil || len(roots) != 1 || !roots[0].IsCA {
		if e == nil {
			e = fmt.Errorf("one explicitly selected authority CA required")
		}
		return r.failure("authority", e, "select the issuer authority PEM explicitly")
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{ServerName: o.Hostname, RootCAs: o.TLSRoots, MinVersion: tls.VersionTLS12}}
	if o.Address != "" {
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, o.Address)
		}
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	authority := o.HTTPSAuthority
	if authority == "" {
		authority = o.Hostname
	}
	fetch := func(method, path string) (*http.Response, error) {
		req, e := http.NewRequestWithContext(ctx, method, "https://"+authority+path, nil)
		if e != nil {
			return nil, e
		}
		return client.Do(req)
	}
	resp, e := fetch("HEAD", "/")
	if e != nil {
		return r.failure("HTTPS", e, "check DNS, ports, the ordinary TLS certificate and service logs")
	}
	resp.Body.Close()
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 || resp.StatusCode >= 500 {
		return r.failure("HTTPS", fmt.Errorf("protected endpoint returned HTTP %d", resp.StatusCode), "repair deployment readiness before retrying")
	}
	r.Pass("HTTPS", "ordinary certificate verification succeeded")
	protectedLeaf := resp.TLS.PeerCertificates[0]
	link := "</bindings/current.jose>; rel=\"tls-certificate-binding\""
	if !strings.Contains(strings.Join(resp.Header.Values("Link"), ","), link) {
		return r.failure("discovery", fmt.Errorf("protected response has no stable binding Link"), "check the paired QWAC middleware and proxy configuration")
	}
	resp, e = fetch("GET", store.DiscoveryURL)
	if e != nil {
		return r.failure("discovery", e, "check binding delivery and service logs")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 || resp.Header.Get("Content-Type") != "application/jose" || resp.Header.Get("Cache-Control") != "no-store" {
		return r.failure("discovery", fmt.Errorf("binding response unavailable or incorrectly served (HTTP %d)", resp.StatusCode), "check binding publication and uncached JOSE delivery")
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(body) > 1<<20 {
		if e == nil {
			e = fmt.Errorf("binding exceeds size limit")
		}
		return r.failure("discovery", e, "check binding size and delivery")
	}
	r.Pass("discovery", "signed binding is available at /bindings/current.jose")
	verify := binding.Verify
	if o.FirefoxCompatibility {
		verify = binding.VerifyFirefox
	}
	leaves := []*x509.Certificate{protectedLeaf}
	if !protectedLeaf.Equal(resp.TLS.PeerCertificates[0]) {
		leaves = append(leaves, resp.TLS.PeerCertificates[0])
	}
	if e = verify(string(body), o.Hostname, leaves, roots[0], time.Now()); e != nil {
		return r.failure("signed binding", e, "check hostname, selected authority, signer validity, binding profile and current TLS coverage; inspect service logs")
	}
	r.Pass("authority", "selected authority validates the supplied signer and signature")
	r.Pass("TLS binding", "binding is current and covers the served TLS certificate")
	r.Ready = true
	return r, nil
}
