package main

import (
	"fmt"
	"github.com/cztamas/qwacamole/devtools/internal/browser"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func integration(dir, binary, metadata string) error {
	if binary == "" || metadata == "" {
		b, m, e := browser.Detect()
		if e != nil {
			return e
		}
		if binary == "" {
			binary = b
		}
		if metadata == "" {
			metadata = m
		}
	}
	dir = filepath.Join(dir, "integration-"+time.Now().Format("20060102-150405"))
	if e := os.MkdirAll(filepath.Join(dir, "evidence"), 0700); e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	start := func(sub string) (*exec.Cmd, error) {
		c := exec.Command(exe, sub, "-state", dir)
		f, e := os.OpenFile(filepath.Join(dir, "evidence", sub+"-console.log"), os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		c.Stdout = f
		c.Stderr = f
		if e = c.Start(); e != nil {
			return nil, e
		}
		return c, nil
	}
	demo, e := start("demo")
	if e != nil {
		return e
	}
	defer func() { _ = demo.Process.Signal(syscall.SIGTERM); _ = demo.Wait() }()
	end := time.Now().Add(30 * time.Second)
	for {
		if _, e = os.Stat(filepath.Join(dir, "bindings/current.json")); e == nil {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("demo startup failed; inspect %s", dir)
		}
		time.Sleep(time.Second)
	}
	tr, e := pki.ReadCert(filepath.Join(dir, "pki/pebble-root.pem"))
	if e != nil {
		return e
	}
	qr, e := pki.ReadCert(filepath.Join(dir, "pki/qwac-root.pem"))
	if e != nil {
		return e
	}
	profile := filepath.Join(dir, "profiles/lifecycle")
	if e = browser.Prepare(profile, metadata, tr, qr, true, true); e != nil {
		return e
	}
	run := func(name string) error {
		r, e := browser.Run(browser.Options{Binary: binary, Profile: profile, NetLog: filepath.Join(dir, "evidence", name+"-netlog.json"), URL: "https://qwac.example.com:8443/?fresh=" + name, InitURL: "https://qwac.example.com:8443/init", CTException: true, ThroughTLSExpiry: name == "held-through-tls-expiry"})
		_ = writeJSON(filepath.Join(dir, "evidence", name+"-result.json"), r)
		if e != nil {
			return e
		}
		return browser.Assert(r, "positive", qr)
	}
	if e = run("initial"); e != nil {
		return e
	}
	fmt.Println("PASS native ACME initial binding")
	if e = lifecycle(dir); e != nil {
		return e
	}
	if e = run("renewed"); e != nil {
		return e
	}
	fmt.Println("PASS native renewed binding, profile unchanged")
	if e = run("held-through-tls-expiry"); e != nil {
		return e
	}
	fmt.Println("PASS continuous native HTTP/2 verification through TLS expiry and a full binding refresh lifetime")
	if e = os.WriteFile(filepath.Join(dir, "restart-request"), nil, 0600); e != nil {
		return e
	}
	end = time.Now().Add(15 * time.Second)
	for {
		if _, e = os.Stat(filepath.Join(dir, "evidence/restart.json")); e == nil {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("Caddy restart not observed")
		}
		time.Sleep(time.Second)
	}
	if e = run("restarted"); e != nil {
		return e
	}
	fmt.Println("PASS restart recovery")
	if e = refreshTest(dir); e != nil {
		return e
	}
	if e = failureTest(dir); e != nil {
		return e
	}
	if e = run("failure-recovery"); e != nil {
		return e
	}
	fmt.Println("PASS native acceptance after failure recovery")
	fixture, e := start("fixture")
	if e != nil {
		return e
	}
	defer func() { _ = fixture.Process.Signal(syscall.SIGTERM); _ = fixture.Wait() }()
	time.Sleep(time.Second)
	err := suite(dir, binary, metadata)
	summary := map[string]any{"initial_native_verified": true, "renewal_native_verified": true, "continuous_http2_through_tls_expiry": true, "profile_unchanged": true, "restart_recovered": true, "independent_refresh": true, "renewal_failure_gates": true, "negative_suite_passed": err == nil}
	if err != nil {
		summary["blocker"] = err.Error()
	}
	_ = writeJSON(filepath.Join(dir, "evidence/integration.json"), summary)
	fmt.Println("Evidence:", dir)
	return err
}
