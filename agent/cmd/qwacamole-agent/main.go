// qwacamole-agent owns supplied QWAC bindings and one local Nginx deployment.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cztamas/qwacamole/agent"
	"github.com/cztamas/qwacamole/store"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: qwacamole-agent validate|render-nginx|serve|deploy|certbot-hook|replace-qwac|reconcile|status|check|migrate -config FILE")
	}
	cmd := os.Args[1]
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fullStop := f.Bool("full-stop-confirmed", false, "operator confirms Nginx master and every worker/connection, agent and renewal automation fully stopped")
	path := f.String("config", "", "absolute deployment config file")
	if e := f.Parse(os.Args[2:]); e != nil {
		return e
	}
	if *path == "" || f.NArg() != 0 {
		return fmt.Errorf("-config FILE required; no positional arguments")
	}
	c, e := agent.LoadConfig(*path)
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch cmd {
	case "migrate":
		return agent.Migrate(ctx, c, *fullStop)
	case "render-nginx":
		fmt.Print(agent.RenderNginx(c))
		return nil
	case "validate":
		if e = agent.CheckStateFormat(c); e != nil {
			return e
		}
		if _, e = c.Material(); e != nil {
			return e
		}
		if e = (agent.Nginx{Config: c}).Validate(ctx); e != nil {
			return e
		}
		fmt.Println("valid QWAC credentials and Nginx deployment configuration")
		return nil
	case "serve":
		if e = agent.CheckStateFormat(c); e != nil {
			return e
		}
		if e = (agent.Nginx{Config: c}).Prerequisites(ctx); e != nil {
			return e
		}
		a, e := agent.Open(c, nil, store.Operations{})
		if e != nil {
			return e
		}
		defer a.Close()
		return a.Serve(ctx, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	case "certbot-hook":
		domains := strings.Fields(os.Getenv("RENEWED_DOMAINS"))
		lineage := os.Getenv("RENEWED_LINEAGE")
		if len(domains) != 1 || domains[0] != c.Host || filepath.Clean(lineage) != filepath.Clean(c.CertbotLineage) {
			return fmt.Errorf("Certbot renewal environment does not match configured single-host lineage")
		}
		cmd = "deploy"
	case "check":
		return agent.Check(ctx, c)
	case "deploy", "replace-qwac", "reconcile", "status":
	default:
		return fmt.Errorf("unknown command %s", cmd)
	}
	result, e := agent.Control(ctx, c, cmd)
	if len(result) > 0 {
		fmt.Println(string(result))
	}
	return e
}
