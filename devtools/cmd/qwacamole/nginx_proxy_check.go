package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/store"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// The origin deliberately stops consuming for longer than the agent's former
// ten-second body deadline. A large direct request fills transport buffers;
// the Nginx request separately exercises its default buffered-upload policy.
func nginxProxyOrigin(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/ws":
		u := ws.HTTPUpgrader{Header: http.Header{"Link": {`</style>; rel="preload"`}}}
		conn, _, _, err := u.Upgrade(r, w)
		if err == nil {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			data, op, err := wsutil.ReadClientData(conn)
			if err == nil {
				_ = wsutil.WriteServerMessage(conn, op, data)
			}
		}
		return true
	case "/slow-body":
		start := time.Now()
		select {
		case <-time.After(12 * time.Second):
		case <-r.Context().Done():
			return true
		}
		h := sha256.New()
		n, err := io.Copy(h, r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"bytes": n, "sha256": fmt.Sprintf("%x", h.Sum(nil)), "elapsed_seconds": time.Since(start).Seconds(), "error": fmt.Sprint(err)})
		return true
	}
	return false
}

func nginxWebSocketCheck(c *pluginCheck, cfg agent.Config, tlsConfig *tls.Config) error {
	conn, err := tls.Dial("tcp", cfg.ProbeAddress, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err = fmt.Fprintf(conn, "GET /ws HTTP/1.1\r\nHost: %s:8443\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", cfg.Host); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		return fmt.Errorf("Nginx WebSocket handshake: %w", err)
	}
	links := strings.Join(resp.Header.Values("Link"), ",")
	if err = writeJSON(filepath.Join(c.dir, "evidence/nginx-websocket.json"), map[string]any{"status": resp.StatusCode, "headers": resp.Header}); err != nil {
		return err
	}
	if resp.StatusCode != 101 || resp.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" || !strings.Contains(links, store.DiscoveryURL) || !strings.Contains(links, "preload") {
		return fmt.Errorf("Nginx WebSocket upgrade failed: %d %v", resp.StatusCode, resp.Header)
	}
	if err = wsutil.WriteClientMessage(conn, ws.OpText, []byte("nginx-upgrade-echo")); err != nil {
		return err
	}
	echo, op, err := wsutil.ReadServerData(struct {
		io.Reader
		io.Writer
	}{reader, conn})
	if err != nil || op != ws.OpText || string(echo) != "nginx-upgrade-echo" {
		return fmt.Errorf("Nginx WebSocket traffic: %q %v", echo, err)
	}
	c.pass("real Nginx TLS WebSocket 101, discovery/upstream Link and bidirectional frame traffic")
	return nil
}

func nginxUploadCheck(c *pluginCheck, client *http.Client, name, url string, size int) error {
	body := bytes.Repeat([]byte("u"), size)
	want := sha256.Sum256(body)
	// Reuse real trust/routing, but allow the deliberate slow origin to finish.
	longClient := *client
	longClient.Timeout = 35 * time.Second
	start := time.Now()
	resp, err := longClient.Post(url, "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s upload: %w", name, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	_ = writeJSON(filepath.Join(c.dir, "evidence/"+name+"-upload.json"), map[string]any{"status": resp.StatusCode, "elapsed_seconds": time.Since(start).Seconds(), "expected_bytes": size, "expected_sha256": fmt.Sprintf("%x", want), "origin_response": string(raw), "read_error": fmt.Sprint(err)})
	var received struct {
		Bytes   int     `json:"bytes"`
		SHA256  string  `json:"sha256"`
		Elapsed float64 `json:"elapsed_seconds"`
		Error   string  `json:"error"`
	}
	if err != nil || json.Unmarshal(raw, &received) != nil || resp.StatusCode != 200 || received.Bytes != size || received.SHA256 != fmt.Sprintf("%x", want) || received.Elapsed < 12 || received.Error != "<nil>" {
		return fmt.Errorf("%s slow-origin upload failed: status=%d body=%s error=%v", name, resp.StatusCode, raw, err)
	}
	c.pass(name + " upload preserves complete body/hash with origin backpressure beyond ten seconds")
	return nil
}
