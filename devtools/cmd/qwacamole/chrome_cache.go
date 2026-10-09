package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/store"
)

// Reuse the native identity/trust verifier for Caddy's real intermediary cache.
// HTTP observations prove stable content/discovery; browser events prove trust
// and the selected replacement identity, independently of our binding verifier.
type chromeCache struct {
	native *nginxNative
	roots  *x509.CertPool
	body   []byte
	links  string
}

func newChromeCache(dir, binary, profile string, tlsRoot, qwacRoot *x509.Certificate) (*chromeCache, error) {
	n := &nginxNative{mode: "chrome", server: "Caddy", binary: binary, dir: dir, profile: profile, root: qwacRoot}
	var err error
	n.trust, err = n.readTrust()
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(tlsRoot)
	return &chromeCache{native: n, roots: roots}, nil
}

func (c *chromeCache) check(name string, qwac *x509.Certificate) error {
	conn, err := tls.Dial("tcp", "127.0.0.1:8443", &tls.Config{RootCAs: c.roots, ServerName: "qwac.example.com"})
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err = fmt.Fprint(conn, "GET /cached HTTP/1.1\r\nHost: qwac.example.com:8443\r\nConnection: close\r\n\r\n"); err != nil {
		return err
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	links := strings.Join(res.Header.Values("Link"), ",")
	if res.StatusCode != 200 || !strings.Contains(links, store.DiscoveryURL) {
		return fmt.Errorf("cached Caddy content/discovery missing")
	}
	if c.body == nil {
		c.body, c.links = body, links
	} else if !bytes.Equal(body, c.body) || links != c.links || res.Header.Get("X-Qwac-Cache") != "hit" {
		return fmt.Errorf("cached Caddy content or discovery changed across lifecycle")
	}
	if err = c.native.check(name, qwac, ""); err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	return writeJSON(filepath.Join(c.native.dir, "evidence", "cache-"+name+".json"), map[string]any{"cache": res.Header.Get("X-Qwac-Cache"), "age": res.Header.Get("Age"), "link": links, "body_sha256": fmt.Sprintf("%x", sum)})
}
