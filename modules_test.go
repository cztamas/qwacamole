package qwacamole

import (
	"context"
	"crypto/x509"
	"github.com/caddyserver/certmagic"
	"testing"
)

type fallback struct{}

func (fallback) IssuerKey() string { return "fallback" }
func (fallback) Issue(context.Context, *x509.CertificateRequest) (*certmagic.IssuedCertificate, error) {
	panic("fallback must never be called")
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
