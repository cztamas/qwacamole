package qwacamole

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"
	"net"
	"testing"
)

type fallback struct{}

func (fallback) IssuerKey() string { return "fallback" }
func (fallback) Issue(context.Context, *x509.CertificateRequest) (*certmagic.IssuedCertificate, error) {
	panic("fallback must never be called")
}

func TestHandshakeGateSurvivesCaddyEventFiltering(t *testing.T) {
	blocked := errors.New("binding startup refused")
	i := &Issuer{startupErr: blocked}
	observed := 0
	cfg := &certmagic.Config{
		Storage: &certmagic.FileStorage{Path: t.TempDir()},
		Issuers: []certmagic.Issuer{i}, Logger: zap.NewNop(),
		ShouldEmitFunc: func(event string) bool { return event == "existing_observer" },
		OnEvent:        func(context.Context, string, map[string]any) error { observed++; return nil },
	}
	i.SetConfig(cfg)
	if !cfg.ShouldEmitFunc("cached_managed_cert") || !cfg.ShouldEmitFunc("existing_observer") || cfg.ShouldEmitFunc("unobserved") {
		t.Fatal("binding events or prior event interests lost")
	}
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	cert, err := cfg.GetCertificateWithContext(context.Background(), &tls.ClientHelloInfo{
		Conn: server, ServerName: "qwac.example.com", SupportedProtos: []string{"h2"},
	})
	if cert != nil || !errors.Is(err, blocked) || observed != 1 {
		t.Fatalf("handshake bypassed abortable binding event: certificate=%v err=%v observed=%d", cert, err, observed)
	}
}
func TestFallbackCannotBypassBindingGate(t *testing.T) {
	i := &Issuer{}
	cfg := &certmagic.Config{Storage: &certmagic.FileStorage{Path: t.TempDir()}, Issuers: []certmagic.Issuer{i, fallback{}}}
	i.SetConfig(cfg)
	if len(cfg.Issuers) != 1 || cfg.Issuers[0] != i {
		t.Fatal("fallback retained")
	}
	if e := i.PreCheck(context.Background(), []string{"qwac.example.com"}, false); e == nil {
		t.Fatal("fallback config accepted")
	}
}
