package qwacamole

import (
	"encoding/json"
	"flag"
	"fmt"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"os"
	"runtime"
	"runtime/debug"
)

var releaseVersion = "development"
var releaseSource = "unknown"

func distributionVersion() map[string]any {
	v := map[string]any{"version": releaseVersion, "source_commit": releaseSource, "goos": runtime.GOOS, "goarch": runtime.GOARCH, "go_version": runtime.Version()}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == "github.com/caddyserver/caddy/v2" {
				v["caddy_version"] = d.Version
			}
			if d.Path == "github.com/cztamas/qwacamole" {
				v["core_version"] = d.Version
			}
		}
	}
	return v
}
func init() {
	f := flag.NewFlagSet("qwac-version", flag.ContinueOnError)
	f.Bool("json", false, "machine-readable distribution identity")
	caddycmd.RegisterCommand(caddycmd.Command{Name: "qwac-version", Short: "Show the packaged Qwacamole and Caddy identity", Flags: f, Func: func(flags caddycmd.Flags) (int, error) {
		v := distributionVersion()
		if flags.Bool("json") {
			return 0, json.NewEncoder(os.Stdout).Encode(v)
		}
		fmt.Printf("Qwacamole %s; Caddy %s; source %s\n", v["version"], v["caddy_version"], v["source_commit"])
		return 0, nil
	}})
}
