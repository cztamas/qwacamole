package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cztamas/qwacamole/agent"
)

// Fault injection is exclusively in devtools. The configured fixed wrapper
// kills the migration process at staged Nginx validation, after durable intent.
func writeUpgradeWrapper(path, binary, dir string) error {
	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
 if [ "$arg" = '%s/nginx.conf' ] && [ -f '%s/interrupt-after-activation' ]; then
  rm '%s/interrupt-after-activation'
  kill -KILL "$PPID"
  exit 1
 fi
 case "$arg" in
  */.migration-stage-*/nginx.conf)
   if [ -f '%s/interrupt-migration' ]; then
    rm '%s/interrupt-migration'
    kill -KILL "$PPID"
    exit 1
   fi
   ;;
 esac
done
exec '%s' "$@"
`, dir, dir, dir, dir, dir, binary)
	return os.WriteFile(path, []byte(script), 0700)
}
func migrationInvariantFiles(cfg agent.Config) (map[string]string, error) {
	result := map[string]string{}
	for _, dir := range []string{filepath.Join(cfg.StateDir, "bindings"), filepath.Join(cfg.StateDir, "generations")} {
		e := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".atomic-") {
				return nil
			}
			if entry.Name() == "https.conf" {
				return nil
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			sum := sha256.Sum256(data)
			rel, _ := filepath.Rel(cfg.StateDir, path)
			result[rel] = fmt.Sprintf("%x", sum)
			return nil
		})
		if e != nil {
			return nil, e
		}
	}
	for _, path := range append(cfg.ChainFiles, cfg.KeyFile, cfg.TrustFile) {
		data, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		sum := sha256.Sum256(data)
		result[filepath.Base(path)] = fmt.Sprintf("%x", sum)
	}
	return result, nil
}
func nginxUpgradeCheck(c *pluginCheck, cfg agent.Config, cfgPath, binary string) error {
	before, e := migrationInvariantFiles(cfg)
	if e != nil {
		return e
	}
	beforeJSON, _ := json.Marshal(before)
	oldTop, e := os.ReadFile(cfg.NginxConfig)
	if e != nil {
		return e
	}
	oldJournal, e := os.ReadFile(filepath.Join(cfg.StateDir, "deployment.json"))
	if e != nil {
		return e
	}
	// Simulate a crash after fresh activation but before the journal commit.
	// Migration must retain that recoverable intent, not invent committed state.
	var interrupted agent.Journal
	if e = json.Unmarshal(oldJournal, &interrupted); e != nil {
		return e
	}
	interrupted.Phase = "confirming"
	interrupted.Previous = interrupted.Active
	oldJournal, e = json.Marshal(interrupted)
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(cfg.StateDir, "deployment.json"), oldJournal, 0600); e != nil {
		return e
	}

	if out, err := exec.Command(binary, "validate", "-config", cfgPath).CombinedOutput(); err == nil || !bytes.Contains(out, []byte("legacy unversioned")) {
		return fmt.Errorf("legacy startup not rejected: %s %v", out, err)
	}
	if out, err := exec.Command(binary, "migrate", "-config", cfgPath).CombinedOutput(); err == nil || !bytes.Contains(out, []byte("full-stop-confirmed")) {
		return fmt.Errorf("missing full-stop assertion accepted: %s %v", out, err)
	}
	c.pass("merged-main unversioned state explicitly identified; migration requires operator full-stop assertion")
	if e = os.WriteFile(filepath.Join(c.dir, "interrupt-migration"), nil, 0600); e != nil {
		return e
	}
	command := func() ([]byte, error) {
		return exec.Command(binary, "migrate", "-config", cfgPath, "-full-stop-confirmed").CombinedOutput()
	}
	out, err := command()
	if err == nil {
		return fmt.Errorf("migration interruption did not terminate process: %s", out)
	}
	top, _ := os.ReadFile(cfg.NginxConfig)
	journal, _ := os.ReadFile(filepath.Join(cfg.StateDir, "deployment.json"))
	if !bytes.Equal(top, oldTop) || !bytes.Equal(journal, oldJournal) {
		return fmt.Errorf("interruption before activation modified source")
	}
	if e = agent.CheckStateFormat(cfg); e == nil || !strings.Contains(e.Error(), "interrupted") {
		return fmt.Errorf("interrupted migration did not block startup: %v", e)
	}
	c.pass("killed migration retains durable restricted backup/source and blocks agent startup")
	if e = os.WriteFile(filepath.Join(c.dir, "interrupt-after-activation"), nil, 0600); e != nil {
		return e
	}
	out, err = command()
	if err == nil {
		return fmt.Errorf("activation interruption did not terminate migration: %s", out)
	}
	if e = agent.CheckStateFormat(cfg); e == nil || !strings.Contains(e.Error(), "interrupted") {
		return fmt.Errorf("partial migration startup not blocked: %v", e)
	}
	c.pass("killed migration after configuration activation still blocks startup and resumes from original backup")

	for attempt := 0; attempt < 2; attempt++ {
		out, err = command()
		if err != nil {
			return fmt.Errorf("migration retry %d: %s: %w", attempt, out, err)
		}
	}
	after, e := migrationInvariantFiles(cfg)
	if e != nil {
		return e
	}
	afterJSON, _ := json.Marshal(after)
	if !bytes.Equal(beforeJSON, afterJSON) {
		return fmt.Errorf("migration changed TLS/QWAC credentials or binding history")
	}
	var old, now agent.Journal
	json.Unmarshal(oldJournal, &old)
	data, _ := os.ReadFile(filepath.Join(cfg.StateDir, "deployment.json"))
	json.Unmarshal(data, &now)
	now.Version = old.Version
	if now != old {
		return fmt.Errorf("journal meaning changed during migration")
	}
	if e = agent.CheckStateFormat(cfg); e != nil {
		return e
	}
	dirs, e := os.ReadDir(cfg.StateDir)
	if e != nil {
		return e
	}
	backupCount := 0
	for _, dir := range dirs {
		if strings.HasPrefix(dir.Name(), ".migration-backup-") {
			st, e := os.Stat(filepath.Join(cfg.StateDir, dir.Name()))
			if e != nil || st.Mode().Perm() != 0700 {
				return fmt.Errorf("migration backup permissions")
			}
			backupCount++
		}
	}
	if backupCount == 0 {
		return fmt.Errorf("migration omitted retained backup")
	}
	if e = writeJSON(filepath.Join(c.dir, "evidence/nginx-migration.json"), map[string]any{"source": "4a93ce6c43bf4a728a69625ac7ed567d54db53f1", "interrupted": true, "repeated": true, "preserved_artifact_hashes": before, "retained_backups": backupCount, "journal": now}); e != nil {
		return e
	}
	c.pass("migration resume/repeat validates real Nginx, preserves journal meaning, every binding artifact and TLS/QWAC key; backups remain private")
	return nil
}
