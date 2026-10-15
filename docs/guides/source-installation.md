# Advanced source installation

Use this path for a custom Caddy build or an agent source build. You need Go
1.27.1, `GOEXPERIMENT=nojsonv2` and authorized private-repository access through
an existing Git credential helper or SSH configuration. Preserve other
`GOPRIVATE` entries if present. The [packaged installation](binary-installation.md)
does not need Go or a source checkout.

## Caddy with xcaddy

Replace `FULL_IMPLEMENTED_CADDY_SOURCE_SHA` with the exact reviewed source you
intend to install. Ensure your Go bin directory is on PATH:

```sh
export GOPRIVATE=github.com/cztamas/qwacamole
export GOWORK=off GOEXPERIMENT=nojsonv2
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
xcaddy build v2.10.2 \
  --with github.com/cztamas/qwacamole/caddy@FULL_IMPLEMENTED_CADDY_SOURCE_SHA \
  --output ./caddy-qwac
./caddy-qwac list-modules
go version -m ./caddy-qwac
```

Expect `tls.issuance.qwac` and `http.handlers.qwac`, without development modules
or local replacements. Keep this executable separate from an existing Caddy
installation. Continue with the [Caddy reference](caddy-plugin.md).

## Production agent

Use a new scratch directory and the exact reviewed source ref. Installation does
not use a workspace or local module replacements:

```sh
(
  agent_install_dir=$(mktemp -d)
  cd "$agent_install_dir"
  GOWORK=off GOEXPERIMENT=nojsonv2 GOPRIVATE=github.com/cztamas/qwacamole \
    GOMODCACHE="$agent_install_dir/modules" GOBIN="$agent_install_dir/bin" \
    go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@FULL_IMPLEMENTED_AGENT_SOURCE_SHA
  "$agent_install_dir/bin/qwacamole-agent" version -json
  go version -m "$agent_install_dir/bin/qwacamole-agent"
  sudo install -m 0755 "$agent_install_dir/bin/qwacamole-agent" /usr/local/bin/qwacamole-agent
)
```

Review the destination before copying; choose a new path for an existing
installation. Continue with [manual configuration](agent-nginx.md) or the Ubuntu
`setup` command in the [quickstart](nginx-quickstart.md).
