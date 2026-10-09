package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/devtools/internal/browser"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
)

type nginxNative struct {
	mode, binary, metadata, driver, certutil string
	server                                   string
	dir, profile                             string
	root                                     *x509.Certificate
	trust                                    []byte
}

func (n *nginxNative) prepare(dir, pkidir string) error {
	n.dir = dir
	n.profile = filepath.Join(dir, "profiles", "nginx-"+n.mode)
	tlsRoot, e := pki.ReadCert(filepath.Join(pkidir, "pebble-root.pem"))
	if e != nil {
		return e
	}
	n.root, e = pki.ReadCert(filepath.Join(pkidir, "qwac-root.pem"))
	if e != nil {
		return e
	}
	if n.mode == "chrome" {
		detected, metadata, e := browser.Detect()
		if e != nil && (n.binary == "" || n.metadata == "") {
			return e
		}
		if n.binary == "" {
			n.binary = detected
		}
		if n.metadata == "" {
			n.metadata = metadata
		}
		if e = browser.Prepare(n.profile, n.metadata, tlsRoot, n.root, true, true); e != nil {
			return e
		}
	} else if n.mode == "firefox" {
		if n.binary == "" {
			n.binary, e = browser.DetectFirefox()
			if e != nil {
				return e
			}
		}
		if n.driver == "" {
			n.driver = ".tools/bin/geckodriver"
		}
		if n.certutil == "" {
			n.certutil, e = exec.LookPath("certutil")
			if e != nil {
				return e
			}
		}
		if e = browser.PrepareFirefox(n.profile, n.certutil, tlsRoot, true, true); e != nil {
			return e
		}
	} else {
		return fmt.Errorf("native browser must be chrome or firefox")
	}
	n.trust, e = n.readTrust()
	return e
}
func (n *nginxNative) readTrust() ([]byte, error) {
	if n.mode == "chrome" {
		paths, e := filepath.Glob(filepath.Join(n.profile, "PKIMetadata", "*", "crs.pb"))
		if e != nil || len(paths) != 1 {
			return nil, fmt.Errorf("one isolated Chrome root store required")
		}
		return os.ReadFile(paths[0])
	}
	listing, e := exec.Command(n.certutil, "-L", "-d", "sql:"+n.profile).Output()
	if e != nil {
		return nil, e
	}
	var grants []string
	for _, line := range strings.Split(string(listing), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && strings.ContainsAny(fields[len(fields)-1], "CTP") {
			grants = append(grants, strings.TrimSpace(line))
		}
	}
	root, e := exec.Command(n.certutil, "-L", "-d", "sql:"+n.profile, "-n", "Qwacamole Pebble TLS TEST", "-a").Output()
	if e != nil {
		return nil, e
	}
	prefs, e := os.ReadFile(filepath.Join(n.profile, "user.js"))
	if e != nil {
		return nil, e
	}
	return append(append([]byte(strings.Join(grants, "\n")), root...), prefs...), nil
}
func (n *nginxNative) check(name string, leaf *x509.Certificate, tlsID string) error {
	evidence := filepath.Join(n.dir, "evidence", "native-"+n.mode+"-"+name)
	if e := os.MkdirAll(evidence, 0700); e != nil {
		return e
	}
	if n.mode == "firefox" {
		r, e := browser.RunFirefox(context.Background(), browser.FirefoxOptions{Binary: n.binary, Driver: n.driver, Profile: n.profile, URL: "https://qwac.example.com:8443/cached", Evidence: evidence, ExpectedQWAC: leaf})
		if err := writeJSON(filepath.Join(evidence, "firefox-result.json"), r); err != nil {
			return err
		}
		if e != nil {
			return e
		}
		if e = browser.AssertFirefox(r, "positive", leaf); e != nil {
			return e
		}
		if strings.ToLower(strings.ReplaceAll(r.UI.TLSFingerprint, ":", "")) != tlsID {
			return fmt.Errorf("native Firefox did not use expected deployed TLS leaf")
		}
	} else {
		log, e := os.Create(filepath.Join(evidence, "chrome.log"))
		if e != nil {
			return e
		}
		defer log.Close()
		r, e := browser.Run(browser.Options{Binary: n.binary, Profile: n.profile, NetLog: filepath.Join(evidence, "netlog.json"), URL: "https://qwac.example.com:8443/cached", CTException: true, Console: log})
		if err := writeJSON(filepath.Join(evidence, "chrome-result.json"), r); err != nil {
			return err
		}
		if e != nil {
			return e
		}
		if e = browser.Assert(r, "positive", n.root); e != nil {
			return e
		}
		leafSeen, tlsSeen := false, false
		observedTLS := ""
		for _, event := range r.Events {
			if event.Type == "CERT_VERIFY_PROC_2QWAC_BINDING" && event.Phase == 1 {
				text, _ := event.Params["tls_certificate"].(string)
				block, _ := pem.Decode([]byte(text))
				if block != nil {
					cert, e := x509.ParseCertificate(block.Bytes)
					if e == nil && (tlsID == "" || binding.ID(cert) == tlsID) {
						tlsSeen = true
						observedTLS = binding.ID(cert)
					}
				}
			}
			if event.Type == "CERT_VERIFY_PROC_2QWAC" && event.Phase == 2 {
				chain, _ := event.Params["verified_cert"].(map[string]any)
				certs, _ := chain["certificates"].([]any)
				if len(certs) > 0 {
					raw, _ := certs[0].(string)
					block, _ := pem.Decode([]byte(raw))
					if block != nil {
						cert, e := x509.ParseCertificate(block.Bytes)
						if e == nil && cert.Equal(leaf) {
							leafSeen = true
						}
					}
				}
			}
		}
		if !leafSeen || !tlsSeen {
			return fmt.Errorf("native Chrome expected QWAC identity/TLS leaf evidence missing")
		}
		if tlsID == "" {
			tlsID = observedTLS // Caddy may renew while a native check is running.
		}
	}
	trust, e := n.readTrust()
	if e != nil {
		return e
	}
	if !bytes.Equal(trust, n.trust) {
		return fmt.Errorf("native profile trust changed across lifecycle")
	}
	sum := sha256.Sum256(trust)
	if e = writeJSON(filepath.Join(evidence, "identity-trust.json"), map[string]any{"owner": leaf.Subject.Organization, "qwac_id": binding.ID(leaf), "tls_id": tlsID, "same_profile": n.profile, "unchanged_trust_sha256": fmt.Sprintf("%x", sum)}); e != nil {
		return e
	}
	server := n.server
	if server == "" {
		server = "Nginx"
	}
	fmt.Println("PASS native", server, n.mode, name, "owner", leaf.Subject.Organization, "unchanged profile/trust")
	return nil
}
