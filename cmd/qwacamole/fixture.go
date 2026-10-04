package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"github.com/cztamas/qwacamole/internal/binding"
	"github.com/cztamas/qwacamole/internal/pki"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func alteredQWAC(m *pki.Material, dir, mode string) (*pki.Material, error) {
	t := *m.Chain[0]
	t.SerialNumber = pki.Serial()
	t.ExtraExtensions = nil
	for _, e := range t.Extensions {
		if e.Id.Equal(pki.QCStatements) {
			t.ExtraExtensions = append(t.ExtraExtensions, e)
		}
	}
	if mode == "qwac-hostname" {
		t.DNSNames = []string{"other.example.com"}
	}
	if mode == "qwac-profile" {
		t.UnknownExtKeyUsage = nil
		t.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	k, e := pki.ReadKey(filepath.Join(dir, "pki/qwac-intermediate-key.pem"))
	if e != nil {
		return nil, e
	}
	leaf, e := pki.Issue(&t, m.Chain[1], &m.Key.PublicKey, k)
	if e != nil {
		return nil, e
	}
	return &pki.Material{Chain: []*x509.Certificate{leaf, m.Chain[1], m.Chain[2]}, Key: m.Key}, nil
}
func fixture(dir string) error {
	m, e := pki.Load(filepath.Join(dir, "pki"))
	if e != nil {
		return e
	}
	leaf, e := pki.ReadCert(filepath.Join(dir, "pki/tls-leaf.pem"))
	if e != nil {
		return e
	}
	key, e := pki.ReadKey(filepath.Join(dir, "pki/tls-leaf-key.pem"))
	if e != nil {
		return e
	}
	docs := map[string]string{}
	for _, mode := range []string{"positive", "signature", "wrong-tls", "qwac-hostname", "expired-binding", "qwac-profile"} {
		mat := m
		if strings.HasPrefix(mode, "qwac-") {
			mat, e = alteredQWAC(m, dir, mode)
			if e != nil {
				return e
			}
		}
		bound := leaf
		if mode == "wrong-tls" {
			bound = m.Chain[1]
		}
		doc, h, e := binding.Sign(mat, []*x509.Certificate{bound}, time.Now(), time.Hour)
		if e != nil {
			return e
		}
		if mode == "expired-binding" {
			h.Exp = time.Now().Add(-time.Minute).Unix()
			h.Iat = time.Now().Add(-2 * time.Minute).Unix()
			doc, _, e = binding.SignHeader(mat, h)
			if e != nil {
				return e
			}
		}
		if mode == "signature" {
			parts := strings.Split(doc, ".")
			b, _ := base64.RawURLEncoding.DecodeString(parts[2])
			b[0] ^= 1
			parts[2] = base64.RawURLEncoding.EncodeToString(b)
			doc = strings.Join(parts, ".")
		}
		docs[mode] = doc
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/bindings/", func(w http.ResponseWriter, r *http.Request) {
		doc, ok := docs[strings.TrimPrefix(r.URL.Path, "/bindings/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/jose")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, doc)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mode := strings.TrimPrefix(r.URL.Path, "/")
		if mode == "" {
			mode = "positive"
		}
		w.Header().Set("Cache-Control", "no-store")
		if _, ok := docs[mode]; ok {
			w.Header().Set("Link", fmt.Sprintf(`</bindings/%s>; rel="tls-certificate-binding"`, mode))
		}
		fmt.Fprint(w, "<!doctype html><title>Qwacamole TEST</title><h1>Synthetic QWAC fixture</h1><p>Server state only; browser verification is captured independently in NetLog.</p>")
	})
	root, e := pki.ReadCert(filepath.Join(dir, "pki/tls-root.pem"))
	if e != nil {
		return e
	}
	rk, e := pki.ReadKey(filepath.Join(dir, "pki/tls-root-key.pem"))
	if e != nil {
		return e
	}
	invalid := map[string]*x509.Certificate{}
	for _, mode := range []string{"tls-hostname", "tls-expiry"} {
		t := pki.Template("qwac.example.com", false)
		t.DNSNames = []string{"qwac.example.com"}
		t.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		if mode == "tls-hostname" {
			t.DNSNames = []string{"other.example.com"}
		}
		if mode == "tls-expiry" {
			t.NotBefore = time.Now().Add(-2 * time.Hour)
			t.NotAfter = time.Now().Add(-time.Hour)
		}
		c, e := pki.Issue(t, root, &key.PublicKey, rk)
		if e != nil {
			return e
		}
		invalid[mode] = c
	}
	// Separate ports retain exactly one local hostname while testing bad TLS.
	for port, mode := range map[string]string{"9444": "tls-hostname", "9445": "tls-expiry"} {
		c := invalid[mode]
		srv := &http.Server{Addr: "127.0.0.1:" + port, Handler: mux, TLSConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{c.Raw}, PrivateKey: key}}}}
		go func() {
			if e := srv.ListenAndServeTLS("", ""); e != nil {
				fmt.Println(e)
			}
		}()
	}
	srv := &http.Server{Addr: "127.0.0.1:9443", Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	return srv.ListenAndServeTLS(filepath.Join(dir, "pki/tls-leaf.pem"), filepath.Join(dir, "pki/tls-leaf-key.pem"))
}
