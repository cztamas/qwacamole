package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/credentials"
)

// Run only on a completed, stopped harness state. This exercises serving changes
// with multiple retained generations, including expired historical TLS leaves.
func agentMigrationPolicyCheck(configPath, binary, evidence string) error {
	cfg, e := agent.LoadConfig(configPath)
	if e != nil {
		return e
	}
	resultPath := filepath.Join(filepath.Dir(configPath), "evidence/result.json")
	data, e := os.ReadFile(resultPath)
	if e != nil {
		return e
	}
	var priorRun struct {
		Error string `json:"error"`
	}
	if e = json.Unmarshal(data, &priorRun); e != nil {
		return e
	}
	if priorRun.Error != "<nil>" {
		return fmt.Errorf("requires completed successful harness state")
	}
	if e = os.MkdirAll(cfg.RuntimeDir, 0700); e != nil {
		return e
	}
	defer os.RemoveAll(cfg.RuntimeDir)
	before, e := migrationInvariantFiles(cfg)
	if e != nil {
		return e
	}
	beforeBytes, _ := json.Marshal(before)
	journalPath := filepath.Join(cfg.StateDir, "deployment.json")
	journal, e := os.ReadFile(journalPath)
	if e != nil {
		return e
	}
	gens, e := os.ReadDir(filepath.Join(cfg.StateDir, "generations"))
	if e != nil {
		return e
	}
	retained, expired := 0, 0
	for _, gen := range gens {
		cert, e := credentials.ReadCert(filepath.Join(cfg.StateDir, "generations", gen.Name(), "fullchain.pem"))
		if e != nil {
			continue
		}
		retained++
		if cert.NotAfter.Before(time.Now()) {
			expired++
		}
	}
	if retained < 2 || expired < 1 {
		return fmt.Errorf("requires multiple retained generations and expired history, got %d/%d", retained, expired)
	}
	size := int64(32 << 20)
	cfg.Version = 1
	cfg.Serving = agent.ServingConfig{MaxBodyBytes: &size, ClientBodyTimeout: "120s", UpstreamSendTimeout: "90s", UpstreamReadTimeout: "120s", RequestBuffering: "off", RedirectHTTPS: true, HSTS: &agent.HSTSConfig{MaxAgeSeconds: 0}}
	targetPath := filepath.Join(filepath.Dir(configPath), "agent-serving-migrated.json")
	if e = writeJSON(targetPath, cfg); e != nil {
		return e
	}
	if e = os.MkdirAll(evidence, 0700); e != nil {
		return e
	}
	for iteration := 0; iteration < 2; iteration++ {
		cmd := exec.Command(binary, "migrate", "-config", targetPath, "-full-stop-confirmed")
		out, err := cmd.CombinedOutput()
		os.WriteFile(filepath.Join(evidence, fmt.Sprintf("migration-%d.log", iteration)), out, 0600)
		if err != nil {
			return fmt.Errorf("versioned serving migration: %s: %w", out, err)
		}
	}
	after, e := migrationInvariantFiles(cfg)
	if e != nil {
		return e
	}
	afterBytes, _ := json.Marshal(after)
	afterJournal, e := os.ReadFile(journalPath)
	if e != nil {
		return e
	}
	var old, next agent.Journal
	json.Unmarshal(journal, &old)
	json.Unmarshal(afterJournal, &next)
	if !bytes.Equal(beforeBytes, afterBytes) || old != next {
		return fmt.Errorf("serving migration changed history, keys or journal meaning")
	}
	if e = agent.CheckStateFormat(cfg); e != nil {
		return e
	}
	if out, err := exec.Command(binary, "validate", "-config", targetPath).CombinedOutput(); err != nil {
		return fmt.Errorf("migrated validation: %s: %w", out, err)
	}
	fmt.Printf("PASS versioned serving migration/repeat retains %d generations (%d expired), all binding/key bytes and journal meaning\n", retained, expired)
	return writeJSON(filepath.Join(evidence, "result.json"), map[string]any{"retained_generations": retained, "expired_generations_preserved": expired, "repeated": true, "same_payload_hashes": true, "same_journal": true, "nginx_validated": true, "activation_attempted": false, "target_serving": cfg.Serving})
}
