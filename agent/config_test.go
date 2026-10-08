package agent

import (
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
