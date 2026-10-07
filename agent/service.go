package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cztamas/qwacamole/bindinghttp"
	"github.com/cztamas/qwacamole/lifecycle"
	"github.com/cztamas/qwacamole/store"
)

// Status distinguishes confirmed TLS activation from binding publication and
// includes independent deployment errors; Certbot's exit code is insufficient.
type Status struct {
	Ready      bool         `json:"ready"`
	Deployment Journal      `json:"deployment"`
	Binding    store.Status `json:"binding"`
}

func (a *Agent) Status() (Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, e := a.Manager.Status()
	return Status{Ready: a.ready.Load() && bindinghttp.Available(a.Manager), Deployment: a.journal, Binding: s}, e
}

// PublicHandler exposes binding bytes and a bodyless gate only. Control,
// diagnostics, certificate files and private keys are never served here.
func (a *Agent) PublicHandler() http.Handler {
	docs := bindinghttp.Handler(a.Manager)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/bindings/") {
			docs.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/gate" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			if a.ready.Load() && bindinghttp.Available(a.Manager) {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}
		http.NotFound(w, r)
	})
}
func (a *Agent) controlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.requestMu.Lock()
		if a.stopping {
			a.requestMu.Unlock()
			http.Error(w, "agent stopping", 503)
			return
		}
		a.requests.Add(1)
		a.requestMu.Unlock()
		defer a.requests.Done()

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/status" && r.Method == http.MethodGet {
			status, e := a.Status()
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		// No paths, commands or credentials are accepted from the control client.
		b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
		if e != nil || len(b) != 0 {
			http.Error(w, "request body must be empty", 400)
			return
		}
		switch r.URL.Path {
		case "/deploy":
			e = a.Deploy(r.Context())
		case "/reconcile":
			e = a.Reconcile(r.Context())
		case "/replace-qwac":
			e = a.ReplaceCredentials()
		default:
			http.NotFound(w, r)
			return
		}
		if e != nil {
			w.WriteHeader(503)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
}

// Serve binds separate loopback delivery and private local control listeners.
// Reconciliation errors leave the service alive but unavailable, allowing an
// operator to diagnose, repair and retry without bypassing the gate.
func (a *Agent) Serve(ctx context.Context, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}
	socket := a.Config.Socket()
	if st, e := os.Lstat(socket); e == nil {
		if st.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("refusing to remove non-socket control path")
		}
		if e = os.Remove(socket); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	control, e := net.Listen("unix", socket)
	if e != nil {
		return e
	}
	defer control.Close()
	defer os.Remove(socket)
	if e = os.Chmod(socket, 0600); e != nil {
		return e
	}
	public, e := net.Listen("tcp", a.Config.PublicListen)
	if e != nil {
		return e
	}
	defer public.Close()
	pub := &http.Server{Handler: a.PublicHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	ctl := &http.Server{Handler: a.controlHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 5 * time.Second}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Go(func() {
		lifecycle.Run(child, a.Manager, func(e lifecycle.Event) {
			if e.Err != nil {
				log.Error("binding maintenance", "event", e.Kind, "error", e.Err, "retry_after", e.RetryAfter, "binding", e.Status)
			} else {
				log.Info("binding maintenance", "event", e.Kind, "collection", e.Collection, "binding", e.Status)
			}
		})
	})
	errors := make(chan error, 2)
	go func() { errors <- pub.Serve(public) }()
	go func() { errors <- ctl.Serve(control) }()
	if e = a.Reconcile(child); e != nil {
		log.Error("deployment reconciliation failed; gate remains closed", "error", e)
	}
	select {
	case <-ctx.Done():
		e = nil
	case e = <-errors:
	}
	a.ready.Store(false)
	a.requestMu.Lock()
	a.stopping = true
	a.requestMu.Unlock()
	cancel()
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = ctl.Shutdown(shutdown)
	_ = pub.Shutdown(shutdown)
	_ = ctl.Close()
	_ = pub.Close()
	wg.Wait()
	a.requests.Wait()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}

// Control calls only the local configured socket; permissions authorize the
// caller. No deployment function is exposed by the public TCP listener.
func Control(ctx context.Context, c Config, command string) (json.RawMessage, error) {
	method, path := http.MethodPost, "/"+command
	if command == "status" {
		method = http.MethodGet
	}
	switch command {
	case "deploy", "reconcile", "replace-qwac", "status":
	default:
		return nil, fmt.Errorf("unknown local control operation")
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.Socket())
	}}
	defer tr.CloseIdleConnections()
	req, e := http.NewRequestWithContext(ctx, method, "http://local"+path, nil)
	if e != nil {
		return nil, e
	}
	resp, e := (&http.Client{Transport: tr, Timeout: 2 * time.Minute}).Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != 200 {
		return b, fmt.Errorf("agent %s: HTTP %d: %s", command, resp.StatusCode, b)
	}
	return b, nil
}
