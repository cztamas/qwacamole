package agent

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Nginx's auth_request runs before the upstream response exists. This proxy
// adds the response-commit check so a delayed upstream cannot send protected
// headers/body after the binding becomes unavailable. Existing streams may
// continue once their response has committed, as in the Caddy integration.
func applicationProxy(origin string, available func() bool) http.Handler {
	u, _ := url.Parse(origin) // Config.Validate has checked the single HTTP origin.
	proxy := &httputil.ReverseProxy{Rewrite: func(p *httputil.ProxyRequest) {
		p.Out.URL.Path = strings.TrimPrefix(p.Out.URL.Path, "/application")
		if p.Out.URL.RawPath != "" {
			p.Out.URL.RawPath = strings.TrimPrefix(p.Out.URL.RawPath, "/application")
		}
		p.SetURL(u)
		// Preserve the origin request target, including raw query ordering/escapes.
		p.Out.URL.RawQuery = p.In.URL.RawQuery
		p.Out.URL.ForceQuery = p.In.URL.ForceQuery
		p.Out.Host = p.In.Host
		// Rewrite removes forwarding headers before this callback. Nginx replaces
		// these values at ingress; local processes are part of the trusted boundary.
		// Do not derive HTTPS or the client IP from this loopback HTTP connection.
		for _, key := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
			if value := p.In.Header.Get(key); value != "" {
				p.Out.Header.Set(key, value)
			}
		}
	}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !available() {
			unavailableResponse(w)
			return
		}
		gate := &responseGate{ResponseWriter: w, available: available}
		requestProxy := *proxy
		requestProxy.ModifyResponse = func(response *http.Response) error {
			gate.upstream = response
			if gate.rejected {
				gate.closeUpstream()
			}
			return nil
		}
		requestProxy.ServeHTTP(gate, r)
	})
}

func unavailableResponse(w http.ResponseWriter) {
	for key := range w.Header() {
		w.Header().Del(key)
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "binding unavailable", http.StatusServiceUnavailable)
}

type responseGate struct {
	http.ResponseWriter
	available           func() bool
	committed, rejected bool
	upstream            *http.Response
}

func (w *responseGate) closeUpstream() {
	if w.upstream != nil {
		w.upstream.Body.Close()
		w.upstream.Body = http.NoBody
	}
}

func (w *responseGate) WriteHeader(code int) {
	if w.committed {
		return
	}
	if !w.available() {
		w.committed, w.rejected = true, true
		w.closeUpstream()
		unavailableResponse(w.ResponseWriter)
		return
	}
	if code >= 200 || code == http.StatusSwitchingProtocols {
		w.committed = true
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *responseGate) Write(b []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	if w.rejected {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
func (w *responseGate) FlushError() error {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *responseGate) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if !w.available() {
		if !w.committed {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		return nil, nil, fmt.Errorf("binding unavailable at protocol upgrade")
	}
	w.committed = true
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func (w *responseGate) Unwrap() http.ResponseWriter { return w.ResponseWriter }
