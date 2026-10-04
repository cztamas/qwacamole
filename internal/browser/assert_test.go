package browser

import (
	"encoding/json"
	"github.com/cztamas/qwacamole/internal/pki"
	"os"
	"path/filepath"
	"testing"
)

func TestPositiveRejectsLaterBindingFailure(t *testing.T) {
	// A long-lived browser can first pass, then fail on a later navigation.
	// Earlier success must not make the whole interactive session pass.
	var r Result
	if err := json.Unmarshal([]byte(`{"events":[
	{"type":"CERT_VERIFY_PROC_CHROME_ROOT_STORE_VERSION","params":{"version_major":1000000}},
	{"type":"CERT_VERIFY_PROC_2QWAC_BINDING","phase":2,"params":{"is_valid_2qwac_binding":true}},
	{"type":"CERT_VERIFY_PROC_2QWAC_BINDING","phase":2,"params":{"error_description":"TLS cert not bound","net_error":-2}}
	]}`), &r); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := pki.Generate(dir, pki.Identity{Host: "qwac.example.com", Organization: "Assertion TEST", Country: "HU", Locality: "Budapest", Registration: "NTRHU-TEST"}); err != nil {
		t.Fatal(err)
	}
	root, err := pki.ReadCert(filepath.Join(dir, "qwac-root.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pem, err := os.ReadFile(filepath.Join(dir, "qwac-root.pem"))
	if err != nil {
		t.Fatal(err)
	}
	failure := r.Events[len(r.Events)-1]
	r.Events[len(r.Events)-1] = Event{Type: "CERT_VERIFY_PROC_2QWAC", Phase: 2, Params: map[string]any{"net_error": float64(0), "verified_cert": map[string]any{"certificates": []any{string(pem)}}}}
	if err := Assert(r, "positive", root); err != nil {
		t.Fatal("baseline native acceptance rejected", err)
	}
	r.Events = append(r.Events, failure)
	if err := Assert(r, "positive", root); err == nil {
		t.Fatal("earlier success hid the later native failure")
	}
}
