package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
)

// Set only by the deterministic package builder. Ordinary go install builds
// report their module version and VCS identity from Go's own build information.
var releaseVersion, releaseSource string

type buildIdentity struct {
	Version     string `json:"version"`
	Source      string `json:"source_commit"`
	CoreVersion string `json:"core_version"`
	GoVersion   string `json:"go_version"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	Modified    bool   `json:"modified"`
}

func identity() buildIdentity {
	r := buildIdentity{Version: "development", Source: "unknown", CoreVersion: "unknown", GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	if b, ok := debug.ReadBuildInfo(); ok {
		if b.Main.Version != "" && b.Main.Version != "(devel)" {
			r.Version = b.Main.Version
			r.Source = "module:" + b.Main.Version
		}
		for _, d := range b.Deps {
			if d.Path == "github.com/cztamas/qwacamole" {
				r.CoreVersion = d.Version
				if d.Replace != nil {
					r.CoreVersion += " (replaced)"
				}
			}
		}
		for _, s := range b.Settings {
			switch s.Key {
			case "vcs.revision":
				r.Source = s.Value
			case "vcs.modified":
				r.Modified = s.Value == "true"
			}
		}
	}
	if releaseVersion != "" {
		r.Version = releaseVersion
	}
	if releaseSource != "" {
		r.Source = releaseSource
		r.Modified = false
	}
	return r
}

func versionCommand(args []string, out io.Writer) error {
	f := flag.NewFlagSet("version", flag.ContinueOnError)
	f.SetOutput(out)
	jsonOutput := f.Bool("json", false, "emit build identity as JSON")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("version accepts only -json; no configuration or credentials required")
	}
	r := identity()
	if *jsonOutput {
		return json.NewEncoder(out).Encode(r)
	}
	_, err := fmt.Fprintf(out, "qwacamole-agent %s\nsource %s (modified=%t)\ncore %s\n%s %s/%s\n", r.Version, r.Source, r.Modified, r.CoreVersion, r.GoVersion, r.GOOS, r.GOARCH)
	return err
}
