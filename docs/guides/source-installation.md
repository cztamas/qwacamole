# Advanced source installation

Use this path for a custom Caddy build or an agent source build. You need Go
1.27.2 and `GOEXPERIMENT=jsonv2`. The repository is public; no Git credentials or
`GOPRIVATE` override is required. If you followed the earlier private-repository
instructions, remove this repository's entries from `GOPRIVATE`, `GONOPROXY` and
`GONOSUMDB`, preserving entries for other projects. The [packaged installation](binary-installation.md)
does not need Go or a source checkout.

**Current availability:** the anonymous installation check on 17 October 2026
failed when the public Go proxy/checksum service could not serve the retained
core dependency. The source commands below have not yet passed that check.
Use the verified [packaged candidate](binary-installation.md) meanwhile; do not
disable checksum verification or add private-module exceptions to bypass it.

## Caddy with xcaddy

These commands use the reviewed implemented source `6b966e265f5535167da8413f2c7b7b2964a5e873`.
Use another ref only after reviewing its compatibility with this dependency baseline.
Ensure your Go bin directory is on PATH:

```sh
export GOWORK=off GOEXPERIMENT=jsonv2
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
xcaddy build v2.11.7 \
  --with github.com/cztamas/qwacamole/caddy@6b966e265f5535167da8413f2c7b7b2964a5e873 \
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
  GOWORK=off GOEXPERIMENT=jsonv2 \
    GOMODCACHE="$agent_install_dir/modules" GOBIN="$agent_install_dir/bin" \
    go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@6b966e265f5535167da8413f2c7b7b2964a5e873
  "$agent_install_dir/bin/qwacamole-agent" version -json
  go version -m "$agent_install_dir/bin/qwacamole-agent"
  sudo install -m 0755 "$agent_install_dir/bin/qwacamole-agent" /usr/local/bin/qwacamole-agent
)
```

Review the destination before copying; choose a new path for an existing
installation. Continue with [manual configuration](agent-nginx.md) or the Ubuntu
`setup` command in the [quickstart](nginx-quickstart.md).
