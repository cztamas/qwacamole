package agent

import (
	"encoding/json"
	"github.com/cztamas/qwacamole/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownFutureStateRejectedWithoutMutation(t *testing.T) {
	for _, file := range []string{"format.json", "deployment.json"} {
		t.Run(file, func(t *testing.T) {
			c := validConfig()
			c.StateDir = t.TempDir()
			os.Chmod(c.StateDir, 0700)
			data := []byte(`{"state_version":99,"nginx_format_version":1}`)
			if file == "deployment.json" {
				data = []byte(`{"state_version":99,"phase":"empty","updated":"2026-10-08T00:00:00Z"}`)
			}
			path := filepath.Join(c.StateDir, file)
			if e := os.WriteFile(path, data, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := Open(c, nil, store.Operations{}); e == nil || !strings.Contains(e.Error(), "unsupported") {
				t.Fatal(e)
			}
			got, _ := os.ReadFile(path)
			entries, _ := os.ReadDir(c.StateDir)
			if string(got) != string(data) || len(entries) != 1 {
				t.Fatal("future state mutated")
			}
		})
	}
}

func TestConfigurationChangesAreClassifiedBeforeMutation(t *testing.T) {
	c := validConfig()
	c.StateDir = t.TempDir()
	os.Chmod(c.StateDir, 0700)
	c.Version = 1
	data, _ := json.Marshal(c)
	os.WriteFile(filepath.Join(c.StateDir, "deployment-config.json"), data, 0600)
	os.WriteFile(filepath.Join(c.StateDir, "format.json"), []byte(`{"state_version":1,"nginx_format_version":1}`), 0600)
	changed := c
	changed.Serving.RedirectHTTPS = true
	if e := CheckStateFormat(changed); e == nil || !strings.Contains(e.Error(), "supported serving settings changed") {
		t.Fatal(e)
	}
	changed = c
	changed.Host = "other.test"
	if e := CheckStateFormat(changed); e == nil || !strings.Contains(e.Error(), "incompatible") {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(filepath.Join(c.StateDir, "deployment-config.json"))
	if string(got) != string(data) {
		t.Fatal("configuration change mutated state")
	}
}
