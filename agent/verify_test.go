//go:build darwin || linux

package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestVerifyUsesCommittedBindingSubdirectoryAndPrivateReadiness(t *testing.T) {
	o := setupFixture(t)
	c := validConfig()
	c.ChainFiles = []string{o.Chain}
	c.KeyFile = o.Key
	c.TrustFile = o.Authority
	c.TLSRoots = o.Authority
	c.StateDir = t.TempDir()
	runtime, e := os.MkdirTemp("/tmp", "qwac-verify-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(runtime) })
	c.RuntimeDir = runtime
	key, e := credentials.ReadKey(o.Key)
	if e != nil {
		t.Fatal(e)
	}
	ca, e := credentials.ReadCert(o.Authority)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: c.Host}, DNSNames: []string{c.Host}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(20 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	leaf, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	material, e := c.Material()
	if e != nil {
		t.Fatal(e)
	}
	doc, _, e := binding.Sign(material, []*x509.Certificate{leaf}, time.Now(), time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	config, e := c.StoreConfig()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(config.Dir, 0700); e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256([]byte(doc))
	state := store.State{URL: "/bindings/" + hex.EncodeToString(hash[:]) + ".jose", Certs: []string{base64.StdEncoding.EncodeToString(der)}}
	data, _ := json.Marshal(state)
	if e = os.WriteFile(filepath.Join(config.Dir, "current.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Link", `</bindings/current.jose>; rel="tls-certificate-binding"`)
			return
		}
		w.Header().Set("Content-Type", "application/jose")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(doc))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	defer server.Close()
	c.ProbeAddress = server.Listener.Addr().String()
	var ready atomic.Bool
	ready.Store(true)
	listener, e := net.Listen("unix", c.Socket())
	if e != nil {
		t.Fatal(e)
	}
	control := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("unexpected control request %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Status{Ready: ready.Load()})
	})}
	go control.Serve(listener)
	defer control.Close()
	report, e := Verify(context.Background(), c)
	if e != nil || !report.Ready || len(report.Checks) != 5 {
		t.Fatal(report, e)
	}
	ready.Store(false)
	report, e = Verify(context.Background(), c)
	if e == nil || report.Ready || report.Checks[0].Name != "Nginx deployment" {
		t.Fatal(report, e)
	}
}
