package qwacamole

import (
	"context"
	"encoding/json"
	"flag"
	"os"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/cztamas/qwacamole/verification"
)

func init() {
	flags := flag.NewFlagSet("qwac-check", flag.ContinueOnError)
	flags.String("hostname", "", "single configured DNS hostname")
	flags.String("authority", "", "explicitly selected QWAC authority PEM")
	flags.Bool("firefox-compatibility", false, "verify an explicitly configured compatibility binding")
	flags.Bool("json", false, "machine-readable report")
	caddycmd.RegisterCommand(caddycmd.Command{Name: "qwac-check", Usage: "--hostname <host> --authority <PEM> [--json] [--firefox-compatibility]", Short: "Verify HTTPS discovery and the current signed TLS binding", Flags: flags, Func: func(f caddycmd.Flags) (int, error) {
		report, e := verification.Verify(context.Background(), verification.Options{Hostname: f.String("hostname"), Authority: f.String("authority"), FirefoxCompatibility: f.Bool("firefox-compatibility")})
		if f.Bool("json") {
			if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
				return 1, err
			}
		} else {
			report.Write(os.Stdout)
		}
		if e != nil {
			return 1, e
		}
		return 0, nil
	}})
}
