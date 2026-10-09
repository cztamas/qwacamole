package coretest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/caddyserver/certmagic"
	"github.com/cztamas/qwacamole/binding"
	plugin "github.com/cztamas/qwacamole/caddy"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

func TestCaddyExpiredReverseProxyCancelsUnboundedResponse(t *testing.T) {
	// Provision a real Caddy configuration context for the proxy's events app.
	// ProvisionContext does not start an admin endpoint or application listeners.
	originalStorage := certmagic.Default.Storage
	base, err := caddy.ProvisionContext(&caddy.Config{})
	certmagic.Default.Storage = originalStorage
	if err != nil {
		t.Fatal(err)
	}
	for _, protocol := range []string{"http1", "http2"} {
		for _, body := range []string{"streaming", "stalled"} {
			t.Run(protocol+"/"+body, func(t *testing.T) {
				cfg, leaf := supplied(t)
				cfg.Lifetime = 3 * time.Second
				var fail atomic.Bool
				ops := store.Operations{Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
					if fail.Load() {
						return "", binding.Header{}, fmt.Errorf("signing outage")
					}
					return binding.Sign(m, cs, now, lifetime)
				}}
				ctx, cancel := caddy.NewContext(base)
				defer cancel()
				h := &plugin.Handler{Binding: cfg}
				if err := h.ProvisionWithOperations(ctx, ops); err != nil {
					t.Fatal(err)
				}
				defer h.Cleanup()
				lease, err := plugin.Acquire(ctx, cfg, ops)
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Release()
				if err = lease.Publish(leaf); err != nil {
					t.Fatal(err)
				}
				fail.Store(true)
				expires := lease.Manager().Snapshot().Expires

				arrived, release, canceled, stop := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(arrived)
					select {
					case <-release:
					case <-stop:
						return
					}
					w.Header().Set("Set-Cookie", "protected=secret")
					w.Header().Set("Link", "</protected>; rel=preload")
					w.Header().Set("X-Protected", "upstream-secret")
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
					tick := time.NewTicker(time.Millisecond)
					defer tick.Stop()
					for {
						select {
						case <-r.Context().Done():
							close(canceled)
							return // No EOF until the proxy cancels this request.
						case <-stop:
							return
						case <-tick.C:
							if body == "streaming" {
								_, _ = io.WriteString(w, "data: protected\n\n")
								w.(http.Flusher).Flush()
							}
						}
					}
				}))
				t.Cleanup(func() { close(stop); upstream.Close() })
				proxy := &reverseproxy.Handler{Upstreams: reverseproxy.UpstreamPool{&reverseproxy.Upstream{Dial: strings.TrimPrefix(upstream.URL, "http://")}}}
				if err = proxy.Provision(ctx); err != nil {
					t.Fatal(err)
				}
				defer proxy.Cleanup()
				served := make(chan error, 1)
				front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r = caddyhttp.PrepareRequest(r, caddy.NewReplacer(), w, &caddyhttp.Server{})
					err := h.ServeHTTP(w, r, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return proxy.ServeHTTP(w, r, caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil }))
					}))
					served <- err
				}))
				pkidir := filepath.Dir(cfg.KeyFile)
				key, err := pki.ReadKey(filepath.Join(pkidir, "tls-leaf-key.pem"))
				if err != nil {
					t.Fatal(err)
				}
				front.EnableHTTP2 = protocol == "http2"
				front.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leaf.Raw}, PrivateKey: key}}}
				front.StartTLS()
				t.Cleanup(func() { front.CloseClientConnections(); front.Close() })
				root, err := pki.ReadCert(filepath.Join(pkidir, "tls-root.pem"))
				if err != nil {
					t.Fatal(err)
				}
				roots := x509.NewCertPool()
				roots.AddCert(root)
				transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}, ForceAttemptHTTP2: protocol == "http2", DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, front.Listener.Addr().String())
				}}
				defer transport.CloseIdleConnections()
				requestCtx, cancelRequest := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancelRequest()
				req, err := http.NewRequestWithContext(requestCtx, "GET", "https://"+cfg.Host+"/stream", nil)
				if err != nil {
					t.Fatal(err)
				}
				type response struct {
					res  *http.Response
					body []byte
					err  error
				}
				completed := make(chan response, 1)
				go func() {
					r := response{}
					r.res, r.err = (&http.Client{Transport: transport}).Do(req)
					if r.err == nil {
						r.body, r.err = io.ReadAll(r.res.Body)
						r.res.Body.Close()
					}
					completed <- r
				}()
				select {
				case <-arrived:
					if time.Now().Unix() >= expires {
						t.Fatal("request did not reach upstream before binding expiry")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("request did not reach real upstream")
				}
				for time.Now().Unix() < expires {
					time.Sleep(10 * time.Millisecond)
				}
				close(release) // Headers become available only after signed expiry.
				select {
				case r := <-completed:
					if r.err != nil {
						t.Fatal("503 reset or failed to complete", r.err)
					}
					if r.res.StatusCode != 503 || len(r.body) != 0 || r.res.Header.Get("Set-Cookie") != "" || r.res.Header.Get("Link") != "" || r.res.Header.Get("X-Protected") != "" || r.res.Header.Get("Cache-Control") != "no-store" {
						t.Fatal("protected response escaped rejection", r.res.StatusCode, r.res.Header, string(r.body))
					}
					wantProto := 1
					if protocol == "http2" {
						wantProto = 2
					}
					if r.res.ProtoMajor != wantProto {
						t.Fatal("wrong client protocol", r.res.Proto)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("rejected unbounded response did not complete promptly")
				}
				select {
				case err := <-served:
					if err != nil {
						t.Fatal("proxy handler did not finish cleanly", err)
					}
				case <-time.After(time.Second):
					t.Fatal("proxy serving goroutine remained occupied")
				}
				select {
				case <-canceled:
				case <-time.After(time.Second):
					t.Fatal("rejected upstream was not canceled")
				}
			})
		}
	}
}

func TestCaddyResponseGatePreservesOtherPanics(t *testing.T) {
	cfg, leaf := supplied(t)
	cfg.Lifetime = 3 * time.Second
	var fail atomic.Bool
	ops := store.Operations{Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, _ bool) (string, binding.Header, error) {
		if fail.Load() {
			return "", binding.Header{}, fmt.Errorf("signing outage")
		}
		return binding.Sign(m, cs, now, lifetime)
	}}
	ctx, cancel := newContext()
	defer cancel()
	h := &plugin.Handler{Binding: cfg}
	if err := h.ProvisionWithOperations(ctx, ops); err != nil {
		t.Fatal(err)
	}
	defer h.Cleanup()
	lease, err := plugin.Acquire(ctx, cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	if err = lease.Publish(leaf); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	expires := lease.Manager().Snapshot().Expires
	assertPanic := func(want any, next caddyhttp.Handler) {
		t.Helper()
		var caught any
		func() {
			defer func() { caught = recover() }()
			_, _ = request(h, "/panic", "GET", next)
		}()
		if caught != want {
			t.Fatal("unrelated abort/panic suppressed", caught, want)
		}
	}
	assertPanic(http.ErrAbortHandler, caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error {
		panic(http.ErrAbortHandler) // A valid streaming response must still abort.
	}))
	assertPanic("unrelated failure", caddyhttp.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		for time.Now().Unix() < expires {
			time.Sleep(10 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK) // Our gate has rejected the response.
		panic("unrelated failure")   // Only ErrAbortHandler may be contained.
	}))
}
