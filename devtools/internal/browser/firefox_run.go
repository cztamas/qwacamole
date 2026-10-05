package browser

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FirefoxOptions struct {
	Binary, Driver, Profile, URL, Evidence string
	Visible                                bool
	ExpectedQWAC                           *x509.Certificate
}

type FirefoxResult struct {
	QWACFailure         string    `json:"qwac_failure,omitempty"`
	Version             string    `json:"version"`
	Profile             string    `json:"profile"`
	AcceptInsecureCerts bool      `json:"accept_insecure_certs"`
	NavigationError     string    `json:"navigation_error,omitempty"`
	TLSFailure          string    `json:"tls_failure,omitempty"`
	UI                  FirefoxUI `json:"native_ui"`
}

type FirefoxUI struct {
	URL                   string `json:"url"`
	Connection            string `json:"connection"`
	Organization          string `json:"organization"`
	Country               string `json:"country"`
	QWACFingerprint       string `json:"qwac_fingerprint"`
	TLSFingerprint        string `json:"tls_fingerprint"`
	QualifiedText         string `json:"qualified_text"`
	QualifiedVisible      bool   `json:"qualified_visible"`
	CertificateOverridden bool   `json:"certificate_overridden"`
}

type firefoxDriver struct {
	base, session string
	client        *http.Client
}

func (d *firefoxDriver) call(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(method, d.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := d.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope struct {
		Value json.RawMessage `json:"value"`
	}
	if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("Firefox WebDriver %s %s: %s", method, path, envelope.Value)
	}
	if out != nil {
		return json.Unmarshal(envelope.Value, out)
	}
	return nil
}

func (d *firefoxDriver) command(method, path string, body, out any) error {
	return d.call(method, "/session/"+d.session+path, body, out)
}

func (d *firefoxDriver) click(selector string) error {
	var element map[string]string
	if err := d.command("POST", "/element", map[string]string{"using": "css selector", "value": selector}, &element); err != nil {
		return err
	}
	id := element["element-6066-11e4-a52e-4f735466cecf"]
	if id == "" {
		return fmt.Errorf("Firefox native UI element missing: %s", selector)
	}
	return d.command("POST", "/element/"+id+"/click", map[string]any{}, nil)
}

// RunFirefox reads the real browser identity panel through Mozilla WebDriver's
// chrome context. It never sets _qwac, edits UI, or substitutes a page verifier.
func RunFirefox(stop context.Context, o FirefoxOptions) (result FirefoxResult, err error) {
	profile, err := filepath.Abs(o.Profile)
	if err != nil {
		return result, err
	}
	log, err := os.OpenFile(filepath.Join(o.Evidence, "firefox-console.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return result, err
	}
	defer log.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return result, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	child := exec.Command(o.Driver, "--host", "127.0.0.1", "--port", fmt.Sprint(port), "--allow-system-access")
	child.Stdout, child.Stderr = log, log
	if err = child.Start(); err != nil {
		return result, err
	}
	done := make(chan struct{})
	go func() { _ = child.Wait(); close(done) }()
	defer func() {
		select {
		case <-done:
		default:
			_ = child.Process.Kill()
			<-done
		}
	}()
	d := &firefoxDriver{base: fmt.Sprintf("http://127.0.0.1:%d", port), client: &http.Client{Timeout: 45 * time.Second}}
	for end := time.Now().Add(10 * time.Second); ; {
		if err = d.call("GET", "/status", nil, nil); err == nil {
			break
		}
		if time.Now().After(end) {
			return result, err
		}
		select {
		case <-stop.Done():
			return result, stop.Err()
		case <-done:
			return result, fmt.Errorf("geckodriver exited during startup")
		case <-time.After(100 * time.Millisecond):
		}
	}
	args := []string{"-profile", profile, "-no-remote"}
	if !o.Visible {
		args = append(args, "-headless")
	}
	var session struct {
		ID           string `json:"sessionId"`
		Capabilities struct {
			Version             string `json:"browserVersion"`
			Profile             string `json:"moz:profile"`
			AcceptInsecureCerts bool   `json:"acceptInsecureCerts"`
		} `json:"capabilities"`
	}
	if err = d.call("POST", "/session", map[string]any{"capabilities": map[string]any{"alwaysMatch": map[string]any{
		"browserName": "firefox", "acceptInsecureCerts": false, "moz:firefoxOptions": map[string]any{"binary": o.Binary, "args": args},
	}}}, &session); err != nil {
		return result, err
	}
	d.session = session.ID
	defer d.command("DELETE", "", nil, nil)
	result.Version, result.Profile, result.AcceptInsecureCerts = session.Capabilities.Version, session.Capabilities.Profile, session.Capabilities.AcceptInsecureCerts
	if result.Profile != profile || result.AcceptInsecureCerts {
		return result, fmt.Errorf("Firefox did not use the exact isolated profile with TLS verification enabled")
	}
	navigationErr := d.command("POST", "/url", map[string]string{"url": o.URL}, nil)
	if err = d.command("POST", "/moz/context", map[string]string{"context": "chrome"}, nil); err != nil {
		return result, err
	}
	if navigationErr != nil {
		result.NavigationError = navigationErr.Error()
		if err = d.command("POST", "/execute/sync", map[string]any{"script": `return gBrowser.securityUI.secInfo?.errorCodeString||"";`, "args": []any{}}, &result.TLSFailure); err != nil {
			return result, err
		}
		if result.TLSFailure == "" {
			if err = d.command("POST", "/moz/context", map[string]string{"context": "content"}, nil); err != nil {
				return result, err
			}
			if err = d.command("POST", "/execute/async", map[string]any{"script": `const done=arguments[arguments.length-1];let n=0;const poll=()=>{const code=document.getFailedCertSecurityInfo?.()?.errorCodeString||document.getNetErrorInfo?.()?.errorCodeString||document.body.getAttribute("code")||document.getElementById("errorCode")?.textContent?.trim();if(code)done(code);else if(++n>100)done("");else setTimeout(poll,20);};poll();`, "args": []any{}}, &result.TLSFailure); err != nil {
				return result, err
			}
			if result.TLSFailure == "" {
				return result, navigationErr
			}
		}
		return result, nil
	}
	if err = d.click("#identity-icon-box"); err != nil {
		return result, err
	}
	var ready bool
	if err = d.command("POST", "/execute/async", map[string]any{"script": `const done=arguments[arguments.length-1]; const promise=typeof gTrustPanelHandler!="undefined"&&gTrustPanelHandler.qwacStatusPromise||gIdentityHandler._qwacStatusPromise; Promise.resolve(promise).then(()=>done(true),()=>done(false));`, "args": []any{}}, &ready); err != nil {
		return result, err
	}
	if !ready {
		return result, fmt.Errorf("Firefox native QWAC determination failed")
	}
	// Wait for the real popup to finish its native open animation before
	// sending the next click; WebDriver does not wait for XUL popup animations.
	var opened bool
	if err = d.command("POST", "/execute/async", map[string]any{"script": `const done=arguments[arguments.length-1]; let tries=0; const poll=()=>{const p=document.getElementById("identity-popup")||document.getElementById("trustpanel-popup"); if(p?.state=="open")done(true); else if(++tries>100)done(false); else setTimeout(poll,20);}; poll();`, "args": []any{}}, &opened); err != nil {
		return result, err
	}
	if !opened {
		return result, fmt.Errorf("Firefox native identity popup did not open")
	}
	var newPanel bool
	_ = d.command("POST", "/execute/sync", map[string]any{"script": `return !!document.getElementById("trustpanel-popup-connection");`, "args": []any{}}, &newPanel)
	securityButton := "#identity-popup-security-button"
	if newPanel {
		securityButton = "#trustpanel-popup-connection"
	}
	if err = d.click(securityButton); err != nil {
		return result, err
	}
	time.Sleep(500 * time.Millisecond)
	const snapshot = `const done=arguments[arguments.length-1]; const sec=gBrowser.securityUI.secInfo; const {QWACs}=ChromeUtils.importESModule("resource://gre/modules/psm/QWACs.sys.mjs");
Promise.resolve(gIdentityHandler._qwac||QWACs.determineQWACStatus(sec,gBrowser.currentURI,gBrowser.selectedBrowser.browsingContext)).then(q=>{
const e=document.getElementById("identity-popup-content-etsi"), owner=document.getElementById("identity-popup-content-owner"), extended=document.getElementById("identity-popup-securityView-extended-info"); done({
url:gBrowser.currentURI.spec, connection:extended?.getAttribute("connection")||"", organization:owner?.textContent||"", country:q?gIdentityHandler.getIdentityData(q).country:"", qwac_fingerprint:q?.sha256Fingerprint||"", tls_fingerprint:sec?.serverCert?.sha256Fingerprint||"",
qualified_text:e?.textContent||"", qualified_visible:!!e&&e.checkVisibility(), certificate_overridden:extended?.getAttribute("connection")=="secure-cert-user-overridden"
}); },()=>done(null));`
	if err = d.command("POST", "/execute/async", map[string]any{"script": snapshot, "args": []any{}}, &result.UI); err != nil {
		return result, err
	}
	// Gecko's screenshot API captures the main window; macOS native popup
	// windows are omitted. Native UI visibility and owner are recorded in JSON.
	if console, e := os.ReadFile(filepath.Join(o.Evidence, "firefox-console.log")); e == nil {
		for _, reason := range []string{"invalid signature", "TLS binding does not cover server certificate", "signing certificate not 2-QWAC", "header has expired", "header contains invalid parameter"} {
			if strings.Contains(string(console), `console.error: "`+reason+`"`) {
				result.QWACFailure = reason
				break
			}
		}
	}
	var encoded string
	if err = d.command("GET", "/screenshot", nil, &encoded); err != nil {
		return result, err
	}
	png, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return result, err
	}
	if err = os.WriteFile(filepath.Join(o.Evidence, "firefox-window.png"), png, 0600); err != nil {
		return result, err
	}
	if data, e := json.MarshalIndent(result, "", "  "); e != nil {
		return result, e
	} else if e = os.WriteFile(filepath.Join(o.Evidence, "firefox-result.json"), data, 0600); e != nil {
		return result, e
	}
	if o.ExpectedQWAC != nil {
		if e := AssertFirefox(result, "positive", o.ExpectedQWAC); e != nil {
			return result, e
		}
	}
	if o.Visible {
		fmt.Println("Firefox is open with its native identity panel. Inspect the signed owner and EU qualified text.")
		fmt.Println("Close this demo browser or press Ctrl-C here to stop its browser and local services.")
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop.Done():
				return result, nil
			case <-done:
				return result, fmt.Errorf("geckodriver stopped while the demo was open")
			case <-ticker.C:
				var handles []string
				if err = d.command("GET", "/window/handles", nil, &handles); err != nil || len(handles) == 0 {
					return result, nil
				}
			}
		}
	}
	return result, nil
}

// AssertFirefox uses the actual native identity panel and Firefox transport
// security state. Negative cases require the intended failure, not a timeout.
func AssertFirefox(r FirefoxResult, expect string, qwac *x509.Certificate) error {
	if r.AcceptInsecureCerts {
		return fmt.Errorf("Firefox TLS validation disabled")
	}
	if strings.HasPrefix(expect, "tls-") {
		want := map[string]string{"tls-untrusted": "SEC_ERROR_UNKNOWN_ISSUER", "tls-hostname": "SSL_ERROR_BAD_CERT_DOMAIN", "tls-expiry": "SEC_ERROR_EXPIRED_CERTIFICATE"}[expect]
		if want != "" && r.TLSFailure == want {
			return nil
		}
		return fmt.Errorf("expected %s, got TLS failure %q", expect, r.TLSFailure)
	}
	if r.NavigationError != "" || r.TLSFailure != "" || r.UI.TLSFingerprint == "" || r.UI.CertificateOverridden {
		return fmt.Errorf("real HTTPS was not verified: %+v", r)
	}
	reasons := map[string]string{"no-qwac-trust": "signing certificate not 2-QWAC", "signature": "invalid signature", "wrong-tls": "TLS binding does not cover server certificate", "qwac-hostname": "signing certificate not 2-QWAC", "qwac-profile": "signing certificate not 2-QWAC", "expired-binding": "header has expired", "critical-header": "header contains invalid parameter"}
	if reason := reasons[expect]; reason != "" {
		if r.QWACFailure != reason {
			return fmt.Errorf("expected native QWAC error %q, got %q", reason, r.QWACFailure)
		}
		expect = "qwac-negative"
	}
	if expect == "qwac-negative" {
		if r.UI.Connection == "secure" && !r.UI.QualifiedVisible && r.UI.QWACFingerprint == "" {
			return nil
		}
		return fmt.Errorf("unexpected qualified state: %+v", r.UI)
	}
	if expect != "positive" || qwac == nil {
		return fmt.Errorf("unknown Firefox expectation %q", expect)
	}
	if r.QWACFailure != "" {
		return fmt.Errorf("native Firefox QWAC verification logged a failure: %s", r.QWACFailure)
	}
	sum := sha256.Sum256(qwac.Raw)
	if r.UI.Connection != "secure-etsi" || !r.UI.QualifiedVisible || r.UI.QualifiedText == "" || len(qwac.Subject.Organization) == 0 || r.UI.Organization != qwac.Subject.Organization[0] || len(qwac.Subject.Country) == 0 || r.UI.Country != qwac.Subject.Country[0] || strings.ToLower(strings.ReplaceAll(r.UI.QWACFingerprint, ":", "")) != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("native Firefox qualified certificate/owner not observed: %+v", r.UI)
	}
	return nil
}
