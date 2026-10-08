package agent

import (
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
