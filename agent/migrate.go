package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/store"
)

type migrationIntent struct {
	Version int    `json:"migration_version"`
	Backup  string `json:"backup"`
	Desired Config `json:"desired"`
}

// Migrate updates only owned configuration formats and serving policy. It never
// reloads Nginx, selects a certificate, signs a binding or discards history.
// The flag is an operator assertion, not an automated proof of drained workers.
func Migrate(ctx context.Context, c Config, fullStop bool) error {
	if !fullStop {
		return fmt.Errorf("migrate requires -full-stop-confirmed after stopping renewal automation, agent, Nginx master AND every old worker/held connection; no drain is inferred from ports, expiry or reload")
	}
	if e := c.Validate(); e != nil {
		return e
	}
	if e := checkPersistedVersions(c.StateDir); e != nil {
		return e
	}
	if e := validateEnabled(c); e != nil {
		return e
	}
	if _, e := readFormat(c.StateDir); e != nil {
		return e
	}
	st, e := os.Lstat(c.StateDir)
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode()&0077 != 0 {
		return fmt.Errorf("migration requires a private real state directory")
	}
	lock, e := lockState(filepath.Join(c.StateDir, ".agent.lock"))
	if e != nil {
		return e
	}
	defer lock.Close()
	ctl, e := lockState(filepath.Join(c.runtimeDir(), ".control.lock"))
	if e != nil {
		return e
	}
	defer ctl.Close()
	intentPath := filepath.Join(c.StateDir, "migration.json")
	var intent migrationIntent
	data, e := os.ReadFile(intentPath)
	if e == nil {
		if e = decodeStrict(data, &intent); e != nil {
			return e
		}
		if intent.Version != 1 || !strings.HasPrefix(intent.Backup, ".migration-backup-") || filepath.Base(intent.Backup) != intent.Backup || !reflect.DeepEqual(intent.Desired, canonicalConfig(c)) {
			return fmt.Errorf("interrupted migration requires original target config and supported migration_version")
		}
	} else if os.IsNotExist(e) {
		backup, e := os.MkdirTemp(c.StateDir, ".migration-backup-")
		if e != nil {
			return e
		}
		if e = copyState(c.StateDir, backup); e != nil {
			return e
		}
		top, e := os.ReadFile(c.NginxConfig)
		if e != nil {
			return e
		}
		if e = store.Atomic(filepath.Join(backup, "top-level.conf"), top); e != nil {
			return e
		}
		intent = migrationIntent{1, filepath.Base(backup), canonicalConfig(c)}
		// Validate original ownership and coverage before recording any activation intent.
		if _, _, e = migrationSource(c, backup); e != nil {
			return e
		}
		data, _ = json.Marshal(intent)
		if e = store.Atomic(intentPath, data); e != nil {
			return e
		}
	} else {
		return e
	}
	backup := filepath.Join(c.StateDir, intent.Backup)
	_, j, e := migrationSource(c, backup)
	if e != nil {
		return e
	}
	stage, e := os.MkdirTemp(c.StateDir, ".migration-stage-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	if e = os.Mkdir(filepath.Join(stage, "enabled"), 0700); e != nil {
		return e
	}
	generations, e := os.ReadDir(filepath.Join(backup, "generations"))
	if e != nil {
		return e
	}
	if e = verifyMigrationPayloads(c, backup); e != nil {
		return e
	}
	updates := map[string][]byte{}
	for _, entry := range generations {
		if !generationID.MatchString(entry.Name()) {
			continue
		} // ignored incomplete staging is preserved in backup
		id := entry.Name()
		rendered := []byte(c.httpsConfig(filepath.Join(c.StateDir, "generations", id)))
		updates[filepath.Join(c.StateDir, "generations", id, "https.conf")] = rendered
		stageGen := filepath.Join(stage, "generations", id)
		if e = os.MkdirAll(stageGen, 0700); e != nil {
			return e
		}
		if e = store.Atomic(filepath.Join(stageGen, "https.conf"), rendered); e != nil {
			return e
		}
	}
	selected, e := selectedFor(c)
	if e != nil {
		return e
	}
	if selected != "" {
		if e = os.Symlink(filepath.Join(stage, "generations", selected, "https.conf"), filepath.Join(stage, "enabled", "site.conf")); e != nil {
			return e
		}
	}
	stageCfg := c
	stageCfg.StateDir = stage
	stageCfg.NginxConfig = filepath.Join(stage, "nginx.conf")
	if e = store.Atomic(stageCfg.NginxConfig, []byte(RenderNginx(stageCfg))); e != nil {
		return e
	}
	if e = (Nginx{Config: stageCfg}).Validate(ctx); e != nil {
		return fmt.Errorf("migration staged Nginx validation failed; backup %s; repeat migrate after repair: %w", backup, e)
	}
	j.Version = StateVersion
	journal, _ := json.MarshalIndent(j, "", "  ")
	config, _ := json.Marshal(canonicalConfig(c))
	updates[filepath.Join(c.StateDir, "deployment.json")] = journal
	updates[filepath.Join(c.StateDir, "deployment-config.json")] = config
	updates[c.NginxConfig] = []byte(RenderNginx(c))
	// While files are replaced, migration.json blocks startup. A retry uses only
	// the durable original backup, even after partial activation of configuration.
	for path, data := range updates {
		if e = ctx.Err(); e != nil {
			return e
		}
		if e = store.Atomic(path, data); e != nil {
			return e
		}
	}
	format, _ := json.Marshal(stateFormat{StateVersion, NginxFormatVersion})
	if e = store.Atomic(filepath.Join(c.StateDir, "format.json"), format); e != nil {
		return e
	}
	if e = (Nginx{Config: c}).Validate(ctx); e != nil {
		return e
	}
	if e = os.Remove(intentPath); e != nil {
		return e
	}
	return syncDir(c.StateDir)
}
func selectedFor(c Config) (string, error) { return (&Agent{Config: c}).selected() }

func migrationSource(c Config, backup string) (Config, Journal, error) {
	var old Config
	var j Journal
	fail := func(e error) (Config, Journal, error) { return old, j, e }
	data, e := os.ReadFile(filepath.Join(backup, "deployment-config.json"))
	if e != nil {
		return fail(e)
	}
	if e = decodeStrict(data, &old); e != nil {
		return fail(e)
	}
	if e = old.Validate(); e != nil {
		return fail(e)
	}
	// Serving metadata/settings do not affect certificate identity, binding
	// coverage, ownership or recovery. Other changes need a separate procedure.
	if !compatibleConfiguration(old, c) {
		return fail(fmt.Errorf("migration supports format and serving changes only; ownership, credentials paths and binding policy must stay identical"))
	}
	format, e := readFormat(backup)
	if e != nil {
		return fail(e)
	}
	top, e := os.ReadFile(filepath.Join(backup, "top-level.conf"))
	if e != nil {
		return fail(e)
	}
	expected := RenderNginx(old)
	if format.State == 0 {
		expected = legacyRenderNginx(old)
	}
	if string(top) != expected {
		return fail(fmt.Errorf("unmanaged or corrupt source top-level Nginx config"))
	}
	data, e = os.ReadFile(filepath.Join(backup, "deployment.json"))
	if os.IsNotExist(e) {
		j = Journal{Phase: "empty"}
	} else if e != nil {
		return fail(e)
	} else if e = decodeStrict(data, &j); e != nil {
		return fail(e)
	}
	if j.Version != format.State {
		return fail(fmt.Errorf("journal state_version disagrees with state format"))
	}
	if e = validateJournal(j); e != nil {
		return fail(e)
	}
	mat, e := c.Material()
	if e != nil {
		return fail(e)
	}
	cfg, e := c.StoreConfig()
	if e != nil {
		return fail(e)
	}
	cfg.Dir = filepath.Join(backup, "bindings")
	m, e := store.Recover(cfg, mat, store.Operations{})
	if e != nil {
		return fail(e)
	}
	covered := map[string]bool{}
	for _, id := range m.Snapshot().IDs {
		covered[id] = true
	}
	dirs, e := os.ReadDir(filepath.Join(backup, "generations"))
	if e != nil {
		return fail(e)
	}
	for _, entry := range dirs {
		if !generationID.MatchString(entry.Name()) {
			continue
		}
		id := entry.Name()
		dir := filepath.Join(backup, "generations", id)
		if !entry.IsDir() {
			return fail(fmt.Errorf("unmanaged generation %s", id))
		}
		rendered, e := os.ReadFile(filepath.Join(dir, "https.conf"))
		if e != nil {
			return fail(e)
		}
		want := old.httpsConfig(filepath.Join(old.StateDir, "generations", id))
		if format.State == 0 {
			want = old.legacyHTTPSConfig(filepath.Join(old.StateDir, "generations", id))
		}
		if string(rendered) != want {
			return fail(fmt.Errorf("unmanaged or corrupt generation configuration %s", id))
		}
		chain, e := os.ReadFile(filepath.Join(dir, "fullchain.pem"))
		if e != nil {
			return fail(e)
		}
		key, e := os.ReadFile(filepath.Join(dir, "privkey.pem"))
		if e != nil {
			return fail(e)
		}
		pair, e := tls.X509KeyPair(chain, key)
		if e != nil {
			return fail(e)
		}
		leaf, e := x509.ParseCertificate(pair.Certificate[0])
		if e != nil {
			return fail(e)
		}
		roots, e := old.roots()
		if e != nil {
			return fail(e)
		}
		intermediates := x509.NewCertPool()
		for _, der := range pair.Certificate[1:] {
			cert, e := x509.ParseCertificate(der)
			if e != nil {
				return fail(e)
			}
			intermediates.AddCert(cert)
		}
		// Historical chain verification preserves expired retained generations; it
		// cannot authorize activation. Reconcile/probe/deploy use current time.
		_, e = leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, DNSName: old.Host, CurrentTime: leaf.NotBefore.Add(time.Second), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		if e != nil {
			return fail(e)
		}
		if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != old.Host || len(leaf.IPAddresses) != 0 || leaf.IsCA || binding.ID(leaf) != id || !covered[id] {
			return fail(fmt.Errorf("generation identity or retained binding coverage mismatch %s", id))
		}
	}
	// Validate every selected include, preserving the exact pointer and journal
	// meaning, including an interrupted deployment requiring reconciliation.
	selected, e := selectedFor(c)
	if e != nil {
		return fail(e)
	}
	if selected != "" && !covered[selected] {
		return fail(fmt.Errorf("selected TLS certificate is unbound"))
	}
	if j.Phase == "empty" && selected != "" {
		return fail(fmt.Errorf("selected TLS without journal"))
	}
	return old, j, nil
}
func validateJournal(j Journal) error {
	switch j.Phase {
	case "empty", "prepared", "bound", "activating", "reloading", "confirming", "rolling_back", "rolled_back", "committed", "needs_deploy":
	default:
		return fmt.Errorf("unsupported deployment journal phase")
	}
	if j.Phase == "committed" && (j.Active == "" || j.Active != j.Candidate) {
		return fmt.Errorf("inconsistent committed deployment journal")
	}
	for _, id := range []string{j.Active, j.Previous, j.Candidate} {
		if id != "" && !generationID.MatchString(id) {
			return fmt.Errorf("invalid journal generation")
		}
	}
	return nil
}
func copyState(source, dest string) error {
	directories := []string{dest}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == source {
			return nil
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		if strings.HasPrefix(entry.Name(), ".migration-") || entry.Name() == ".agent.lock" || entry.Name() == ".control.lock" || entry.Name() == "control.sock" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			if e := os.Mkdir(target, 0700); e != nil {
				return e
			}
			directories = append(directories, target)
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if rel != filepath.Join("enabled", "site.conf") {
				return fmt.Errorf("unmanaged state symlink %s", rel)
			}
			link, e := os.Readlink(path)
			if e != nil {
				return e
			}
			return os.Symlink(link, target)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported state file %s", rel)
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return store.Atomic(target, data)
	})
	if err != nil {
		return err
	}
	// File fsync alone cannot persist a newly created directory's entry in its
	// parent. Sync the complete backup tree bottom-up before durable intent.
	for i := len(directories) - 1; i >= 0; i-- {
		if e := syncDir(directories[i]); e != nil {
			return e
		}
	}
	return nil
}

// A retry may replace only configuration files. Payload changes since the
// snapshot require operator repair, so stale backup validation cannot bless a
// different TLS key/certificate or a different binding manifest.
func verifyMigrationPayloads(c Config, backup string) error {
	want, wantErr := os.Readlink(filepath.Join(backup, "enabled", "site.conf"))
	got, gotErr := os.Readlink(filepath.Join(c.StateDir, "enabled", "site.conf"))
	if !(os.IsNotExist(wantErr) && os.IsNotExist(gotErr)) && (wantErr != nil || gotErr != nil || want != got) {
		return fmt.Errorf("selected generation changed since migration backup; restore consistent state before retry")
	}

	for _, name := range []string{"bindings", "generations"} {
		root := filepath.Join(backup, name)
		if e := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || entry.Name() == "https.conf" {
				return nil
			}
			rel, e := filepath.Rel(backup, path)
			if e != nil {
				return e
			}
			want, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			got, e := os.ReadFile(filepath.Join(c.StateDir, rel))
			if e != nil {
				return e
			}
			if !bytes.Equal(want, got) {
				return fmt.Errorf("migration payload changed since backup: %s; restore consistent state before retry", rel)
			}
			return nil
		}); e != nil {
			return e
		}
	}
	return nil
}
