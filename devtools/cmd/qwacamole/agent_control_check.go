package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/agent"
)

func checkAgentControlBoundary(ctx context.Context, cfg agent.Config) error {
	for path, mode := range map[string]os.FileMode{cfg.RuntimeDir: 0700, cfg.Socket(): 0600} {
		info, e := os.Lstat(path)
		if e != nil || info.Mode().Perm() != mode {
			return fmt.Errorf("private runtime permissions differ at %s: %v", path, e)
		}
	}
	public := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{"/status", "/deploy", "/replace-qwac", "/generations/privkey.pem", "/__qwac_control"} {
		r, e := public.Post("http://"+cfg.PublicListen+path, "application/json", nil)
		if e != nil {
			return e
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 404 {
			return fmt.Errorf("public deployment/private-file path exposed: %s %d", path, r.StatusCode)
		}
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", cfg.Socket())
	}}
	defer transport.CloseIdleConnections()
	control := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, test := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/deploy", "", 405},
		{"POST", "/deploy", `{"path":"/untrusted"}`, 400},
		{"POST", "/execute", "", 404},
	} {
		req, e := http.NewRequestWithContext(ctx, test.method, "http://local"+test.path, strings.NewReader(test.body))
		if e != nil {
			return e
		}
		r, e := control.Do(req)
		if e != nil {
			return e
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != test.code {
			return fmt.Errorf("private control request accepted: %s %s %d", test.method, test.path, r.StatusCode)
		}
	}
	return nil
}
