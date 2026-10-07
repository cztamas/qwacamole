package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cztamas/qwacamole/devtools/internal/browser"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: qwacamole browser-demo|firefox-demo|firefox-check|firefox-suite|demo|browser|browser-suite|integration-test|pki|fixture|lifecycle|refresh-test|failure-test|clean")
	}
	f := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	pluginBin := f.String("plugin-binary", "bin/caddy-plugin", "plugin-only Caddy binary for portable smoke")
	dir := f.String("state", ".local", "state directory")
	bin := f.String("browser", "", "stock browser executable")
	metadata := f.String("metadata", "", "genuine PKIMetadata version directory")
	driver := f.String("geckodriver", "", "Firefox geckodriver executable")
	certutil := f.String("certutil", "", "NSS certutil executable for isolated Firefox TLS trust")
	profile := f.String("profile", "", "new isolated profile path")
	ct := f.Bool("ct-exception", false, "disable only CT enforcement in this test process")
	tlsTrust := f.Bool("tls-trust", true, "inject TLS root")
	reuse := f.Bool("reuse-profile", false, "reuse existing isolated profile without trust patching")
	qwacTrust := f.Bool("qwac-trust", true, "inject QWAC root")
	url := f.String("url", "https://qwac.example.com:8443/", "navigation URL")
	screenshot := f.Bool("screenshot", false, "capture the rendered diagnostic page")
	throughExpiry := f.Bool("through-tls-expiry", false, "keep the same browser connection active past its TLS leaf expiry (up to ten minutes)")
	expect := f.String("expect", "positive", "required native browser result")
	name := f.String("name", "positive", "evidence case name")
	tlsRootPath := f.String("tls-root", "", "override TLS trust root PEM")
	country := f.String("country", "HU", "synthetic organization country")
	locality := f.String("locality", "Budapest", "synthetic organization locality")
	registration := f.String("registration", "NTRHU-TEST-0001", "synthetic organization identifier")
	org := f.String("organization", "Qwacamole Synthetic Organization TEST", "synthetic organization")
	_ = f.Parse(os.Args[2:])
	var e error
	switch os.Args[1] {
	case "firefox-integration":
		e = firefoxIntegration(*dir, *bin, *driver, *certutil)
	case "plugin-smoke":
		e = pluginSmoke(*dir, *pluginBin)
	case "firefox-suite":
		e = firefoxSuite(*dir, *bin, *driver, *certutil)
	case "firefox-check":
		if *bin == "" {
			*bin, e = browser.DetectFirefox()
			if e != nil {
				break
			}
		}
		if *driver == "" {
			*driver = ".tools/bin/geckodriver"
		}
		if *certutil == "" {
			*certutil, e = exec.LookPath("certutil")
			if e != nil {
				break
			}
		}
		rootPath := *tlsRootPath
		if rootPath == "" {
			rootPath = filepath.Join(*dir, "pki/pebble-root.pem")
		}
		var tr, qr *x509.Certificate
		tr, e = pki.ReadCert(rootPath)
		if e != nil {
			break
		}
		qr, e = pki.ReadCert(filepath.Join(*dir, "pki/qwac-leaf.pem"))
		if e != nil {
			break
		}
		if *profile == "" {
			*profile = filepath.Join(*dir, "profiles", "firefox-"+fmt.Sprint(time.Now().UnixNano()))
		}
		if !*reuse {
			e = browser.PrepareFirefox(*profile, *certutil, tr, *tlsTrust, *qwacTrust)
			if e != nil {
				break
			}
		}
		if filepath.Base(*name) != *name || *name == "." || *name == ".." {
			e = fmt.Errorf("name must be one filename component")
			break
		}
		evidence := filepath.Join(*dir, "evidence", "firefox-"+*name)
		e = os.MkdirAll(evidence, 0700)
		if e != nil {
			break
		}
		var result browser.FirefoxResult
		result, e = browser.RunFirefox(context.Background(), browser.FirefoxOptions{Binary: *bin, Driver: *driver, Profile: *profile, URL: *url, Evidence: evidence})
		if writeErr := writeJSON(filepath.Join(evidence, "firefox-result.json"), result); e == nil {
			e = writeErr
		}
		d, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(d))
		if e == nil {
			e = browser.AssertFirefox(result, *expect, qr)
		}
	case "firefox-demo":
		e = firefoxDemo(*dir, *bin, *driver, *certutil, pki.Identity{Host: "qwac.example.com", Organization: *org, Country: *country, Locality: *locality, Registration: *registration})
	case "browser-demo":
		e = browserDemo(*dir, *bin, *metadata, pki.Identity{Host: "qwac.example.com", Organization: *org, Country: *country, Locality: *locality, Registration: *registration})
	case "clean":
		e = clean(*dir)
	case "integration-test":
		e = integration(*dir, *bin, *metadata)
	case "refresh-test":
		e = refreshTest(*dir)
	case "failure-test":
		e = failureTest(*dir)
	case "browser-suite":
		e = suite(*dir, *bin, *metadata)
	case "lifecycle":
		e = lifecycle(*dir)
	case "demo":
		e = demo(*dir)
	case "pki":
		e = pki.Generate(filepath.Join(*dir, "pki"), pki.Identity{Host: "qwac.example.com", Organization: *org, Country: *country, Locality: *locality, Registration: *registration})
	case "fixture":
		e = fixture(*dir)
	case "browser":
		if *bin == "" || *metadata == "" {
			b, m, err := browser.Detect()
			if err != nil {
				log.Fatal(err)
			}
			if *bin == "" {
				*bin = b
			}
			if *metadata == "" {
				*metadata = m
			}
		}
		rootPath := *tlsRootPath
		if rootPath == "" {
			rootPath = filepath.Join(*dir, "pki/tls-root.pem")
		}
		tr, err := pki.ReadCert(rootPath)
		if err != nil {
			log.Fatal(err)
		}
		qr, err := pki.ReadCert(filepath.Join(*dir, "pki/qwac-root.pem"))
		if err != nil {
			log.Fatal(err)
		}
		if *profile == "" {
			*profile = filepath.Join(*dir, "profiles", *name+fmt.Sprint(time.Now().UnixNano()))
		}
		if !*reuse {
			if e = browser.Prepare(*profile, *metadata, tr, qr, *tlsTrust, *qwacTrust); e != nil {
				log.Fatal(e)
			}
		}
		_ = os.MkdirAll(filepath.Join(*dir, "evidence"), 0700)
		initURL, err := urlpkg(*url)
		if err != nil {
			log.Fatal(err)
		}
		var result browser.Result
		result, e = browser.Run(browser.Options{Binary: *bin, Profile: *profile, NetLog: filepath.Join(*dir, "evidence", *name+"-netlog.json"), URL: *url, InitURL: initURL, CTException: *ct, Screenshot: *screenshot, ThroughTLSExpiry: *throughExpiry})
		d, _ := json.MarshalIndent(result, "", "  ")
		writeErr := os.WriteFile(filepath.Join(*dir, "evidence", *name+"-result.json"), d, 0600)
		fmt.Println(string(d))
		if e == nil {
			e = writeErr
		}
		if e == nil {
			e = browser.Assert(result, *expect, qr)
		}
	default:
		log.Fatal("unknown command")
	}
	if e != nil {
		log.Fatal(e)
	}
}

func urlpkg(target string) (string, error) {
	u, e := url.Parse(target)
	if e != nil {
		return "", e
	}
	u.Path = "/init"
	u.RawQuery = ""
	return u.String(), nil
}

func clean(dir string) error {
	abs, e := filepath.Abs(dir)
	if e != nil {
		return e
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	if abs == cwd || abs == "/" {
		return fmt.Errorf("refusing cleanup of repository or filesystem root")
	}
	if _, e = os.Stat(filepath.Join(abs, ".qwacamole-state")); e != nil {
		return fmt.Errorf("refusing cleanup without project state marker: %w", e)
	}
	return os.RemoveAll(abs)
}
