package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/devtools/internal/browser"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

// firefoxIntegration owns fresh services and one unchanged isolated browser
// profile. The real cache is in the demo supervisor, so it survives Caddy reload.
func firefoxIntegration(base, binary, driver, certutil string) error {
	var e error
	if binary == "" {
		binary, e = browser.DetectFirefox()
		if e != nil {
			return e
		}
	}
	if driver == "" {
		driver = ".tools/bin/geckodriver"
	}
	if certutil == "" {
		certutil, e = exec.LookPath("certutil")
		if e != nil {
			return e
		}
	}
	if e = checkDemoPorts(); e != nil {
		return e
	}
	base, e = filepath.Abs(base)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(base, 0700); e != nil {
		return e
	}
	dir, e := os.MkdirTemp(base, "firefox-integration-")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Join(dir, "evidence"), 0700); e != nil {
		return e
	}
	fmt.Println("STATE", dir)
	pkidir := filepath.Join(dir, "pki")
	if e = pki.GenerateFirefox(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Alpha Firefox TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-ALPHA-FIREFOX"}); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "firefox-compatibility"), []byte("explicit Firefox test profile\n"), 0600); e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	start := func(sub string) (*exec.Cmd, error) {
		f, e := os.OpenFile(filepath.Join(dir, "evidence", sub+"-console.log"), os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		cmd := exec.Command(exe, sub, "-state", dir)
		cmd.Stdout, cmd.Stderr = f, f
		if e = cmd.Start(); e != nil {
			return nil, e
		}
		return cmd, nil
	}
	demo, e := start("demo")
	if e != nil {
		return e
	}
	defer stopChild(demo)
	readState := func() (store.State, error) {
		var s store.State
		d, e := os.ReadFile(filepath.Join(dir, "bindings/current.json"))
		if e == nil {
			e = json.Unmarshal(d, &s)
		}
		return s, e
	}
	var first store.State
	for end := time.Now().Add(30 * time.Second); ; {
		first, e = readState()
		if e == nil {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("Firefox demo startup failed: %w", e)
		}
		time.Sleep(200 * time.Millisecond)
	}
	tlsRoot, e := pki.ReadCert(filepath.Join(pkidir, "pebble-root.pem"))
	if e != nil {
		return e
	}
	qwac, e := pki.ReadCert(filepath.Join(pkidir, "qwac-leaf.pem"))
	if e != nil {
		return e
	}
	profile := filepath.Join(dir, "profiles/lifecycle")
	if e = browser.PrepareFirefox(profile, certutil, tlsRoot, true, true); e != nil {
		return e
	}
	userPrefs, e := os.ReadFile(filepath.Join(profile, "user.js"))
	if e != nil {
		return e
	}
	trust := func() ([]byte, error) {
		out, e := exec.Command(certutil, "-L", "-d", "sql:"+profile).Output()
		if e != nil {
			return nil, e
		}
		// Firefox may cache untrusted intermediate certificates. Compare trust
		// grants, not harmless intermediate-cache additions to the NSS database.
		var trusted []string
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 1 && strings.ContainsAny(fields[len(fields)-1], "CTP") {
				trusted = append(trusted, strings.TrimSpace(line))
			}
		}
		root, e := exec.Command(certutil, "-L", "-d", "sql:"+profile, "-n", "Qwacamole Pebble TLS TEST", "-a").Output()
		if e != nil {
			return nil, e
		}
		return append([]byte(strings.Join(trusted, "\n")), root...), nil
	}
	initialTrust, e := trust()
	if e != nil {
		return e
	}
	checkTrust := func() error {
		current, e := trust()
		if e != nil {
			return e
		}
		prefs, e := os.ReadFile(filepath.Join(profile, "user.js"))
		if e != nil {
			return e
		}
		if !bytes.Equal(current, initialTrust) || !bytes.Equal(prefs, userPrefs) {
			return fmt.Errorf("Firefox profile trust/preferences changed")
		}
		return nil
	}
	run := func(name, path string, leaf *x509.Certificate) error {
		evidence := filepath.Join(dir, "evidence", "firefox-"+name)
		if e := os.MkdirAll(evidence, 0700); e != nil {
			return e
		}
		r, e := browser.RunFirefox(context.Background(), browser.FirefoxOptions{Binary: binary, Driver: driver, Profile: profile, URL: "https://qwac.example.com:8443" + path, Evidence: evidence, ExpectedQWAC: leaf})
		if writeErr := writeJSON(filepath.Join(evidence, "firefox-result.json"), r); writeErr != nil {
			return writeErr
		}
		if e != nil {
			return e
		}
		if e = browser.AssertFirefox(r, "positive", leaf); e != nil {
			return e
		}
		if e = checkTrust(); e != nil {
			return e
		}
		fmt.Println("PASS native Firefox", name, "owner", r.UI.Organization, "profile/trust unchanged")
		return nil
	}
	if e = run("initial", "/", qwac); e != nil {
		return e
	}
	if e = run("cached-initial", "/cached", qwac); e != nil {
		return e
	}
	pool := x509.NewCertPool()
	pool.AddCert(tlsRoot)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:8443")
	}, ForceAttemptHTTP2: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	fetch := func(path string) (*http.Response, []byte, error) {
		res, e := client.Get("https://qwac.example.com:8443" + path)
		if e != nil {
			return nil, nil, e
		}
		defer res.Body.Close()
		d, e := io.ReadAll(res.Body)
		return res, d, e
	}
	cached, body, e := fetch("/cached")
	if e != nil {
		return e
	}
	links := strings.Join(cached.Header.Values("Link"), ",")
	if cached.StatusCode != 200 || cached.Header.Get("X-Qwac-Cache") != "hit" || !strings.Contains(links, store.DiscoveryURL) {
		return fmt.Errorf("Firefox cache not populated with stable Link")
	}
	if e = lifecycle(dir); e != nil {
		return e
	}
	if time.Now().Unix() <= first.Expires {
		return fmt.Errorf("Firefox cache scenario did not outlive initial binding expiry")
	}
	if e = run("cached-renewed", "/cached", qwac); e != nil {
		return e
	}
	checkCache := func(name string) error {
		transport.CloseIdleConnections()
		res, d, e := fetch("/cached")
		if e != nil {
			return e
		}
		if res.StatusCode != 200 || res.Header.Get("X-Qwac-Cache") != "hit" || !bytes.Equal(body, d) || strings.Join(res.Header.Values("Link"), ",") != links {
			return fmt.Errorf("intermediary rewrote or missed cached response")
		}
		_, doc, e := fetch(store.DiscoveryURL)
		if e != nil {
			return e
		}
		root, e := pki.ReadCert(filepath.Join(pkidir, "qwac-root.pem"))
		if e != nil {
			return e
		}
		if e = binding.VerifyFirefox(string(doc), "qwac.example.com", res.TLS.PeerCertificates[:1], root, time.Now()); e != nil {
			return e
		}
		h, _, _, _, e := binding.Decode(string(doc))
		if e != nil {
			return e
		}
		sum := sha256.Sum256(d)
		return writeJSON(filepath.Join(dir, "evidence", "cache-"+name+".json"), map[string]any{"cache": res.Header.Get("X-Qwac-Cache"), "age": res.Header.Get("Age"), "link": links, "body_sha256": hex.EncodeToString(sum[:]), "connection_tls_id": binding.ID(res.TLS.PeerCertificates[0]), "binding_iat": h.Iat, "binding_exp": h.Exp, "initial_binding_exp": first.Expires})
	}
	if e = checkCache("renewed"); e != nil {
		return e
	}
	if e = refreshTest(dir); e != nil {
		return e
	}
	replaced, e := replaceDemoQWAC(pkidir, "Alpha Firefox Replacement TEST")
	if e != nil {
		return e
	}
	configPath := filepath.Join(dir, "caddy.json")
	if out, e := exec.Command("bin/caddy", "reload", "--config", configPath, "--force").CombinedOutput(); e != nil {
		return fmt.Errorf("Firefox replacement reload: %w: %s", e, out)
	}
	if e = run("cached-replacement", "/cached", replaced); e != nil {
		return e
	}
	if e = checkCache("replacement"); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "restart-request"), nil, 0600); e != nil {
		return e
	}
	for end := time.Now().Add(15 * time.Second); ; {
		if _, e := os.Stat(filepath.Join(dir, "evidence/restart.json")); e == nil {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("Firefox Caddy restart not observed")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if e = run("restarted", "/cached", replaced); e != nil {
		return e
	}
	if e = failureTest(dir); e != nil {
		return e
	}
	if e = run("failure-recovery", "/cached", replaced); e != nil {
		return e
	}
	fixture, e := start("fixture")
	if e != nil {
		return e
	}
	defer stopChild(fixture)
	time.Sleep(time.Second)
	if e = firefoxSuite(dir, binary, driver, certutil); e != nil {
		return e
	}
	if e = checkTrust(); e != nil {
		return e
	}
	fmt.Println("PASS native Firefox lifecycle, cached discovery and all eleven fixtures")
	return writeJSON(filepath.Join(dir, "evidence/firefox-integration.json"), map[string]any{"initial_native_verified": true, "renewal_native_verified": true, "profile": profile, "profile_trust_unchanged": true, "cached_discovery_after_expiry_renewal_replacement": true, "restart_recovered": true, "independent_refresh": true, "renewal_failure_gates": true, "fixture_checks": 11})
}

func replaceDemoQWAC(pkidir, organization string) (*x509.Certificate, error) {
	old, e := pki.ReadCert(filepath.Join(pkidir, "qwac-leaf.pem"))
	if e != nil {
		return nil, e
	}
	ca, e := pki.ReadCert(filepath.Join(pkidir, "qwac-intermediate.pem"))
	if e != nil {
		return nil, e
	}
	caKey, e := pki.ReadKey(filepath.Join(pkidir, "qwac-intermediate-key.pem"))
	if e != nil {
		return nil, e
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return nil, e
	}
	tmpl := *old
	tmpl.RawSubject = nil
	tmpl.SerialNumber = pki.Serial()
	tmpl.Subject.Organization = []string{organization}
	tmpl.Subject.ExtraNames = append(tmpl.Subject.ExtraNames, old.Subject.Names...)
	for n, name := range tmpl.Subject.ExtraNames {
		if name.Type.String() == "2.5.4.10" {
			tmpl.Subject.ExtraNames[n].Value = organization
		}
	}
	tmpl.ExtraExtensions = old.Extensions
	cert, e := pki.Issue(&tmpl, ca, &key.PublicKey, caKey)
	if e != nil {
		return nil, e
	}
	if e = store.Atomic(filepath.Join(pkidir, "qwac-leaf.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})); e != nil {
		return nil, e
	}
	if e = store.Atomic(filepath.Join(pkidir, "qwac-leaf-key.pem"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})); e != nil {
		return nil, e
	}
	return cert, nil
}
