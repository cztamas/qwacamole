package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestVersionWithoutConfiguration(t *testing.T) {
	old := os.Args
	os.Args = []string{"qwacamole-agent", "version", "-json"}
	t.Cleanup(func() { os.Args = old })
	if err := run(); err != nil {
		t.Fatal("version attempted deployment setup", err)
	}
	var out bytes.Buffer
	if err := versionCommand([]string{"-json"}, &out); err != nil {
		t.Fatal(err)
	}
	var r buildIdentity
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Version == "" || r.Source == "" || r.GoVersion == "" || r.GOOS == "" || r.GOARCH == "" {
		t.Fatal("missing build identity", r)
	}
	for _, args := range [][]string{{"secret.pem"}, {"-config", "missing.json"}} {
		if err := versionCommand(args, &out); err == nil {
			t.Fatal("accepted unrelated arguments", args)
		}
	}
}

func TestReleaseIdentityOverridesOnlyVersionAndSource(t *testing.T) {
	oldVersion, oldSource := releaseVersion, releaseSource
	releaseVersion, releaseSource = "0.1.0-alpha.1", "0123456789012345678901234567890123456789"
	t.Cleanup(func() { releaseVersion, releaseSource = oldVersion, oldSource })
	r := identity()
	if r.Version != releaseVersion || r.Source != releaseSource || r.Modified || r.GoVersion == "" || r.GOOS == "" {
		t.Fatal(r)
	}
}
