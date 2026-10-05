package main

import (
	"fmt"
	"github.com/cztamas/qwacamole/devtools/internal/browser"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"path/filepath"
	"time"
)

func suite(dir, binary, metadata string) error {
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
	tlsRoot, e := pki.ReadCert(filepath.Join(dir, "pki/tls-root.pem"))
	if e != nil {
		return e
	}
	qwacRoot, e := pki.ReadCert(filepath.Join(dir, "pki/qwac-root.pem"))
	if e != nil {
		return e
	}
	type test struct {
		Name, Path    string
		TLS, QWAC, CT bool
	}
	tests := []test{{"positive", "9443/positive", true, true, true}, {"no-qwac-trust", "9443/positive", true, false, true}, {"no-tls-trust", "9443/positive", false, true, true}, {"signature", "9443/signature", true, true, true}, {"wrong-tls", "9443/wrong-tls", true, true, true}, {"qwac-hostname", "9443/qwac-hostname", true, true, true}, {"expired-binding", "9443/expired-binding", true, true, true}, {"qwac-profile", "9443/qwac-profile", true, true, true}, {"tls-hostname", "9444/positive", true, true, true}, {"tls-expiry", "9445/positive", true, true, true}, {"ct-required", "9443/positive", true, true, false}}
	results := map[string]any{}
	failures := 0
	for _, tc := range tests {
		profile := filepath.Join(dir, "profiles", tc.Name+fmt.Sprint(time.Now().UnixNano()))
		if e = browser.Prepare(profile, metadata, tlsRoot, qwacRoot, tc.TLS, tc.QWAC); e != nil {
			return e
		}
		target := "https://qwac.example.com:" + tc.Path
		initURL, _ := urlpkg(target)
		r, err := browser.Run(browser.Options{Binary: binary, Profile: profile, URL: target, InitURL: initURL, NetLog: filepath.Join(dir, "evidence", tc.Name+"-netlog.json"), CTException: tc.CT})
		if err == nil {
			err = browser.Assert(r, tc.Name, qwacRoot)
		}
		entry := map[string]any{"passed": err == nil, "browser_version": r.Version}
		if err != nil {
			entry["error"] = err.Error()
			failures++
			fmt.Println("FAIL", tc.Name, err)
		} else {
			fmt.Println("PASS", tc.Name)
		}
		results[tc.Name] = entry
		_ = writeJSON(filepath.Join(dir, "evidence", tc.Name+"-result.json"), r)
		_ = writeJSON(filepath.Join(dir, "evidence/browser-suite.json"), results)
	}
	if failures > 0 {
		return fmt.Errorf("%d native browser acceptance tests failed; see %s", failures, filepath.Join(dir, "evidence/browser-suite.json"))
	}
	return nil
}
