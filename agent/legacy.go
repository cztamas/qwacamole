package agent

import (
	"fmt"
	"path/filepath"
)

// Exact merged-main format, retained exclusively for offline verification.
func legacyRenderNginx(c Config) string {
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
func (c Config) legacyHTTPSConfig(generation string) string {
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
