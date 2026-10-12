package qwacamole

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func TestOperationalJSONDurationsAndCaddyfileSettings(t *testing.T) {
	for _, raw := range []string{
		`{"lifetime":"45s","refresh_before":"10s","document_grace":"24h","max_certificates":12,"max_document_bytes":65536}`,
		`{"lifetime":45000000000,"refresh_before":10000000000,"document_grace":86400000000000,"max_certificates":12,"max_document_bytes":65536}`,
	} {
		var b BindingConfig
		if e := json.Unmarshal([]byte(raw), &b); e != nil {
			t.Fatal(e)
		}
		if b.Lifetime != 45*time.Second || b.RefreshBefore != 10*time.Second || b.DocumentGrace != 24*time.Hour || b.MaxCertificates != 12 || b.MaxDocumentBytes != 65536 {
			t.Fatal("JSON settings changed", b)
		}
	}
	var b BindingConfig
	if e := json.Unmarshal([]byte(`{"refresh_befor":"1s"}`), &b); e == nil {
		t.Fatal("unknown policy field accepted")
	}
	d := caddyfile.NewTestDispenser(`qwac_binding {
	hostname qwac.example.com
	directory /tmp/qwac-policy-test
	chain_files /tmp/chain.pem
	key_file /tmp/key.pem
	trust_files /tmp/root.pem
	lifetime 1h
	refresh_before 10m
	document_grace 48h
	max_certificates 32
	max_document_bytes 65536
}`)
	h := new(Handler)
	if e := h.UnmarshalCaddyfile(d); e != nil {
		t.Fatal(e)
	}
	if h.Binding.RefreshBefore != 10*time.Minute || h.Binding.DocumentGrace != 48*time.Hour || h.Binding.MaxCertificates != 32 || h.Binding.MaxDocumentBytes != 65536 {
		t.Fatal("Caddyfile policy not preserved", h.Binding)
	}
}

func TestFirefoxPublicCaddyfileSwitch(t *testing.T) {
	for _, tc := range []struct {
		option        string
		valid, compat bool
	}{{"", true, false}, {"firefox_compatibility", true, true}, {"firefox_compatibility true", false, false}, {"firefox_compatibility false", false, false}} {
		t.Run(tc.option, func(t *testing.T) {
			i := new(Issuer)
			e := i.UnmarshalCaddyfile(caddyfile.NewTestDispenser(`qwac {
 hostname example.test
 directory /tmp/public-example
 chain_files /tmp/chain.pem
 key_file /tmp/key.pem
 trust_files /tmp/authority.pem
 ` + tc.option + "\n}"))
			if (e == nil) != tc.valid {
				t.Fatal(e)
			}
			if e == nil && i.Binding.FirefoxCompatibility != tc.compat {
				t.Fatal("compatibility default changed")
			}
		})
	}
}

func TestConventionalStateDirectory(t *testing.T) {
	b, e := (BindingConfig{ChainFiles: []string{"/chain.pem"}, KeyFile: "/key.pem", TrustFiles: []string{"/authority.pem"}}).normalized()
	if e == nil {
		t.Fatal("missing hostname accepted", b)
	}
	b.Host = "example.test"
	b, e = b.normalized()
	if e != nil || b.Dir == "" {
		t.Fatal(b, e)
	}
	if b.Lifetime != time.Hour || b.FirefoxCompatibility {
		t.Fatal("defaults changed", b)
	}
}
