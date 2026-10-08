package store

import (
	"testing"
	"time"
)

func TestOperationalPolicyValidationAndDefaults(t *testing.T) {
	c, e := (Config{}).WithDefaults()
	if e != nil || c.Lifetime != time.Hour || c.RefreshBefore != 20*time.Minute || c.DocumentGrace != 24*time.Hour || c.MaxCertificates != 256 || c.MaxDocumentBytes != 1<<20 {
		t.Fatal("unexpected deployment defaults", c, e)
	}
	for _, bad := range []Config{
		{Lifetime: time.Second}, {Lifetime: 3 * time.Second, RefreshBefore: 2 * time.Second},
		{RefreshBefore: -time.Second}, {DocumentGrace: -time.Second},
		{MaxCertificates: -1}, {MaxDocumentBytes: 512},
	} {
		if _, e := bad.WithDefaults(); e == nil {
			t.Fatal("invalid policy accepted", bad)
		}
	}
	for _, lifetime := range []time.Duration{3 * time.Second, 45 * time.Second, time.Hour} {
		cfg, e := (Config{Lifetime: lifetime}).WithDefaults()
		if e != nil {
			t.Fatal(e)
		}
		m := &Manager{cfg: cfg, current: State{Published: 100, Expires: 100 + int64(lifetime/time.Second)}}
		want := time.Unix(m.current.Expires, 0).Add(-lifetime / 3)
		if !m.NextRefresh().Equal(want) {
			t.Fatal("refresh did not use signed expiry", lifetime, m.NextRefresh(), want)
		}
		m.current.Expires = 103 // actual TLS/QWAC cap overrides long configured lifetime
		if !m.NextRefresh().Equal(time.Unix(102, 0)) {
			t.Fatal("refresh margin ignored effective lifetime")
		}
		m.current.Expires = 102
		if !m.NextRefresh().Equal(time.Unix(102, 0)) {
			t.Fatal("near-expiry signing would spin instead of waiting for expiry")
		}
	}
}
