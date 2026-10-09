package main

import (
	"bufio"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cztamas/qwacamole/devtools/internal/pki"
	"github.com/cztamas/qwacamole/store"
)

// This real-server check expires a synthetic supplied credential. It requires no
// production fault hooks and repairs the same configured files without restart.
func pluginExpiryCheck(c *pluginCheck, configPath, pkidir string, roots *x509.CertPool, arrived <-chan string, release chan struct{}) error {
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	leafPath := filepath.Join(pkidir, "qwac-leaf.pem")
	prior, err := os.ReadFile(leafPath)
	if err != nil {
		return err
	}
	defer store.Atomic(leafPath, prior)
	old, err := pki.ReadCert(leafPath)
	if err != nil {
		return err
	}
	key, err := pki.ReadKey(filepath.Join(pkidir, "qwac-leaf-key.pem"))
	if err != nil {
		return err
	}
	ca, err := pki.ReadCert(filepath.Join(pkidir, "qwac-intermediate.pem"))
	if err != nil {
		return err
	}
	caKey, err := pki.ReadKey(filepath.Join(pkidir, "qwac-intermediate-key.pem"))
	if err != nil {
		return err
	}
	template := *old
	template.SerialNumber = pki.Serial()
	template.NotAfter = time.Now().Add(8 * time.Second)
	template.ExtraExtensions = old.Extensions
	short, err := pki.Issue(&template, ca, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if !short.PublicKey.(*rsa.PublicKey).Equal(old.PublicKey) {
		return fmt.Errorf("short expiry changed key")
	}
	if err = pki.WriteCert(leafPath, short); err != nil {
		return err
	}
	if _, err = c.command("short-qwac-reload", "reload", "--config", configPath, "--force"); err != nil {
		return err
	}
	if err = c.ready(); err != nil {
		return err
	}
	state, err := c.state()
	if err != nil {
		return err
	}
	if state.Expires > short.NotAfter.Unix() {
		return fmt.Errorf("signed lifetime exceeded short credential")
	}
	client := *c.client
	client.Timeout = 15 * time.Second
	codes := make(chan int, 1)
	failures := make(chan error, 2)
	go func() {
		res, err := client.Get("https://qwac.example.com:8443/delayed")
		if err != nil {
			failures <- err
			return
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			failures <- err
			return
		}
		if strings.Contains(string(body), "application-first") {
			failures <- fmt.Errorf("protected body leaked after expiry")
			return
		}
		codes <- res.StatusCode
	}()
	conn, err := tls.Dial("tcp", "127.0.0.1:8443", &tls.Config{RootCAs: roots, ServerName: "qwac.example.com"})
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if _, err = fmt.Fprint(conn, "GET /delayed-ws HTTP/1.1\r\nHost: qwac.example.com:8443\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"); err != nil {
		return err
	}
	for range 2 {
		select {
		case <-arrived:
		case err := <-failures:
			return err
		case <-time.After(5 * time.Second):
			return fmt.Errorf("delayed origin request did not arrive before expiry")
		}
	}
	// Maintenance may extend the first observed binding up to the credential's
	// hard deadline. Wait past that deadline, not a superseded document's exp.
	for time.Now().Unix() <= short.NotAfter.Unix() {
		time.Sleep(50 * time.Millisecond)
	}
	unblock()
	select {
	case code := <-codes:
		if code != 503 {
			return fmt.Errorf("delayed Caddy response escaped expiry: %d", code)
		}
	case err := <-failures:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("delayed Caddy response did not finish")
	}
	upgrade, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return err
	}
	defer upgrade.Body.Close()
	if upgrade.StatusCode != 503 || upgrade.Header.Get("Upgrade") != "" || upgrade.Header.Get("Link") != "" {
		return fmt.Errorf("expired Caddy upgrade escaped gate: %d %v", upgrade.StatusCode, upgrade.Header)
	}
	fresh, err := tls.Dial("tcp", "127.0.0.1:8443", &tls.Config{RootCAs: roots, ServerName: "qwac.example.com"})
	if err == nil {
		fresh.Close()
		return fmt.Errorf("expired binding allowed fresh ordinary handshake")
	}
	log, err := os.ReadFile(filepath.Join(c.dir, "evidence/caddy.log"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(log), "TLS activation requires a current verified binding") {
		return fmt.Errorf("handshake failed without intended binding diagnosis")
	}
	c.pass("actual QWAC expiry rejects delayed response, WebSocket upgrade and fresh handshake")
	if err = store.Atomic(leafPath, prior); err != nil {
		return err
	}
	if _, err = c.command("repaired-qwac-reload", "reload", "--config", configPath, "--force"); err != nil {
		return err
	}
	if err = c.ready(); err != nil {
		return err
	}
	fresh, err = tls.Dial("tcp", "127.0.0.1:8443", &tls.Config{RootCAs: roots, ServerName: "qwac.example.com"})
	if err != nil {
		return err
	}
	fresh.Close()
	c.pass("restored QWAC inputs recover verified Caddy service without process restart")
	return nil
}
