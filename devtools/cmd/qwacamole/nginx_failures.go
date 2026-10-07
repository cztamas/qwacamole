package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

func nginxFailureChecks(c *pluginCheck, cfg agent.Config, cfgPath, agentBin, certbotBin string, env []string, client *http.Client) error {
	ctx := context.Background()
	faults := filepath.Join(c.dir, "faults")
	if e := os.MkdirAll(faults, 0700); e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	daemon, e := c.start(exe, "agent-faults", "agent-fault-service", "-config", cfgPath, "-state", faults)
	if e != nil {
		return e
	}
	defer stopChild(daemon)
	status := func() (agent.Status, error) {
		var s agent.Status
		d, e := agent.Control(ctx, cfg, "status")
		if e == nil {
			e = json.Unmarshal(d, &s)
		}
		return s, e
	}
	for end := time.Now().Add(15 * time.Second); ; {
		s, err := status()
		if err == nil && s.Ready {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("fault service reconciliation: %v %+v", err, s)
		}
		time.Sleep(100 * time.Millisecond)
	}
	flag := func(name string, enabled bool) error {
		path := filepath.Join(faults, name)
		if enabled {
			return os.WriteFile(path, []byte("development-only failure\n"), 0600)
		}
		return os.Remove(path)
	}
	readState := func() (store.State, error) {
		var s store.State
		d, e := os.ReadFile(filepath.Join(cfg.StateDir, "bindings", "current.json"))
		if e == nil {
			e = json.Unmarshal(d, &s)
		}
		return s, e
	}
	response := func(path string) (int, error) {
		r, e := client.Get("https://" + cfg.Host + ":8443" + path)
		if e != nil {
			return 0, e
		}
		defer r.Body.Close()
		_, e = io.Copy(io.Discard, r.Body)
		return r.StatusCode, e
	}
	before, e := status()
	if e != nil {
		return e
	}
	old := before.Deployment.Active
	oldState, e := readState()
	if e != nil {
		return e
	}
	assertOld := func() error {
		s, e := status()
		if e != nil || !s.Ready || s.Deployment.Active != old {
			return fmt.Errorf("prior usable deployment lost: %+v %v", s, e)
		}
		leaf, e := (agent.Nginx{Config: cfg}).Probe(ctx)
		if e != nil || binding.ID(leaf) != old {
			return fmt.Errorf("unexpected served leaf after failure: %v", e)
		}
		return agent.Check(ctx, cfg)
	}
	// Unlike the automatic-renewal acceptance earlier, this explicitly forced
	// issuance supplies new real ACME candidates for deterministic fault tests.
	issue := func(name string) (*x509.Certificate, error) {
		args := []string{"certonly", "--force-renewal", "--non-interactive", "--agree-tos", "--email", "demo@example.invalid", "--webroot", "-w", cfg.Webroot, "-d", cfg.Host, "--cert-name", cfg.Host, "--key-type", "rsa", "--rsa-key-size", "2048", "--server", "https://localhost:14000/dir", "--config-dir", filepath.Join(c.dir, "certbot"), "--work-dir", filepath.Join(c.dir, "certbot-work"), "--logs-dir", filepath.Join(c.dir, "certbot-logs"), "--deploy-hook", agentBin + " certbot-hook -config " + cfgPath, "--no-directory-hooks"}
		cmd := exec.Command(certbotBin, args...)
		cmd.Env = env
		out, commandErr := cmd.CombinedOutput()
		if e := os.WriteFile(filepath.Join(c.dir, "evidence", name+"-certbot.log"), out, 0600); e != nil {
			return nil, e
		}
		writeJSON(filepath.Join(c.dir, "evidence", name+"-certbot-exit.json"), map[string]any{"command_error": fmt.Sprint(commandErr), "automatic_renewal": false})
		leaf, e := credentials.ReadCert(filepath.Join(cfg.CertbotLineage, "fullchain.pem"))
		if e != nil || binding.ID(leaf) == old {
			return nil, fmt.Errorf("fault candidate issuance failed: %v %v: %s", commandErr, e, out)
		}
		return leaf, nil
	}
	if e = flag("sign-new", true); e != nil {
		return e
	}
	candidate, e := issue("signing-failure")
	if e != nil {
		return e
	}
	s, e := status()
	if e != nil || !strings.Contains(s.Deployment.LastError, "injected binding signing failure") {
		return fmt.Errorf("deploy-hook signing failure was not recorded independently: %+v %v", s, e)
	}
	if e = assertOld(); e != nil {
		return e
	}
	if e = flag("sign-new", false); e != nil {
		return e
	}
	c.pass("real Certbot candidate signing failure preserves prior TLS; hook error tracked separately from Certbot exit")
	keyPath := filepath.Join(cfg.CertbotLineage, "privkey.pem")
	key, e := os.ReadFile(keyPath)
	if e != nil {
		return e
	}
	badKey, e := os.ReadFile(filepath.Join(filepath.Dir(cfg.KeyFile), "tls-leaf-key.pem"))
	if e != nil {
		return e
	}
	if e = os.WriteFile(keyPath, badKey, 0600); e != nil {
		return e
	}
	_, mismatch := agent.Control(ctx, cfg, "deploy")
	if e = os.WriteFile(keyPath, key, 0600); e != nil {
		return e
	}
	if mismatch == nil || !strings.Contains(mismatch.Error(), "private key does not match") {
		return fmt.Errorf("mismatched candidate was not rejected: %v", mismatch)
	}
	qwacKey, e := os.ReadFile(cfg.KeyFile)
	if e != nil {
		return e
	}
	if e = os.WriteFile(cfg.KeyFile, badKey, 0600); e != nil {
		return e
	}
	_, invalid := agent.Control(ctx, cfg, "replace-qwac")
	if e = os.WriteFile(cfg.KeyFile, qwacKey, 0600); e != nil {
		return e
	}
	if invalid == nil {
		return fmt.Errorf("invalid supplied QWAC key accepted")
	}
	if e = assertOld(); e != nil {
		return e
	}
	c.pass("mismatched TLS key and invalid supplied QWAC credentials preserve prior deployment")
	for _, boundary := range []string{"document", "manifest"} {
		if e = flag(boundary, true); e != nil {
			return e
		}
		_, failed := agent.Control(ctx, cfg, "deploy")
		if e = flag(boundary, false); e != nil {
			return e
		}
		if failed == nil || !strings.Contains(failed.Error(), "injected binding") {
			return fmt.Errorf("%s publication failure hidden: %v", boundary, failed)
		}
		if e = assertOld(); e != nil {
			return e
		}
		got, e := readState()
		if e != nil || !reflect.DeepEqual(got.IDs, oldState.IDs) {
			return fmt.Errorf("failed publication changed retained set: %v", e)
		}
		if _, e = os.Stat(filepath.Join(cfg.StateDir, "generations", binding.ID(candidate))); !os.IsNotExist(e) {
			return fmt.Errorf("candidate staged before publication: %v", e)
		}
	}
	c.pass("sign/document/manifest failures never stage or activate the unbound real candidate")
	for _, boundary := range []string{"reload-once", "skip-reload-once"} {
		if e = flag(boundary, true); e != nil {
			return e
		}
		_, failed := agent.Control(ctx, cfg, "deploy")
		if failed == nil {
			return fmt.Errorf("%s wrongly confirmed candidate", boundary)
		}
		if boundary == "skip-reload-once" && !strings.Contains(failed.Error(), "fresh verified TLS confirmation") {
			return fmt.Errorf("zero-exit reload rejected for unrelated reason: %v", failed)
		}
		if e = assertOld(); e != nil {
			return e
		}
	}
	c.pass("reload failure and zero-exit nonactivation both restore fresh-verified prior TLS")
	config, e := os.ReadFile(cfg.NginxConfig)
	if e != nil {
		return e
	}
	if e = os.WriteFile(cfg.NginxConfig, append(config, []byte("invalid_nginx_directive;\n")...), 0600); e != nil {
		return e
	}
	_, failed := agent.Control(ctx, cfg, "deploy")
	if e = os.WriteFile(cfg.NginxConfig, config, 0600); e != nil {
		return e
	}
	if failed == nil || !strings.Contains(failed.Error(), "nginx config differs") {
		return fmt.Errorf("invalid Nginx configuration not rejected: %v", failed)
	}
	if _, e = agent.Control(ctx, cfg, "reconcile"); e != nil {
		return e
	}
	if e = assertOld(); e != nil {
		return e
	}
	if _, e = agent.Control(ctx, cfg, "deploy"); e != nil {
		return e
	}
	if e = agent.Check(ctx, cfg); e != nil {
		return e
	}
	s, e = status()
	if e != nil || !s.Ready || s.Deployment.Active != binding.ID(candidate) {
		return fmt.Errorf("safe retry did not activate candidate: %+v %v", s, e)
	}
	old = s.Deployment.Active
	c.pass("invalid Nginx config repairs/reconciles and safe retry activates the already-bound candidate")
	if e = flag("sign-all", true); e != nil {
		return e
	}
	s, e = status()
	if e != nil {
		return e
	}
	for time.Now().Unix() < s.Binding.BindingExpires-1 {
		time.Sleep(100 * time.Millisecond)
	}
	code, e := response("/delayed")
	if e != nil || code != 503 {
		return fmt.Errorf("response committed after binding expiry: %d %v", code, e)
	}
	c.pass("protected response delayed across signed expiry rejects at response commitment")
	for time.Now().Unix() <= s.Binding.BindingExpires {
		time.Sleep(100 * time.Millisecond)
	}
	code, e = response("/app")
	if e != nil || code != 403 {
		return fmt.Errorf("expired state gate bypassed: %d %v", code, e)
	}
	code, e = response(store.DiscoveryURL)
	if e != nil || code != 503 {
		return fmt.Errorf("expired binding delivered: %d %v", code, e)
	}
	c.pass("actual signed expiry closes Nginx protected-response gate and binding endpoint")
	if e = flag("sign-all", false); e != nil {
		return e
	}
	if _, e = agent.Control(ctx, cfg, "replace-qwac"); e != nil {
		return e
	}
	if e = agent.Check(ctx, cfg); e != nil {
		return e
	}
	c.pass("repaired signer safely republishes and reopens Nginx gate")
	full, e := readState()
	if e != nil || len(full.IDs) != cfg.MaxCertificates {
		return fmt.Errorf("capacity fixture is not full: %v %d", e, len(full.IDs))
	}
	extra, e := issue("capacity-failure")
	if e != nil {
		return e
	}
	s, e = status()
	if e != nil || !strings.Contains(s.Deployment.LastError, "capacity") {
		return fmt.Errorf("capacity refusal missing: %+v %v", s, e)
	}
	if e = assertOld(); e != nil {
		return e
	}
	got, e := readState()
	if e != nil || !reflect.DeepEqual(got.IDs, full.IDs) {
		return fmt.Errorf("capacity refusal pruned retained leaves: %v", e)
	}
	if _, e = os.Stat(filepath.Join(cfg.StateDir, "generations", binding.ID(extra))); !os.IsNotExist(e) {
		return fmt.Errorf("over-capacity TLS candidate staged: %v", e)
	}
	c.pass("real ACME over-capacity candidate refuses before staging without pruning old TLS coverage")
	return nil
}
