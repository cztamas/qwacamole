package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/agent"
)

func nginxHTTPPolicyCheck(c *pluginCheck, cfg agent.Config, ready bool) error {
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	token := filepath.Join(cfg.Webroot, ".well-known/acme-challenge/policy-test")
	if e := os.MkdirAll(filepath.Dir(token), 0755); e != nil {
		return e
	}
	if e := os.WriteFile(token, []byte("challenge-stays-available"), 0644); e != nil {
		return e
	}
	for _, test := range []struct {
		path, host string
		code       int
	}{
		{"/.well-known/acme-challenge/policy-test", cfg.Host, 200},
		{"/a%2Fb?q=%2F", cfg.Host, func() int {
			if !cfg.Serving.RedirectHTTPS {
				return 404
			}
			if !ready {
				return 503
			}
			return 308
		}()},
		{"//evil.invalid/a?x=y", "evil.invalid", 421},
		{"/a", cfg.Host + ":1234", 421},
	} {
		req, _ := http.NewRequest("GET", "http://"+cfg.HTTPListen+test.path, nil)
		req.Host = test.host
		req.Header.Set("X-Forwarded-Host", "evil.invalid")
		req.Header.Set("Forwarded", "host=evil.invalid;proto=http")
		var resp *http.Response
		var e error
		for end := time.Now().Add(3 * time.Second); ; {
			resp, e = client.Do(req)
			if e == nil || time.Now().After(end) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if e != nil {
			return e
		}
		data, e := io.ReadAll(resp.Body)
		resp.Body.Close()
		if e != nil || resp.StatusCode != test.code || resp.Header.Get("Strict-Transport-Security") != "" {
			return fmt.Errorf("HTTP policy %s: %d %v %s %v", test.path, resp.StatusCode, resp.Header, data, e)
		}
		if resp.StatusCode == 308 && resp.Header.Get("Location") != "https://"+cfg.PublicAuthority()+test.path {
			return fmt.Errorf("redirect destination: %v", resp.Header)
		}
		if resp.StatusCode == 200 && string(data) != "challenge-stays-available" {
			return fmt.Errorf("challenge rewritten")
		}
	}
	c.pass(fmt.Sprintf("HTTP challenge, Host rejection, fixed-authority redirect and HTTPS-only HSTS (confirmed=%v)", ready))
	return nil
}
func nginxServingCheck(c *pluginCheck, cfg agent.Config, client *http.Client) error {
	// A distinct browser-side loopback address proves the agent does not replace
	// Nginx's trusted client address with the 127.0.0.1 local delivery hop.
	inspectionTransport := client.Transport.(*http.Transport).Clone()
	inspectionTransport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.2")}}).DialContext(ctx, network, cfg.ProbeAddress)
	}
	defer inspectionTransport.CloseIdleConnections()
	inspectionClient := *client
	inspectionClient.Transport = inspectionTransport
	evidence := []map[string]any{}
	for _, forged := range []bool{false, true} {
		path := "/metadata/a%2Fb?x=%2F&x=two;three"
		req, _ := http.NewRequest("GET", "https://"+cfg.PublicAuthority()+path, nil)
		if forged {
			for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
				req.Header.Set(key, "evil.invalid")
			}
		}
		resp, e := inspectionClient.Do(req)
		if e != nil {
			return e
		}
		var meta map[string]string
		e = json.NewDecoder(resp.Body).Decode(&meta)
		resp.Body.Close()
		evidence = append(evidence, map[string]any{"forged": forged, "status": resp.StatusCode, "origin": meta, "headers": resp.Header})
		if e != nil || resp.StatusCode != 200 || meta["host"] != cfg.PublicAuthority() || meta["forwarded"] != "" || meta["xff"] != "127.0.0.2" || meta["xfh"] != cfg.PublicAuthority() || meta["xfp"] != "https" || meta["uri"] != path || meta["url"] != "https://"+cfg.PublicAuthority()+"/done" || !strings.Contains(resp.Header.Get("Set-Cookie"), "Secure") {
			return fmt.Errorf("origin trusted metadata mismatch: %v %v %v", meta, resp.Header, e)
		}
		wantHSTS := ""
		if cfg.Serving.HSTS != nil {
			wantHSTS = fmt.Sprintf("max-age=%d", cfg.Serving.HSTS.MaxAgeSeconds)
			if cfg.Serving.HSTS.IncludeSubdomains {
				wantHSTS += "; includeSubDomains"
			}
		}
		if resp.Header.Get("Strict-Transport-Security") != wantHSTS {
			return fmt.Errorf("HSTS policy %v", resp.Header)
		}
	}
	for _, host := range []string{"evil.invalid", cfg.Host + ":1234", strings.ReplaceAll(cfg.Host, ".", "x") + ":8443"} {
		req, _ := http.NewRequest("GET", "https://"+cfg.PublicAuthority()+"/metadata", nil)
		req.Host = host
		resp, e := client.Do(req)
		if e != nil {
			return e
		}
		resp.Body.Close()
		if resp.StatusCode != 421 || resp.Header.Get("Location") != "" {
			return fmt.Errorf("unsupported authority admitted %s: %d", host, resp.StatusCode)
		}
	}
	resp, e := client.Get("https://" + cfg.PublicAuthority() + "/metadata-url")
	if e != nil {
		return e
	}
	resp.Body.Close()
	if resp.StatusCode != 307 || resp.Header.Get("Location") != "https://"+cfg.PublicAuthority()+"/done" {
		return fmt.Errorf("origin HTTPS redirect changed %v", resp.Header)
	}
	if e = writeJSON(filepath.Join(c.dir, "evidence/nginx-serving-metadata.json"), evidence); e != nil {
		return e
	}
	c.pass("Nginx replaces forged forwarding metadata, preserves external port/encoded URL, origin secure cookies and absolute redirects; unsupported Host rejected")
	if e = nginxHTTPPolicyCheck(c, cfg, true); e != nil {
		return e
	}
	uploadSize := 2 << 20
	want := 413
	if cfg.Serving.MaxBodyBytes != nil {
		want = 200
	}
	resp, e = client.Post("https://"+cfg.PublicAuthority()+"/upload", "application/octet-stream", bytes.NewReader(bytes.Repeat([]byte("b"), uploadSize)))
	if e != nil {
		return e
	}
	raw, e := io.ReadAll(resp.Body)
	resp.Body.Close()
	if e != nil || resp.StatusCode != want {
		return fmt.Errorf("body limit status=%d want=%d %s %v", resp.StatusCode, want, raw, e)
	}
	if want == 200 {
		var value struct {
			Bytes int `json:"bytes"`
		}
		if json.Unmarshal(raw, &value) != nil || value.Bytes != uploadSize {
			return fmt.Errorf("larger upload corrupted %s", raw)
		}
	}
	c.pass(fmt.Sprintf("configured body limit: 2 MiB upload status %d", want))
	if cfg.Serving.ClientBodyTimeout != "" {
		conn, e := tls.Dial("tcp", cfg.ProbeAddress, &tls.Config{ServerName: cfg.Host, RootCAs: client.Transport.(*http.Transport).TLSClientConfig.RootCAs})
		if e != nil {
			return e
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprintf(conn, "POST /upload HTTP/1.1\r\nHost: %s\r\nContent-Length: 1024\r\n\r\nx", cfg.PublicAuthority())
		startBody := time.Now()
		response, e := http.ReadResponse(bufio.NewReader(conn), nil)
		if e != nil {
			access, readErr := os.ReadFile(filepath.Join(cfg.NginxPrefix, "access.log"))
			if e != io.ErrUnexpectedEOF || readErr != nil || !strings.Contains(string(access), `"POST /upload HTTP/1.1" 408 0`) || time.Since(startBody) < 1400*time.Millisecond || time.Since(startBody) > 3*time.Second {
				return fmt.Errorf("body inactivity timeout not established: %v elapsed %s", e, time.Since(startBody))
			}
		} else {
			response.Body.Close()
			if response.StatusCode != 408 {
				return fmt.Errorf("body timeout status %d", response.StatusCode)
			}
		}
		c.pass("configured client-body inactivity timeout: Nginx records 408 and closes incomplete unbuffered upload")
		long := *client
		long.Timeout = 22 * time.Second
		start := time.Now()
		resp, e = long.Get("https://" + cfg.PublicAuthority() + "/timeout-read")
		if e != nil {
			return e
		}
		resp.Body.Close()
		if resp.StatusCode != 504 || time.Since(start) < 14*time.Second {
			return fmt.Errorf("read timeout %d elapsed %s", resp.StatusCode, time.Since(start))
		}
		c.pass("configured upstream-read inactivity timeout returns 504")
		start = time.Now()
		resp, e = long.Post("https://"+cfg.PublicAuthority()+"/timeout-send", "application/octet-stream", bytes.NewReader(bytes.Repeat([]byte("s"), 64<<20)))
		if e != nil {
			return e
		}
		resp.Body.Close()
		if resp.StatusCode != 504 || time.Since(start) > 10*time.Second {
			return fmt.Errorf("send timeout %d elapsed %s", resp.StatusCode, time.Since(start))
		}
		c.pass("configured upstream-send inactivity timeout under origin backpressure returns 504")
	}
	return nil
}
