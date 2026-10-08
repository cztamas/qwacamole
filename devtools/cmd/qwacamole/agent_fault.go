package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

// Fault injection lives only in devtools. The installed production executable
// has no fault flags, file triggers or synthetic issuance path.
type faultNginx struct {
	agent.Nginx
	dir string
}

func (f faultNginx) Reload(ctx context.Context) error {
	for _, name := range []string{"reload-once", "skip-reload-once"} {
		path := filepath.Join(f.dir, name)
		if e := os.Remove(path); e == nil {
			if name == "skip-reload-once" {
				return nil
			}
			return fmt.Errorf("injected Nginx reload failure")
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	return f.Nginx.Reload(ctx)
}

func agentFaultService(config, dir string) error {
	cfg, e := agent.LoadConfig(config)
	if e != nil {
		return e
	}
	if e = (agent.Nginx{Config: cfg}).Prerequisites(context.Background()); e != nil {
		return e
	}
	fail := func(name string) bool { _, e := os.Stat(filepath.Join(dir, name)); return e == nil }
	ops := store.Operations{
		Sign: func(m *credentials.Credentials, cs []*x509.Certificate, now time.Time, lifetime time.Duration, newCertificate bool) (string, binding.Header, error) {
			if fail("sign-all") || (newCertificate && fail("sign-new")) {
				return "", binding.Header{}, fmt.Errorf("injected binding signing failure")
			}
			if cfg.FirefoxCompatibility {
				return binding.SignFirefox(m, cs, now, lifetime)
			}
			return binding.Sign(m, cs, now, lifetime)
		},
		WriteDocument: func(path string, d []byte) error {
			if fail("document") {
				return fmt.Errorf("injected binding document storage failure")
			}
			return store.Atomic(path, d)
		},
		CommitManifest: func(path string, d []byte, _ bool) error {
			if fail("manifest") {
				return fmt.Errorf("injected binding manifest storage failure")
			}
			return store.Atomic(path, d)
		},
	}
	a, e := agent.Open(cfg, faultNginx{Nginx: agent.Nginx{Config: cfg}, dir: dir}, ops)
	if e != nil {
		return e
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Serve(ctx, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
}
