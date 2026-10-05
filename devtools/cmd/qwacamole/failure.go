package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func failureTest(dir string) error {
	root, e := pki.ReadCert(filepath.Join(dir, "pki/pebble-root.pem"))
	if e != nil {
		return e
	}
	qr, e := pki.ReadCert(filepath.Join(dir, "pki/qwac-root.pem"))
	if e != nil {
		return e
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	getCert := func() (*x509.Certificate, error) {
		c, e := tls.Dial("tcp", "127.0.0.1:8443", &tls.Config{ServerName: "qwac.example.com", RootCAs: roots})
		if e != nil {
			return nil, e
		}
		defer c.Close()
		return c.ConnectionState().PeerCertificates[0], nil
	}
	logPath := filepath.Join(dir, "evidence/caddy-restarted.log")
	if _, e = os.Stat(logPath); e != nil {
		logPath = filepath.Join(dir, "evidence/caddy.log")
	}
	defer os.Remove(filepath.Join(dir, "bindings/fail-sign"))
	defer os.Remove(filepath.Join(dir, "bindings/fail-publish"))
	if e = os.WriteFile(filepath.Join(dir, "bindings/fail-sign"), nil, 0600); e != nil {
		return e
	}
	old, e := getCert()
	if e != nil {
		return e
	}
	reports := []any{}
	for _, mode := range []string{"fail-sign", "fail-publish"} {
		path := filepath.Join(dir, "bindings", mode)
		if e = os.WriteFile(path, nil, 0600); e != nil {
			return e
		}
		before, e := os.ReadFile(logPath)
		if e != nil {
			return e
		}
		deadline := time.Now().Add(4 * time.Minute)
		seen := false
		for time.Now().Before(deadline) {
			time.Sleep(time.Second)
			c, e := getCert()
			if e != nil {
				return fmt.Errorf("previous HTTPS unavailable during %s: %w", mode, e)
			}
			if !c.Equal(old) {
				return fmt.Errorf("replacement TLS activated despite %s", mode)
			}
			raw, e := os.ReadFile(filepath.Join(dir, "bindings/current.json"))
			if e != nil {
				return e
			}
			var s struct {
				URL string `json:"url"`
			}
			if e = jsonUnmarshal(raw, &s); e != nil {
				return e
			}
			doc, e := os.ReadFile(filepath.Join(dir, "bindings", filepath.Base(s.URL)))
			if e != nil {
				return e
			}
			if e = binding.Verify(string(doc), "qwac.example.com", []*x509.Certificate{old}, qr, time.Now()); e != nil {
				return fmt.Errorf("previous binding unavailable during %s: %w", mode, e)
			}
			logs, e := os.ReadFile(logPath)
			if e != nil {
				return e
			}
			newlog := ""
			if len(logs) >= len(before) {
				newlog = string(logs[len(before):])
			}
			if strings.Contains(newlog, "injected binding "+mode+" failure for new TLS certificate") {
				seen = true
				reports = append(reports, map[string]any{"failure": mode, "previous_tls_id": binding.ID(old), "old_https_and_binding_valid": true, "replacement_not_activated": true, "log_excerpt": newlog})
				fmt.Println("PASS", mode, "replacement rejected, old HTTPS and binding valid")
				break
			}
		}
		if !seen {
			return fmt.Errorf("no automatic renewal failure observed for %s", mode)
		}
		_ = os.Remove(path)
	}
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		c, e := getCert()
		if e != nil {
			return e
		}
		if !c.Equal(old) {
			reports = append(reports, map[string]any{"recovered_tls_id": binding.ID(c), "automatic_retry_recovered": true})
			return writeJSON(filepath.Join(dir, "evidence/failure-test.json"), reports)
		}
	}
	return fmt.Errorf("automatic retry did not recover")
}
func jsonUnmarshal(d []byte, v any) error { return json.Unmarshal(d, v) }
