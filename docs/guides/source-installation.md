# Advanced source installation

Use this path for a custom Caddy build or an agent source build. You need Go
1.27.2 and `GOEXPERIMENT=jsonv2`. The repository is public; no Git credentials or
`GOPRIVATE` override is required. If you followed the earlier private-repository
instructions, remove this repository's entries from `GOPRIVATE`, `GONOPROXY` and
`GONOSUMDB`, preserving entries for other projects. The [packaged installation](binary-installation.md)
does not need Go or a source checkout.

These commands require the `0.1.0-alpha.1` module tags. Check the
[release page](https://github.com/cztamas/qwacamole/releases/tag/v0.1.0-alpha.1)
for publication status and the verified source identities. Stop if a tag is
missing; keep public proxy/checksum verification enabled.

## Caddy with xcaddy

These commands use version `v0.1.0-alpha.1` and its reviewed dependency baseline.
Ensure your Go bin directory is on PATH:

```sh
export GOWORK=off GOEXPERIMENT=jsonv2
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
xcaddy build v2.11.7 \
  --with github.com/cztamas/qwacamole/caddy@v0.1.0-alpha.1 \
  --output ./caddy-qwac
./caddy-qwac list-modules
go version -m ./caddy-qwac
```

Expect `tls.issuance.qwac` and `http.handlers.qwac`, without development modules
or local replacements. Keep this executable separate from an existing Caddy
installation. Continue with the [Caddy reference](caddy-plugin.md).

## Production agent

Use a new scratch directory and the reviewed release version. Installation does
not use a workspace or local module replacements:

```sh
(
  agent_install_dir=$(mktemp -d)
  cd "$agent_install_dir"
  GOWORK=off GOEXPERIMENT=jsonv2 \
    GOMODCACHE="$agent_install_dir/modules" GOBIN="$agent_install_dir/bin" \
    go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@v0.1.0-alpha.1
  "$agent_install_dir/bin/qwacamole-agent" version -json
  go version -m "$agent_install_dir/bin/qwacamole-agent"
  sudo install -m 0755 "$agent_install_dir/bin/qwacamole-agent" /usr/local/bin/qwacamole-agent
)
```

Review the destination before copying; choose a new path for an existing
installation. Continue with [manual configuration](agent-nginx.md) or the Ubuntu
`setup` command in the [quickstart](nginx-quickstart.md).
