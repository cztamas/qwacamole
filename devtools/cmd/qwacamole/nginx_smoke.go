package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/cacheproxy"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
	"github.com/miekg/dns"
)

// nginxSmoke owns every process and directory it creates. Certbot performs real
// HTTP-01 validation through Nginx; the agent is a separate production binary.
func nginxSmoke(parent, agentBin, nginxBin, certbotBin string) (retErr error) {
	if e := checkDemoPorts(); e != nil {
		return e
	}
	parent, e := filepath.Abs(parent)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(parent, 0700); e != nil {
		return e
	}
	dir, e := os.MkdirTemp(parent, "nginx-smoke-")
	if e != nil {
		return e
	}
	fmt.Println("STATE", dir)
	runtimeDir, e := os.MkdirTemp("", "qwac-agent-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(runtimeDir)
	os.MkdirAll(filepath.Join(dir, "evidence"), 0700)
	c := &pluginCheck{dir: dir, env: append(os.Environ(), "PEBBLE_VA_NOSLEEP=1", "PEBBLE_VA_ALWAYS_VALID=0", "PEBBLE_AUTHZREUSE=0", "PEBBLE_WFE_NONCEREJECT=0")}
	var nginx, daemon *exec.Cmd
	defer func() {
		stopChild(daemon)
		stopChild(nginx)
		stopChild(c.pebble)
		writeJSON(filepath.Join(dir, "evidence/result.json"), map[string]any{"checks": c.results, "error": fmt.Sprint(retErr)})
	}()
	for _, p := range []*string{&agentBin, &nginxBin, &certbotBin} {
		*p, e = filepath.Abs(*p)
		if e != nil {
			return e
		}
	}
	pkidir := filepath.Join(dir, "pki")
	if e = pki.Generate(pkidir, pki.Identity{Host: "qwac.example.com", Organization: "Nginx Agent TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-NGINX-TEST"}); e != nil {
		return e
	}
	dnsMux := dns.NewServeMux()
	dnsMux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for _, q := range r.Question {
			if q.Name == "qwac.example.com." && q.Qtype == dns.TypeA {
				m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET}, A: net.ParseIP("127.0.0.1").To4()})
			}
		}
		_ = w.WriteMsg(m)
	})
	ds := &dns.Server{Addr: "127.0.0.1:8053", Net: "udp", Handler: dnsMux}
	go ds.ListenAndServe()
	defer ds.Shutdown()
	pebbleConfig := map[string]any{"pebble": map[string]any{"listenAddress": "127.0.0.1:14000", "managementListenAddress": "127.0.0.1:15000", "certificate": filepath.Join(pkidir, "acme-leaf.pem"), "privateKey": filepath.Join(pkidir, "acme-leaf-key.pem"), "httpPort": 5002, "tlsPort": 5001, "profiles": map[string]any{"default": map[string]any{"description": "agent renewal test", "validityPeriod": 120}}, "retryAfter": map[string]int{"authz": 1, "order": 1}}}
	if e = writeJSON(filepath.Join(dir, "pebble.json"), pebbleConfig); e != nil {
		return e
	}
	c.pebble, e = c.start(".tools/bin/pebble", "pebble", "-config", filepath.Join(dir, "pebble.json"), "-dnsserver", "127.0.0.1:8053")
	if e != nil {
		return e
	}
	ca, e := os.ReadFile(filepath.Join(pkidir, "acme-root.pem"))
	if e != nil {
		return e
	}
	acmeRoots := x509.NewCertPool()
	acmeRoots.AppendCertsFromPEM(ca)
	management := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: acmeRoots}}, Timeout: 3 * time.Second}
	var tlsRoot []byte
	for end := time.Now().Add(15 * time.Second); ; {
		resp, err := management.Get("https://localhost:15000/roots/0")
		if err == nil {
			tlsRoot, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && resp.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(end) {
			return fmt.Errorf("Pebble management unavailable: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	rootPath := filepath.Join(pkidir, "pebble-root.pem")
	if e = os.WriteFile(rootPath, tlsRoot, 0600); e != nil {
		return e
	}
	appLn, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer appLn.Close()
	releaseHeld := make(chan struct{})
	var cache atomic.Pointer[cacheproxy.Proxy]
	app := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cached" {
			if p := cache.Load(); p != nil {
				p.ServeHTTP(w, r)
			} else {
				http.Error(w, "cache not configured", 503)
			}
			return
		}
		if r.URL.Path == "/held" {
			w.WriteHeader(200)
			w.Write([]byte("held-stream\n"))
			w.(http.Flusher).Flush()
			tick := time.NewTicker(500 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-r.Context().Done():
					return
				case <-releaseHeld:
					return
				case <-tick.C:
					w.Write([]byte("held-stream\n"))
					w.(http.Flusher).Flush()
				}
			}
		}
		w.Header().Set("Link", `</style>; rel="preload"`)
		w.Header().Set("Cache-Control", "public, max-age=600")
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/app")
			w.WriteHeader(307)
			return
		}
		if r.URL.Path == "/error" {
			w.WriteHeader(418)
			w.Write([]byte("application error"))
			return
		}
		if r.URL.Path == "/conditional" {
			w.Header().Set("ETag", `"app"`)
			if r.Header.Get("If-None-Match") == `"app"` {
				w.WriteHeader(304)
				return
			}
		}
		if r.URL.Path == "/cache-origin" {
			w.Write([]byte("cached Nginx application body"))
			return
		}
		w.WriteHeader(201)
		w.Write([]byte("application-first\n"))
		w.(http.Flusher).Flush()
		w.Write([]byte("application-second\n"))
	})}
	go app.Serve(appLn)
	defer app.Close()
	cfg := agent.Config{Host: "qwac.example.com", RuntimeDir: runtimeDir, StateDir: filepath.Join(dir, "agent"), ChainFiles: []string{filepath.Join(pkidir, "qwac-leaf.pem"), filepath.Join(pkidir, "qwac-intermediate.pem"), filepath.Join(pkidir, "qwac-root.pem")}, KeyFile: filepath.Join(pkidir, "qwac-leaf-key.pem"), TrustFile: filepath.Join(pkidir, "qwac-root.pem"), TLSRoots: rootPath, CertbotLineage: filepath.Join(dir, "certbot", "live", "qwac.example.com"), PublicListen: "127.0.0.1:18081", HTTPListen: "127.0.0.1:5002", HTTPSListen: "127.0.0.1:8443", ProbeAddress: "127.0.0.1:8443", Application: "http://" + appLn.Addr().String(), Webroot: filepath.Join(dir, "webroot"), NginxBinary: nginxBin, NginxPrefix: filepath.Join(dir, "nginx"), NginxConfig: filepath.Join(dir, "nginx.conf"), Lifetime: "45s", DocumentGrace: "2s"}
	for _, path := range []string{cfg.StateDir, filepath.Join(cfg.StateDir, "enabled"), cfg.NginxPrefix, cfg.Webroot} {
		if e = os.MkdirAll(path, 0700); e != nil {
			return e
		}
	}
	if e = os.WriteFile(cfg.NginxConfig, []byte(agent.RenderNginx(cfg)), 0600); e != nil {
		return e
	}
	cfgPath := filepath.Join(dir, "agent.json")
	if e = writeJSON(cfgPath, cfg); e != nil {
		return e
	}
	if e = (agent.Nginx{Config: cfg}).Validate(context.Background()); e != nil {
		return e
	}
	nginx, e = c.start(nginxBin, "nginx", "-p", cfg.NginxPrefix, "-c", cfg.NginxConfig, "-g", "daemon off;")
	if e != nil {
		return e
	}
	startAgent := func() error {
		var err error
		daemon, err = c.start(agentBin, "agent", "serve", "-config", cfgPath)
		return err
	}
	if e = startAgent(); e != nil {
		return e
	}
	ctx := context.Background()
	for end := time.Now().Add(15 * time.Second); ; {
		_, e = agent.Control(ctx, cfg, "status")
		if e == nil {
			break
		}
		if time.Now().After(end) {
			return e
		}
		time.Sleep(100 * time.Millisecond)
	}
	c.pass("isolated production agent and HTTP-only Nginx bootstrap")
	hook := agentBin + " certbot-hook -config " + cfgPath
	args := []string{"certonly", "--non-interactive", "--agree-tos", "--email", "demo@example.invalid", "--webroot", "-w", cfg.Webroot, "-d", cfg.Host, "--cert-name", cfg.Host, "--key-type", "rsa", "--rsa-key-size", "2048", "--server", "https://localhost:14000/dir", "--config-dir", filepath.Join(dir, "certbot"), "--work-dir", filepath.Join(dir, "certbot-work"), "--logs-dir", filepath.Join(dir, "certbot-logs"), "--deploy-hook", hook, "--no-directory-hooks"}
	issue := exec.Command(certbotBin, args...)
	issue.Env = append(c.env, "REQUESTS_CA_BUNDLE="+filepath.Join(pkidir, "acme-root.pem"))
	out, e := issue.CombinedOutput()
	os.WriteFile(filepath.Join(dir, "evidence/certbot-initial.log"), out, 0600)
	if e != nil {
		return fmt.Errorf("Certbot initial issuance: %w: %s", e, out)
	}
	var status agent.Status
	raw, e := agent.Control(ctx, cfg, "status")
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &status); e != nil {
		return e
	}
	if !status.Ready || status.Deployment.Phase != "committed" {
		return fmt.Errorf("Certbot returned success without confirmed deployment: %s", raw)
	}
	c.pass("real Certbot HTTP-01 issuance, deploy-hook publication and verified Nginx activation")
	if e = agent.Check(ctx, cfg); e != nil {
		return e
	}
	c.pass("fresh verified browser-facing TLS leaf covered by qualified binding")
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(tlsRoot)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, cfg.ProbeAddress)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	cache.Store(&cacheproxy.Proxy{Origin: "https://" + cfg.Host + ":8443/cache-origin", Client: client})

	get := func(path, method string, conditional bool) (*http.Response, []byte, error) {
		req, e := http.NewRequest(method, "https://"+cfg.Host+":8443"+path, nil)
		if e != nil {
			return nil, nil, e
		}
		if conditional {
			req.Header.Set("If-None-Match", `"app"`)
		}
		r, e := client.Do(req)
		if e != nil {
			return nil, nil, e
		}
		defer r.Body.Close()
		b, e := io.ReadAll(r.Body)
		return r, b, e
	}
	for _, test := range []struct {
		path, method string
		code         int
		conditional  bool
	}{{"/app", "GET", 201, false}, {"/redirect", "GET", 307, false}, {"/error", "GET", 418, false}, {"/conditional", "GET", 304, true}} {
		r, _, e := get(test.path, test.method, test.conditional)
		if e != nil {
			return e
		}
		if r.StatusCode != test.code || r.Header.Get("Cache-Control") != "public, max-age=600" || !strings.Contains(strings.Join(r.Header.Values("Link"), ","), "preload") || !strings.Contains(strings.Join(r.Header.Values("Link"), ","), store.DiscoveryURL) {
			return fmt.Errorf("application response changed: %s %d %v", test.path, r.StatusCode, r.Header)
		}
	}
	c.pass("ordinary reverse-proxy streaming/cache/Link, redirect, error and conditional responses")
	for _, test := range []struct {
		method string
		code   int
	}{{"GET", 200}, {"HEAD", 200}, {"POST", 405}} {
		r, b, e := get(store.DiscoveryURL, test.method, false)
		if e != nil {
			return e
		}
		if r.StatusCode != test.code || r.Header.Get("Cache-Control") != "no-store" || r.Header.Get("Link") != "" || (test.method == "HEAD" && len(b) != 0) {
			return fmt.Errorf("binding delivery semantics: %s %d %v", test.method, r.StatusCode, r.Header)
		}
		if test.method == "GET" {
			root, _ := credentials.ReadCert(cfg.TrustFile)
			if e = binding.Verify(string(b), cfg.Host, []*x509.Certificate{r.TLS.PeerCertificates[0]}, root, time.Now()); e != nil {
				return e
			}
		}
	}
	c.pass("binding GET/HEAD/no-store/method rejection and nonrecursive discovery")

	readBinding := func() (store.State, error) {
		var s store.State
		b, e := os.ReadFile(filepath.Join(cfg.StateDir, "bindings", "current.json"))
		if e == nil {
			e = json.Unmarshal(b, &s)
		}
		return s, e
	}
	initialBinding, e := readBinding()
	if e != nil {
		return e
	}
	cachedInitial, cachedBody, e := get("/cached", "GET", false)
	if e != nil {
		return e
	}
	cachedHash := sha256.Sum256(cachedBody)
	cachedLink := strings.Join(cachedInitial.Header.Values("Link"), ",")
	independentRefresh := false

	// Keep one active HTTP/1 stream on the original authenticated TLS connection.
	held, e := tls.Dial("tcp", cfg.ProbeAddress, &tls.Config{RootCAs: roots, ServerName: cfg.Host})
	if e != nil {
		return e
	}
	defer held.Close()
	oldID := binding.ID(held.ConnectionState().PeerCertificates[0])
	fmt.Fprintf(held, "GET /held HTTP/1.1\r\nHost: %s:8443\r\nConnection: keep-alive\r\n\r\n", cfg.Host)
	heldResp, e := http.ReadResponse(bufio.NewReader(held), nil)
	if e != nil {
		return e
	}
	defer func() { held.Close(); heldResp.Body.Close() }()
	var chunks atomic.Int64
	streamErr := make(chan error, 1)
	go func() {
		b := make([]byte, 12)
		for {
			n, e := heldResp.Body.Read(b)
			if n > 0 {
				chunks.Add(1)
			}
			if e != nil {
				streamErr <- e
				return
			}
		}
	}()
	originalID := status.Deployment.Active
	renewArgs := []string{"renew", "--non-interactive", "--no-random-sleep-on-renew", "--no-directory-hooks", "--config-dir", filepath.Join(dir, "certbot"), "--work-dir", filepath.Join(dir, "certbot-work"), "--logs-dir", filepath.Join(dir, "certbot-logs")}
	for end := time.Now().Add(150 * time.Second); ; {
		renewal := exec.Command(certbotBin, renewArgs...)
		renewal.Env = issue.Env
		out, e := renewal.CombinedOutput()
		f, _ := os.OpenFile(filepath.Join(dir, "evidence/certbot-renew.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if f != nil {
			f.Write(out)
			f.Close()
		}
		if e != nil {
			return fmt.Errorf("automatic Certbot renew: %w: %s", e, out)
		}
		raw, e = agent.Control(ctx, cfg, "status")
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &status); e != nil {
			return e
		}
		if current, err := readBinding(); err != nil {
			return err
		} else if !independentRefresh && current.Published > initialBinding.Published && current.Expires > initialBinding.Expires && reflect.DeepEqual(current.IDs, initialBinding.IDs) {
			independentRefresh = true
			writeJSON(filepath.Join(dir, "evidence/independent-refresh.json"), map[string]any{"before": initialBinding, "after": current})
			c.pass("independent signed binding refresh without TLS renewal")
		}
		if status.Ready && status.Deployment.Active != originalID {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("Certbot did not renew automatically")
		}
		select {
		case e := <-streamErr:
			return fmt.Errorf("held stream disconnected before renewal: %w", e)
		case <-time.After(2 * time.Second):
		}
	}
	// Fresh confirmation is separate from the original stream; no reconnect is
	// accepted as held-connection proof.
	if e = agent.Check(ctx, cfg); e != nil {
		return e
	}
	if binding.ID(held.ConnectionState().PeerCertificates[0]) != oldID || chunks.Load() < 2 {
		return fmt.Errorf("held TLS identity or active stream lost")
	}
	transport.CloseIdleConnections()
	r, b, e := get(store.DiscoveryURL, "GET", false)
	if e != nil {
		return e
	}
	root, _ := credentials.ReadCert(cfg.TrustFile)
	if e = binding.Verify(string(b), cfg.Host, []*x509.Certificate{held.ConnectionState().PeerCertificates[0], r.TLS.PeerCertificates[0]}, root, time.Now()); e != nil {
		return e
	}
	select {
	case e := <-streamErr:
		return fmt.Errorf("held stream ended during renewal: %w", e)
	default:
	}
	if e = writeJSON(filepath.Join(dir, "evidence/renewal-held.json"), map[string]any{"old_tls_id": oldID, "new_tls_id": status.Deployment.Active, "same_held_connection": true, "chunks": chunks.Load(), "both_bound": true}); e != nil {
		return e
	}
	c.pass("automatic Certbot renewal and fresh verified controlled activation")
	c.pass("original active TLS stream survives Nginx reload with both leaves bound")
	close(releaseHeld)
	if !independentRefresh {
		return fmt.Errorf("renewal occurred without separately observed independent refresh")
	}
	if time.Now().Unix() <= initialBinding.Expires {
		return fmt.Errorf("cache test must cross original signed expiry")
	}
	cachedRenewed, body, e := get("/cached", "GET", false)
	if e != nil {
		return e
	}
	if cachedRenewed.Header.Get("X-Qwac-Cache") != "hit" || sha256.Sum256(body) != cachedHash || strings.Join(cachedRenewed.Header.Values("Link"), ",") != cachedLink {
		return fmt.Errorf("cached response or Link changed across expiry/renewal")
	}
	if e = agent.Check(ctx, cfg); e != nil {
		return e
	}
	writeJSON(filepath.Join(dir, "evidence/cache-renewed.json"), map[string]any{"age": cachedRenewed.Header.Get("Age"), "original_exp": initialBinding.Expires, "link": cachedLink, "body_sha256": fmt.Sprintf("%x", cachedHash), "cache": "hit"})
	c.pass("real cache longer than original signed lifetime discovers renewed binding")
	replacement, e := replaceDemoQWAC(pkidir, "Nginx Replacement TEST")
	if e != nil {
		return e
	}
	if _, e = agent.Control(ctx, cfg, "replace-qwac"); e != nil {
		return e
	}
	r, b, e = get(store.DiscoveryURL, "GET", false)
	if e != nil {
		return e
	}
	if e = binding.Verify(string(b), cfg.Host, []*x509.Certificate{r.TLS.PeerCertificates[0]}, root, time.Now()); e != nil {
		return e
	}
	_, chain, _, _, e := binding.Decode(string(b))
	if e != nil {
		return e
	}
	if !chain[0].Equal(replacement) {
		return fmt.Errorf("QWAC replacement not installed")
	}
	cachedReplaced, body, e := get("/cached", "GET", false)
	if e != nil {
		return e
	}
	if cachedReplaced.Header.Get("X-Qwac-Cache") != "hit" || sha256.Sum256(body) != cachedHash || strings.Join(cachedReplaced.Header.Values("Link"), ",") != cachedLink {
		return fmt.Errorf("cached response changed across QWAC replacement")
	}
	c.pass("same-authority QWAC replacement through unchanged cached discovery")
	for end := time.Now().Add(5 * time.Second); ; {
		_, err := os.Stat(filepath.Join(cfg.StateDir, "bindings", filepath.Base(initialBinding.URL)))
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("expired historical document was not collected")
		}
		time.Sleep(100 * time.Millisecond)
	}
	current, e := readBinding()
	if e != nil {
		return e
	}
	if len(current.IDs) != 2 {
		return fmt.Errorf("collection lost held TLS coverage")
	}
	c.pass("authenticated automatic collection preserves current document and retained leaves")
	stopChild(daemon)
	daemon = nil
	r, _, e = get("/app", "GET", false)
	if e != nil {
		return e
	}
	if r.StatusCode < 500 {
		return fmt.Errorf("agent-down gate bypassed: %d", r.StatusCode)
	}
	c.pass("agent unavailability rejects protected application requests")
	if e = startAgent(); e != nil {
		return e
	}
	for end := time.Now().Add(15 * time.Second); ; {
		r, _, e = get("/app", "GET", false)
		if e == nil && r.StatusCode == 201 {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("agent restart did not reconcile: %v", e)
		}
		time.Sleep(100 * time.Millisecond)
	}
	c.pass("agent restart reconciles files and actual verified served TLS")
	return nil
}
