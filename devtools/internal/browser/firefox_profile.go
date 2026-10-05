package browser

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

func DetectFirefox() (string, error) {
	const path = "/Applications/Firefox.app/Contents/MacOS/firefox"
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("install stock Firefox 153 or newer, or pass -browser /absolute/path/to/firefox")
}

func PrepareFirefox(profile, certutil string, tlsRoot *x509.Certificate, tlsTrust, qwacTrust bool) error {
	if _, err := os.Stat(profile); err == nil {
		return fmt.Errorf("profile already exists: %s", profile)
	}
	profile, err := filepath.Abs(profile)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	run := func(args ...string) error {
		out, err := exec.Command(certutil, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("NSS certutil: %w: %s", err, out)
		}
		return nil
	}
	if err = run("-N", "--empty-password", "-d", "sql:"+profile); err != nil {
		return err
	}
	if tlsTrust {
		path := filepath.Join(profile, "pebble-tls-root.pem")
		if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsRoot.Raw}), 0600); err != nil {
			return err
		}
		if err = run("-A", "-d", "sql:"+profile, "-n", "Qwacamole Pebble TLS TEST", "-t", "C,,", "-a", "-i", path); err != nil {
			return err
		}
	}
	// QWAC roots are not imported into NSS: stock Firefox recognizes Mozilla's
	// separate compiled test authority set when this profile pref is enabled.
	prefs := map[string]any{
		// Pin the stock identity-panel UI; Firefox 157's new Trust Panel has a
		// broken showSubView call in this build. This does not change verification.
		"browser.urlbar.trustPanel.featureGate":                 false,
		"security.qwacs.enabled":                                true,
		"security.qwacs.enable_test_trust_anchors":              qwacTrust,
		"security.enterprise_roots.enabled":                     false,
		"security.certerrors.mitm.auto_enable_enterprise_roots": false,
		"network.dns.localDomains":                              "qwac.example.com",
		"network.trr.mode":                                      5,
		"network.proxy.type":                                    0,
		"browser.cache.disk.enable":                             false,
		"browser.cache.memory.enable":                           false,
		"browser.startup.homepage":                              "about:blank",
		"browser.startup.homepage_override.mstone":              "ignore",
		"browser.shell.checkDefaultBrowser":                     false,
		"browser.aboutwelcome.enabled":                          false,
		"datareporting.policy.dataSubmissionEnabled":            false,
		"toolkit.telemetry.enabled":                             false,
	}
	names := make([]string, 0, len(prefs))
	for name := range prefs {
		names = append(names, name)
	}
	sort.Strings(names)
	text := "// Generated only for this isolated Qwacamole test profile.\n"
	for _, name := range names {
		key, _ := json.Marshal(name)
		value, _ := json.Marshal(prefs[name])
		text += fmt.Sprintf("user_pref(%s, %s);\n", key, value)
	}
	return os.WriteFile(filepath.Join(profile, "user.js"), []byte(text), 0600)
}
