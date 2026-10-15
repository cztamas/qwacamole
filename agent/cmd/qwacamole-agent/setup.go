package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cztamas/qwacamole/agent"
	"os"
	"sort"
)

func setupCommand(args []string) error {
	f := flag.NewFlagSet("setup", flag.ContinueOnError)
	var o agent.SetupOptions
	f.StringVar(&o.Hostname, "hostname", "", "single literal DNS hostname")
	f.StringVar(&o.Application, "upstream", "", "HTTP application upstream")
	f.StringVar(&o.Chain, "chain", "", "supplied leaf-first QWAC PEM chain")
	f.StringVar(&o.Key, "key", "", "matching private signing key")
	f.StringVar(&o.Authority, "authority", "", "explicitly selected QWAC authority PEM")
	f.StringVar(&o.Email, "email", "", "ordinary ACME contact")
	f.StringVar(&o.ACMEServer, "acme-server", "", "ordinary HTTPS ACME directory (default Let's Encrypt)")
	f.BoolVar(&o.DryRun, "dry-run", false, "preflight and show planned changes without installing files")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fmt.Errorf("setup accepts flags only")
	}
	p, e := agent.Setup(context.Background(), o)
	if e != nil {
		return e
	}
	if p.Existing {
		fmt.Println("Setup already matches; files, credentials and binding state preserved.")
	} else if o.DryRun {
		fmt.Println("Dry run: no deployment files changed.")
		dirs := make([]string, 0, len(p.Directories))
		for path := range p.Directories {
			dirs = append(dirs, path)
		}
		sort.Strings(dirs)
		for _, path := range dirs {
			fmt.Fprintf(os.Stdout, "directory %04o %s\n", p.Directories[path], path)
		}
		for _, file := range p.Files {
			fmt.Fprintf(os.Stdout, "file %04o %s\n", file.Mode, file.Path)
		}
	} else {
		fmt.Println("Configured one dedicated Nginx deployment. Services have not been started.")
	}
	fmt.Println("Start, issue ordinary TLS, then verify:")
	for _, command := range p.Commands {
		fmt.Println("sudo " + command)
	}
	return nil
}
