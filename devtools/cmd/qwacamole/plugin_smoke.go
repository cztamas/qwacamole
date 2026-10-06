package main

// This portable development check supplies TEST credentials to a separate,
// plugin-only Caddy process. None of this enrollment or DNS code is in the plugin.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
	"github.com/miekg/dns"
)

type pluginCheck struct {
	dir, bin string
	env      []string
	client   *http.Client
	results  []string
	caddy    *exec.Cmd
	pebble   *exec.Cmd
}

func (c *pluginCheck) command(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(c.bin, args...)
	cmd.Env = c.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	d, e := cmd.Output()
	os.WriteFile(filepath.Join(c.dir, "evidence", name+".log"), append(append([]byte(nil), d...), stderr.Bytes()...), 0600)
	if e != nil {
		return d, fmt.Errorf("%s: %w: %s %s", name, e, d, stderr.Bytes())
	}
	return d, nil
}
func (c *pluginCheck) start(bin, name string, args ...string) (*exec.Cmd, error) {
	f, e := os.OpenFile(filepath.Join(c.dir, "evidence", name+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	cmd := exec.Command(bin, args...)
	cmd.Env = c.env
	cmd.Stdout = f
	cmd.Stderr = f
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	return cmd, nil
}
func stopChild(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}
}
func (c *pluginCheck) pass(s string) { c.results = append(c.results, s); fmt.Println("PASS", s) }
func (c *pluginCheck) state() (store.State, error) {
	var s store.State
	d, e := os.ReadFile(filepath.Join(c.dir, "bindings/current.json"))
	if e == nil {
		e = json.Unmarshal(d, &s)
	}
	return s, e
}
func (c *pluginCheck) get(path, method string) (*http.Response, []byte, error) {
	req, e := http.NewRequest(method, "https://qwac.example.com:8443"+path, nil)
	if e != nil {
		return nil, nil, e
	}
	res, e := c.client.Do(req)
	if e != nil {
		return nil, nil, e
	}
	defer res.Body.Close()
	d, e := io.ReadAll(res.Body)
	return res, d, e
}
func (c *pluginCheck) ready() error {
	for end := time.Now().Add(30 * time.Second); time.Now().Before(end); {
		res, _, e := c.get("/app", "GET")
		if e == nil && res.StatusCode == 201 {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("plugin Caddy did not serve protected application; see caddy.log")
}
func cloneJSON(cfg map[string]any) map[string]any {
	d, _ := json.Marshal(cfg)
	var out map[string]any
	json.Unmarshal(d, &out)
	return out
}
func httpServer(cfg map[string]any) map[string]any {
	for _, s := range cfg["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any) {
		return s.(map[string]any)
	}
	panic("no HTTP server")
}
func issuerJSON(cfg map[string]any) map[string]any {
	return cfg["apps"].(map[string]any)["tls"].(map[string]any)["automation"].(map[string]any)["policies"].([]any)[0].(map[string]any)["issuers"].([]any)[0].(map[string]any)
}
func findBindingHandler(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		if x["handler"] == "qwac" {
			return x
		}
		for _, z := range x {
			if h := findBindingHandler(z); h != nil {
				return h
			}
		}
	case []any:
		for _, z := range x {
			if h := findBindingHandler(z); h != nil {
				return h
			}
		}
	}
	return nil
}
func (c *pluginCheck) validateJSON(cfg map[string]any, name string, wantOK bool) error {
	path := filepath.Join(c.dir, name+".json")
	if e := writeJSON(path, cfg); e != nil {
		return e
	}
	_, e := c.command(name, "validate", "--config", path)
	if (e == nil) != wantOK {
		return fmt.Errorf("validation %s wanted success=%v: %v", name, wantOK, e)
	}
	return nil
}

func pluginSmoke(parent, pluginBin string) (retErr error) {
	parent, e := filepath.Abs(parent)
	if e != nil {
		return e
	}
	os.MkdirAll(parent, 0700)
	dir, e := os.MkdirTemp(parent, "plugin-smoke-")
	if e != nil {
		return e
	}
	fmt.Println("STATE", dir)
	bin, e := filepath.Abs(pluginBin)
	if e != nil {
		return e
	}
	c := &pluginCheck{dir: dir, bin: bin}
	os.MkdirAll(filepath.Join(dir, "evidence"), 0700)
	defer func() {
		stopChild(c.caddy)
		stopChild(c.pebble)
		writeJSON(filepath.Join(dir, "evidence/result.json"), map[string]any{"checks": c.results, "error": fmt.Sprint(retErr), "plugin_binary": bin})
	}()
	id := pki.Identity{Host: "qwac.example.com", Organization: "Plugin supplied TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST-PLUGIN"}
	pkidir := filepath.Join(dir, "pki")
	if e = pki.Generate(pkidir, id); e != nil {
		return e
	}
	// A real streaming application with its own cache and multiple Link fields.
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer ln.Close()
	upstream := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `</style>; rel="preload"`)
		w.Header().Add("Link", `</next>; rel="next"`)
		w.Header().Set("Cache-Control", "public, max-age=321")
		w.WriteHeader(201)
		w.Write([]byte("application-first\n"))
		w.(http.Flusher).Flush()
		w.Write([]byte("application-second\n"))
	})}
	go upstream.Serve(ln)
	defer upstream.Close()
	docroot := filepath.Join(dir, "www")
	os.MkdirAll(docroot, 0700)
	os.WriteFile(filepath.Join(docroot, "index.html"), []byte("ordinary static application\n"), 0600)
	leaf, _ := os.ReadFile(filepath.Join(pkidir, "qwac-leaf.pem"))
	inter, _ := os.ReadFile(filepath.Join(pkidir, "qwac-intermediate.pem"))
	bundle := filepath.Join(pkidir, "bundle.pem")
	os.WriteFile(bundle, append(leaf, inter...), 0600)
	c.env = append(os.Environ(), "STATE="+dir, "UPSTREAM="+ln.Addr().String(), "QWAC_SITE=https://qwac.example.com:8443", "QWAC_HOST=qwac.example.com", "QWAC_STATE="+filepath.Join(dir, "bindings"), "QWAC_CHAIN="+bundle, "QWAC_KEY="+filepath.Join(pkidir, "qwac-leaf-key.pem"), "QWAC_TRUST="+filepath.Join(pkidir, "qwac-root.pem"), "DOCUMENT_ROOT="+docroot, "ACME_DIRECTORY=https://localhost:14000/dir", "XDG_DATA_HOME="+filepath.Join(dir, "xdg-data"), "XDG_CONFIG_HOME="+filepath.Join(dir, "xdg-config"), "PEBBLE_VA_NOSLEEP=1", "PEBBLE_VA_ALWAYS_VALID=0", "PEBBLE_AUTHZREUSE=0", "PEBBLE_WFE_NONCEREJECT=0")
	inventory, e := c.command("module-inventory", "list-modules")
	if e != nil {
		return e
	}
	for _, id := range []string{"tls.issuance.qwac", "http.handlers.qwac"} {
		if !strings.Contains(string(inventory), id+"\n") {
			return fmt.Errorf("missing module %s", id)
		}
	}
	if strings.Contains(string(inventory), "qwac_demo") {
		return fmt.Errorf("dev module leaked")
	}
	c.pass("plugin-only inventory")
	var cfg map[string]any
	for _, example := range []string{"pebble", "reverse-proxy", "file-server"} {
		path := "config/Caddyfile." + example
		d, e := c.command("adapt-"+example, "adapt", "--config", path, "--adapter", "caddyfile", "--pretty")
		if e != nil {
			return e
		}
		var adapted map[string]any
		if e = json.Unmarshal(d, &adapted); e != nil {
			return e
		}
		if e = c.validateJSON(adapted, "validate-"+example, true); e != nil {
			return e
		}
		if example == "pebble" {
			cfg = adapted
		}
	}
	c.pass("all three Caddyfile examples adapt and validate")
	original, _ := os.ReadFile("config/Caddyfile.pebble")
	for name, text := range map[string]string{"unknown-option": strings.Replace(string(original), "lifetime 45s", "lifetmie 45s", 1), "missing-key": strings.Replace(string(original), "key_file {$STATE}/pki/qwac-leaf-key.pem", "", 1), "invalid-duration": strings.Replace(string(original), "lifetime 45s", "lifetime someday", 1), "duplicate-setting": strings.Replace(string(original), "lifetime 45s", "lifetime 45s\n lifetime 1m", 1)} {
		path := filepath.Join(dir, name+".Caddyfile")
		os.WriteFile(path, []byte(text), 0600)
		if _, e = c.command(name, "adapt", "--config", path, "--adapter", "caddyfile"); e == nil {
			return fmt.Errorf("bad syntax accepted: %s", name)
		}
	}
	for _, name := range []string{"missing-trust", "inconsistent-handler", "fallback", "external-manager", "on-demand", "wrong-host"} {
		bad := cloneJSON(cfg)
		iss := issuerJSON(bad)
		switch name {
		case "missing-trust":
			delete(iss["binding"].(map[string]any), "trust_files")
		case "inconsistent-handler":
			findBindingHandler(bad)["binding"].(map[string]any)["lifetime"] = "2m"
		case "fallback":
			p := bad["apps"].(map[string]any)["tls"].(map[string]any)["automation"].(map[string]any)["policies"].([]any)[0].(map[string]any)
			p["issuers"] = append(p["issuers"].([]any), map[string]any{"module": "acme"})
		case "external-manager":
			p := bad["apps"].(map[string]any)["tls"].(map[string]any)["automation"].(map[string]any)["policies"].([]any)[0].(map[string]any)
			p["get_certificate"] = []any{map[string]any{"via": "http", "url": "http://127.0.0.1:9999"}}
		case "on-demand":
			p := bad["apps"].(map[string]any)["tls"].(map[string]any)["automation"].(map[string]any)["policies"].([]any)[0].(map[string]any)
			p["on_demand"] = true
		case "wrong-host":
			iss["binding"].(map[string]any)["hostname"] = "other.example.com"
		}
		if e = c.validateJSON(bad, "invalid-"+name, false); e != nil {
			return e
		}
	}
	c.pass("invalid syntax, missing trust, inconsistent pairing and bypass policies reject")
	// Existing numeric lifetime JSON remains valid after adding explicit trust.
	numeric := cloneJSON(cfg)
	issuerJSON(numeric)["binding"].(map[string]any)["lifetime"] = int64(45 * time.Second)
	findBindingHandler(numeric)["binding"].(map[string]any)["lifetime"] = int64(45 * time.Second)
	if e = c.validateJSON(numeric, "legacy-numeric-duration", true); e != nil {
		return e
	}
	c.pass("legacy numeric JSON duration migration")
	// Accelerate real ACME renewal only in this test's adapted JSON.
	automation := cfg["apps"].(map[string]any)["tls"].(map[string]any)["automation"].(map[string]any)
	automation["renew_interval"] = "2s"
	automation["policies"].([]any)[0].(map[string]any)["renewal_window_ratio"] = 0.8
	configPath := filepath.Join(dir, "caddy.json")
	if e = writeJSON(configPath, cfg); e != nil {
		return e
	}
	dnsMux := dns.NewServeMux()
	dnsMux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for _, q := range r.Question {
			if q.Name == "qwac.example.com." && q.Qtype == dns.TypeA {
				m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}, A: net.IPv4(127, 0, 0, 1)})
			}
		}
		w.WriteMsg(m)
	})
	ds := &dns.Server{Addr: "127.0.0.1:8053", Net: "udp", Handler: dnsMux}
	go ds.ListenAndServe()
	defer ds.Shutdown()
	pebbleConfig := map[string]any{"pebble": map[string]any{"listenAddress": "127.0.0.1:14000", "managementListenAddress": "127.0.0.1:15000", "certificate": filepath.Join(pkidir, "acme-leaf.pem"), "privateKey": filepath.Join(pkidir, "acme-leaf-key.pem"), "httpPort": 5002, "tlsPort": 5001, "profiles": map[string]any{"default": map[string]any{"description": "Plugin smoke real renewal", "validityPeriod": 90}}, "retryAfter": map[string]int{"authz": 1, "order": 1}}}
	pebblePath := filepath.Join(dir, "pebble.json")
	writeJSON(pebblePath, pebbleConfig)
	c.pebble, e = c.start(".tools/bin/pebble", "pebble", "-config", pebblePath, "-dnsserver", "127.0.0.1:8053")
	if e != nil {
		return e
	}
	acmeRoot, e := pki.ReadCert(filepath.Join(pkidir, "acme-root.pem"))
	if e != nil {
		return e
	}
	pool := x509.NewCertPool()
	pool.AddCert(acmeRoot)
	management := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}, Timeout: time.Second}
	var roots []byte
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
		res, e := management.Get("https://localhost:15000/roots/0")
		if e == nil {
			roots, _ = io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode == 200 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	tlsPool := x509.NewCertPool()
	if !tlsPool.AppendCertsFromPEM(roots) {
		return fmt.Errorf("no Pebble TLS root")
	}
	os.WriteFile(filepath.Join(pkidir, "pebble-root.pem"), roots, 0600)
	c.client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: tlsPool}, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:8443")
	}, ForceAttemptHTTP2: true}, Timeout: 5 * time.Second}
	c.caddy, e = c.start(c.bin, "caddy", "run", "--config", configPath)
	if e != nil {
		return e
	}
	if e = c.ready(); e != nil {
		return e
	}
	initial, e := c.state()
	if e != nil {
		return e
	}
	res, body, e := c.get("/app", "GET")
	if e != nil {
		return e
	}
	if res.StatusCode != 201 || string(body) != "application-first\napplication-second\n" || len(res.Header.Values("Link")) != 3 || res.Header.Get("Cache-Control") != "public, max-age=321" {
		return fmt.Errorf("application/header preservation failed: %v %s", res, body)
	}
	c.pass("real reverse_proxy status/body/streaming and header preservation")
	get, doc, e := c.get(initial.URL, "GET")
	if e != nil {
		return e
	}
	head, headBody, e := c.get(initial.URL, "HEAD")
	if e != nil {
		return e
	}
	if get.StatusCode != 200 || head.StatusCode != 200 || len(headBody) != 0 || len(doc) == 0 || get.Header.Get("Content-Type") != "application/jose" || get.Header.Get("Cache-Control") != "no-store" || head.ContentLength != int64(len(doc)) {
		return fmt.Errorf("binding GET/HEAD failed")
	}
	for _, test := range []struct {
		path, method string
		status       int
	}{{initial.URL, "POST", 405}, {"/bindings/current.json", "GET", 404}, {"/bindings/../current.json", "GET", 404}, {"/bindings/" + strings.Repeat("0", 64) + ".jose", "GET", 404}} {
		res, _, e := c.get(test.path, test.method)
		if e != nil || res.StatusCode != test.status {
			return fmt.Errorf("endpoint control %+v: %v %v", test, res, e)
		}
	}
	c.pass("binding GET/HEAD, methods, path validation and no-store isolation")
	// Detect automatic renewal without restart/credential changes and verify union.
	var renewed store.State
	for end := time.Now().Add(100 * time.Second); time.Now().Before(end); {
		renewed, e = c.state()
		if e == nil && len(renewed.IDs) > len(initial.IDs) {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if len(renewed.IDs) <= len(initial.IDs) {
		return fmt.Errorf("no real automatic renewal")
	}
	root, _ := pki.ReadCert(filepath.Join(pkidir, "qwac-root.pem"))
	var certs []*x509.Certificate
	for _, raw := range renewed.Certs {
		der, _ := base64.StdEncoding.DecodeString(raw)
		crt, e := x509.ParseCertificate(der)
		if e != nil {
			return e
		}
		certs = append(certs, crt)
	}
	_, doc, e = c.get(renewed.URL, "GET")
	if e != nil {
		return e
	}
	if e = binding.Verify(string(doc), id.Host, certs, root, time.Now()); e != nil {
		return e
	}
	c.pass("real automatic Pebble renewal and durable binding coverage")
	before, e := c.state()
	if e != nil {
		return e
	}
	for n := 0; n < 3; n++ {
		if _, e = c.command(fmt.Sprintf("unchanged-reload-%d", n), "reload", "--config", configPath, "--force"); e != nil {
			return e
		}
		if e = c.ready(); e != nil {
			return e
		}
	}
	after, _ := c.state()
	if len(after.IDs) < len(before.IDs) || after.Owner != before.Owner {
		return fmt.Errorf("unchanged reload lost coverage/identity")
	}
	c.pass("three unchanged forced Caddy reloads")
	if out, e := c.command("conflicting-process-owner", "validate", "--config", configPath); e == nil || !strings.Contains(fmt.Sprint(e), "already owned") {
		return fmt.Errorf("cross-process ownership not rejected at state lock: %v %s", e, out)
	}
	c.pass("conflicting process rejected by exclusive state lock")
	// Replace leaf and key at exactly the same configured paths, same authority.
	oldLeaf, e := pki.ReadCert(filepath.Join(pkidir, "qwac-leaf.pem"))
	if e != nil {
		return e
	}
	ca, _ := pki.ReadCert(filepath.Join(pkidir, "qwac-intermediate.pem"))
	caKey, _ := pki.ReadKey(filepath.Join(pkidir, "qwac-intermediate-key.pem"))
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return e
	}
	tmpl := *oldLeaf
	tmpl.RawSubject = nil
	tmpl.SerialNumber = pki.Serial()
	tmpl.Subject.Organization = []string{"Plugin replacement TEST"}
	tmpl.Subject.ExtraNames = append(tmpl.Subject.ExtraNames, oldLeaf.Subject.Names...)
	for n, v := range tmpl.Subject.ExtraNames {
		if v.Type.String() == "2.5.4.10" {
			tmpl.Subject.ExtraNames[n].Value = "Plugin replacement TEST"
		}
	}
	tmpl.ExtraExtensions = oldLeaf.Extensions
	crt, e := pki.Issue(&tmpl, ca, &key.PublicKey, caKey)
	if e != nil {
		return e
	}
	pki.WriteCert(filepath.Join(pkidir, "qwac-leaf.pem"), crt)
	pki.WriteKey(filepath.Join(pkidir, "qwac-leaf-key.pem"), key)
	if _, e = c.command("replacement-reload", "reload", "--config", configPath, "--force"); e != nil {
		return e
	}
	if e = c.ready(); e != nil {
		return e
	}
	replaced, _ := c.state()
	if replaced.Owner.Organization != "Plugin replacement TEST" || len(replaced.IDs) < len(after.IDs) {
		return fmt.Errorf("replacement was not published with retained coverage")
	}
	c.pass("same-path QWAC leaf/key replacement under same authority")
	keyPath := filepath.Join(pkidir, "qwac-leaf-key.pem")
	validKey, _ := os.ReadFile(keyPath)
	os.WriteFile(keyPath, []byte("invalid key"), 0600)
	if _, e = c.command("rejected-credentials-reload", "reload", "--config", configPath, "--force"); e == nil {
		return fmt.Errorf("invalid credentials reload accepted")
	}
	if e = c.ready(); e != nil {
		return e
	}
	unchanged, _ := c.state()
	if unchanged.Owner != replaced.Owner || len(unchanged.IDs) < len(replaced.IDs) {
		return fmt.Errorf("rejected reload damaged working config")
	}
	os.WriteFile(keyPath, validKey, 0600)
	bad := cloneJSON(cfg)
	findBindingHandler(bad)["binding"].(map[string]any)["lifetime"] = "2m"
	badPath := filepath.Join(dir, "rejected-config.json")
	writeJSON(badPath, bad)
	if _, e = c.command("rejected-config-reload", "reload", "--config", badPath); e == nil {
		return fmt.Errorf("inconsistent reload accepted")
	}
	if e = c.ready(); e != nil {
		return e
	}
	c.pass("invalid credentials and inconsistent config reloads reject; old application remains active")
	stopChild(c.caddy)
	c.caddy = nil
	c.client.CloseIdleConnections()
	c.caddy, e = c.start(c.bin, "caddy", "run", "--config", configPath)
	if e != nil {
		return e
	}
	if e = c.ready(); e != nil {
		return e
	}
	restarted, _ := c.state()
	if restarted.Owner != replaced.Owner || len(restarted.IDs) < len(replaced.IDs) {
		return fmt.Errorf("restart lost replacement/coverage")
	}
	c.pass("real process restart and recovered binding history")
	// Reload to the complete file_server example using the same protected state.
	adapted, e := c.command("adapt-file-runtime", "adapt", "--config", "config/Caddyfile.file-server", "--adapter", "caddyfile", "--pretty")
	if e != nil {
		return e
	}
	var files map[string]any
	json.Unmarshal(adapted, &files)
	files["admin"] = cfg["admin"]
	files["storage"] = cfg["storage"]
	fileHTTP := files["apps"].(map[string]any)["http"].(map[string]any)
	baseHTTP := cfg["apps"].(map[string]any)["http"].(map[string]any)
	fileHTTP["http_port"] = baseHTTP["http_port"]
	fileHTTP["https_port"] = baseHTTP["https_port"]
	issuerJSON(files)["acme"] = issuerJSON(cfg)["acme"]
	fileCfg := filepath.Join(dir, "files.json")
	writeJSON(fileCfg, files)
	// Bundle also changes with credentials, as operators must keep supplied material consistent.
	newLeaf, _ := os.ReadFile(filepath.Join(pkidir, "qwac-leaf.pem"))
	os.WriteFile(bundle, append(newLeaf, inter...), 0600)
	if _, e = c.command("file-server-reload", "reload", "--config", fileCfg); e != nil {
		return e
	}
	res, d, e := c.get("/", "GET")
	if e != nil || res.StatusCode != 200 || string(d) != "ordinary static application\n" || !strings.Contains(strings.Join(res.Header.Values("Link"), ","), "tls-certificate-binding") {
		return fmt.Errorf("file_server runtime failed: %v %v %s", e, res, d)
	}
	c.pass("real file_server application and reachable binding endpoints")
	return nil
}
