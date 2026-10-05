package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func lifecycle(dir string) error {
	root, e := pki.ReadCert(filepath.Join(dir, "pki/pebble-root.pem"))
	if e != nil {
		return e
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "qwac.example.com"}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:8443")
	}, ForceAttemptHTTP2: false}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	get := func() (*http.Response, []byte, error) {
		r, e := client.Get("https://qwac.example.com:8443/state")
		if e != nil {
			return nil, nil, e
		}
		d, e := io.ReadAll(r.Body)
		r.Body.Close()
		return r, d, e
	}
	old, d, e := get()
	if e != nil {
		return e
	}
	oldCert := old.TLS.PeerCertificates[0]
	_ = os.WriteFile(filepath.Join(dir, "evidence/rotation-before.json"), d, 0600)
	start := time.Now()
	fmt.Printf("Holding existing HTTP/1 TLS connection, certificate %s\n", binding.ID(oldCert))
	for time.Since(start) < 5*time.Minute {
		time.Sleep(time.Second)
		r, d, e := get()
		if e != nil {
			return e
		}
		if !r.TLS.PeerCertificates[0].Equal(oldCert) {
			return fmt.Errorf("held connection reconnected; overlap evidence invalid")
		}
		var state struct {
			URL string   `json:"url"`
			IDs []string `json:"tls_ids"`
		}
		if e = json.Unmarshal(d, &state); e != nil {
			return e
		}
		hasOld, hasNew := false, false
		for _, id := range state.IDs {
			if id == binding.ID(oldCert) {
				hasOld = true
			} else {
				hasNew = true
			}
		}
		if !hasOld || !hasNew {
			continue
		}
		doc, e := client.Get("https://qwac.example.com:8443" + state.URL)
		if e != nil {
			return e
		}
		b, e := io.ReadAll(doc.Body)
		doc.Body.Close()
		if e != nil {
			return e
		}
		qr, e := pki.ReadCert(filepath.Join(dir, "pki/qwac-root.pem"))
		if e != nil {
			return e
		}
		if e = binding.Verify(string(b), "qwac.example.com", []*x509.Certificate{oldCert}, qr, time.Now()); e != nil {
			return e
		}
		fresh, e := tls.Dial("tcp", "127.0.0.1:8443", &tls.Config{RootCAs: roots, ServerName: "qwac.example.com"})
		if e != nil {
			return e
		}
		newCert := fresh.ConnectionState().PeerCertificates[0]
		fresh.Close()
		if oldCert.Equal(newCert) {
			continue
		}
		if e = binding.Verify(string(b), "qwac.example.com", []*x509.Certificate{oldCert, newCert}, qr, time.Now()); e != nil {
			return e
		}
		report := map[string]any{"old_tls_id": binding.ID(oldCert), "new_tls_id": binding.ID(newCert), "elapsed_seconds": time.Since(start).Seconds(), "existing_connection_tls_id": binding.ID(r.TLS.PeerCertificates[0]), "binding_url": state.URL, "old_and_new_bound": true}
		_ = os.WriteFile(filepath.Join(dir, "evidence/rotation-after.json"), d, 0600)
		if e = writeJSON(filepath.Join(dir, "evidence/rotation.json"), report); e != nil {
			return e
		}
		fmt.Println("Observed automatic TLS rotation with old connection retained and overlapping binding.")
		return nil
	}
	return fmt.Errorf("automatic rotation not observed within five minutes")
}
