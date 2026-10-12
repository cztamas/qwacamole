package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{Host: "example.test", StateDir: "/var/lib/qwac", RuntimeDir: "/run/qwac", ChainFiles: []string{"/etc/qwac/chain.pem"}, KeyFile: "/etc/qwac/key.pem", TrustFile: "/etc/qwac/root.pem", CertbotLineage: "/etc/letsencrypt/live/example.test", PublicListen: "127.0.0.1:18081", HTTPListen: "0.0.0.0:80", HTTPSListen: "0.0.0.0:443", ProbeAddress: "127.0.0.1:443", Application: "http://127.0.0.1:8080", Webroot: "/var/lib/acme", NginxBinary: "/usr/sbin/nginx", NginxPrefix: "/var/lib/nginx", NginxConfig: "/etc/nginx/nginx.conf"}
}
func TestConfigurationRejectsUnsupportedAndInjectedValues(t *testing.T) {
	for _, mutate := range []func(*Config){func(c *Config) { c.Host = "*.example.test" }, func(c *Config) { c.PublicListen = "0.0.0.0:18081" }, func(c *Config) { c.ProbeAddress = "198.51.100.1:443" }, func(c *Config) { c.StateDir = "relative" }, func(c *Config) { c.StateDir = "/tmp/state\"; include /unmanaged;" }, func(c *Config) { c.RuntimeDir = "/" + strings.Repeat("a", 101) }, func(c *Config) { c.Application = "http://user:password@localhost" }, func(c *Config) { c.Lifetime = "2s" }} {
		c := validConfig()
		mutate(&c)
		if e := c.Validate(); e == nil {
			t.Fatal("unsupported config accepted", c)
		}
	}
	if e := validConfig().Validate(); e != nil {
		t.Fatal(e)
	}
}
func TestDurationValuesDoNotCollide(t *testing.T) {
	c := validConfig()
	c.Lifetime = "6s"
	c.RefreshBefore = "2s"
	c.DocumentGrace = "2s"
	s, e := c.StoreConfig()
	if e != nil {
		t.Fatal(e)
	}
	if s.Lifetime != 6*time.Second || s.RefreshBefore != 2*time.Second || s.DocumentGrace != 2*time.Second {
		t.Fatal(s)
	}
}
func TestNginxTemplateHasNoMovingCertbotTLSPaths(t *testing.T) {
	c := validConfig()
	gen := filepath.Join(c.StateDir, "generations", strings.Repeat("a", 64))
	s := c.httpsConfig(gen)
	if strings.Contains(s, c.CertbotLineage) || !strings.Contains(s, gen) || !strings.Contains(s, "auth_request /__qwac_gate;") || !strings.Contains(s, "internal;") {
		t.Fatal(s)
	}
	if strings.Contains(RenderNginx(c), "ssl_certificate") {
		t.Fatal("bootstrap activated HTTPS")
	}
}

func TestServingSettingsAreTypedAndBounded(t *testing.T) {
	zero := int64(0)
	big := int64(2 << 20)
	bad := int64(-1)
	c := validConfig()
	for _, s := range []ServingConfig{{}, {MaxBodyBytes: &zero}, {MaxBodyBytes: &big, ClientBodyTimeout: "1500ms", UpstreamSendTimeout: "10s", UpstreamReadTimeout: "24h", RequestBuffering: "off", HTTPSPort: 8443, HSTS: &HSTSConfig{MaxAgeSeconds: 0}}} {
		c.Serving = s
		if e := c.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, s := range []ServingConfig{{MaxBodyBytes: &bad}, {ClientBodyTimeout: "0s"}, {UpstreamReadTimeout: "25h"}, {UpstreamSendTimeout: "1us"}, {RequestBuffering: "on; include bad"}, {HTTPSPort: 65536}, {HSTS: &HSTSConfig{MaxAgeSeconds: -1}}} {
		c.Serving = s
		if e := c.Validate(); e == nil {
			t.Fatal("accepted", s)
		}
	}
	c = validConfig()
	if c.PublicAuthority() != "example.test" {
		t.Fatal(c.PublicAuthority())
	}
	c.Serving.HTTPSPort = 8443
	if c.PublicAuthority() != "example.test:8443" {
		t.Fatal(c.PublicAuthority())
	}
}

func TestPublicFirefoxJSONRemainsAnExplicitBoolean(t *testing.T) {
	for _, value := range []string{"true", "false", `"true"`} {
		c := validConfig()
		c.FirefoxCompatibility = false
		encoded, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		var fields map[string]json.RawMessage
		if e = json.Unmarshal(encoded, &fields); e != nil {
			t.Fatal(e)
		}
		fields["firefox_compatibility"] = json.RawMessage(value)
		encoded, e = json.Marshal(fields)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "agent.json")
		if e = os.WriteFile(path, encoded, 0600); e != nil {
			t.Fatal(e)
		}
		actual, e := LoadConfig(path)
		if value == `"true"` {
			if e == nil {
				t.Fatal("string instead of JSON boolean accepted")
			}
		} else if e != nil || actual.FirefoxCompatibility != (value == "true") {
			t.Fatal(actual, e)
		}
	}
}
