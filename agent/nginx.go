package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Target contains only the local deployment boundaries exercised by Nginx and
// reconciliation tests. Requests cannot supply commands or process arguments.
type Target interface {
	Validate(context.Context) error
	Reload(context.Context) error
	Probe(context.Context) (*x509.Certificate, error)
}

type Nginx struct{ Config Config }

func (n Nginx) command(ctx context.Context, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	all := []string{"-p", n.Config.NginxPrefix, "-c", n.Config.NginxConfig}
	all = append(all, args...)
	out, e := exec.CommandContext(ctx, n.Config.NginxBinary, all...).CombinedOutput()
	if e != nil {
		return fmt.Errorf("nginx %v: %w: %s", args, e, out)
	}
	return nil
}

// Prerequisites detects missing auth_request/TLS support and rejects a modified
// top-level config, which could otherwise bypass the agent-controlled paths.
func (n Nginx) Prerequisites(ctx context.Context) error {
	out, e := exec.CommandContext(ctx, n.Config.NginxBinary, "-V").CombinedOutput()
	if e != nil {
		return e
	}
	for _, flag := range []string{"--with-http_auth_request_module", "--with-http_ssl_module"} {
		if !strings.Contains(string(out), flag) {
			return fmt.Errorf("nginx requires %s", flag)
		}
	}
	d, e := os.ReadFile(n.Config.NginxConfig)
	if e != nil {
		return e
	}
	if string(d) != RenderNginx(n.Config) {
		return fmt.Errorf("nginx config differs from render-nginx output; unmanaged routes/includes are unsupported")
	}
	return nil
}
func (n Nginx) Validate(ctx context.Context) error {
	if e := n.Prerequisites(ctx); e != nil {
		return e
	}
	return n.command(ctx, "-t")
}
func (n Nginx) Reload(ctx context.Context) error { return n.command(ctx, "-s", "reload") }

// Probe always opens a new normally verified connection with the configured SNI.
func (n Nginx) Probe(ctx context.Context) (*x509.Certificate, error) {
	roots, e := n.Config.roots()
	if e != nil {
		return nil, e
	}
	d := tls.Dialer{NetDialer: &net.Dialer{Timeout: 3 * time.Second}, Config: &tls.Config{ServerName: n.Config.Host, RootCAs: roots, MinVersion: tls.VersionTLS12}}
	conn, e := d.DialContext(ctx, "tcp", n.Config.ProbeAddress)
	if e != nil {
		return nil, e
	}
	defer conn.Close()
	return conn.(*tls.Conn).ConnectionState().PeerCertificates[0], nil
}

func quote(s string) string { return `"` + s + `"` }

// RenderNginx produces the complete supported config, initially HTTP-only. The
// agent selects the HTTPS include only after binding publication. auth_request
// failure (including agent unavailability) never permits an application request.
func RenderNginx(c Config) string {
	return fmt.Sprintf(`worker_processes 1;
pid %s;
error_log %s info;
events { worker_connections 1024; }
http {
  access_log %s;
  client_body_temp_path %s;
  default_type application/octet-stream;
  map $http_upgrade $qwac_connection_upgrade {
    default upgrade;
    '' close;
  }
  server {
    listen %s;
    server_name %s;
    location /.well-known/acme-challenge/ { root %s; }
    location / { return 404; }
  }
  include %s;
}
`, quote(filepath.Join(c.NginxPrefix, "nginx.pid")), quote(filepath.Join(c.NginxPrefix, "error.log")), quote(filepath.Join(c.NginxPrefix, "access.log")), quote(filepath.Join(c.NginxPrefix, "client_body_temp")), c.HTTPListen, c.Host, quote(c.Webroot), quote(filepath.Join(c.StateDir, "enabled", "*.conf")))
}
func (c Config) httpsConfig(generation string) string {
	return fmt.Sprintf(`server {
  listen %s ssl;
  server_name %s;
  ssl_certificate %s;
  ssl_certificate_key %s;
  ssl_session_cache off;
  ssl_session_tickets off;
  location ^~ /bindings/ {
    proxy_pass http://%s;
    add_header Cache-Control no-store always;
    proxy_buffering off;
    proxy_cache off;
  }
  location = /__qwac_gate {
    internal;
    proxy_pass http://%s/gate;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_cache off;
  }
  location ^~ /__qwac_control { return 404; }
  location / {
    auth_request /__qwac_gate;
    add_header Link '</bindings/current.jose>; rel="tls-certificate-binding"' always;
    proxy_pass http://%s/application/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $qwac_connection_upgrade;
    proxy_buffering off;
    proxy_intercept_errors off;
    proxy_set_header Host $host;
  }
}
`, c.HTTPSListen, c.Host, quote(filepath.Join(generation, "fullchain.pem")), quote(filepath.Join(generation, "privkey.pem")), c.PublicListen, c.PublicListen, c.PublicListen)
}
