# Contributing

This repository is preparing an unpublished public alpha. Repository visibility,
licensing and release publication are owner decisions still outstanding. Do not
assume an open-source license or publish packages/tags on the project's behalf.

Use a feature branch. Keep core (`credentials`, `profile`, `binding`, `store`,
`lifecycle`, `bindinghttp`)
standard-library only; the nested `caddy` module adds the Caddy integration;
`agent` owns supplied-credential deployment and local Nginx process coordination;
`devtools` owns synthetic authorities, browser trust setup, negative fixtures and
test orchestration. Never add test enrollment/browser dependencies to the
installed plugin. Preserve Mozilla fixture attribution and clearly identify the
already-public test key; never commit generated private keys or browser profiles.

The tested development environment is macOS arm64, Go 1.27.1, xcaddy 0.4.5 and
Caddy 2.10.2, with `GOEXPERIMENT=nojsonv2`. See the README for bootstrap and the
dated reports for exact browser/component pairs. Portable CI uses Linux and an
installed Go toolchain; root-only `go test ./...` misses nested modules.

```sh
make build
make test
make plugin-smoke STATE=.local/contribution-smoke
git diff --check
```

For local installed-plugin changes, also run `make plugin-build`, inspect
`bin/caddy-plugin list-modules` and `go version -m bin/caddy-plugin`, and preserve
the dependency-boundary check. When core APIs change, push a core commit, resolve
its real Go version, then update the nested plugin requirement. Test the exact
implemented remote plugin ref with `scripts/check-remote-install.sh <full-SHA>`
and run the portable smoke against `bin/caddy-remote`. Local replacements alone
do not prove independent installation.

For agent changes, run `make agent-build`, real `make nginx-smoke`, then native
`make nginx-chrome-test` and `make nginx-firefox-test` sequentially. Use
`scripts/check-remote-agent.sh <full-SHA>` to install an actual immutable remote
agent, and pass `-agent-binary bin/qwacamole-agent-remote` to the Nginx harness.
The production agent must remain free of Caddy/browser/test-PKI dependencies.
Keep signing/storage/reload/crash injection inside devtools. Test all four
modules with `scripts/check-independent-modules.sh` as well as the workspace;
update real dependency pins after pushing their implemented production refs.

The Nginx metadata acceptance uses a client IPv4 loopback address distinct from
the agent's `127.0.0.1` delivery hop. Linux can bind `127.0.0.2` directly; macOS
requires an assigned alias. The harness reuses an existing distinct loopback
alias on macOS and rejects when none is assigned; it never changes interfaces
or falls back to the agent-hop address.

Tests should exercise behavioral invariants: publication before TLS activation,
old/new complete state under failures, authenticated connection coverage beyond
leaf expiry, cached discovery, and safe owned-artifact collection. Negative tests
must assert their intended error, not timeouts or unrelated transport failures.
For browser-facing changes, run Chrome then Firefox sequentially in fresh
isolated state. Preserve the exact known Chromium ignored-exp failing assertion;
any additional failure needs investigation. Firefox's explicit compatibility mode
does not replace strict standard bindings by default. Page labels/HTTP success
are not native browser proof.

Caddy Chromium integration acceptance uses a two-minute synthetic Pebble TLS
lifetime, while the interactive demo keeps its six-minute default (override with
`demo -tls-lifetime 2m`). The held-connection check still requires the identical
TLS fingerprint beyond leaf expiry plus a complete 45-second binding lifetime.
This bounds interference from Chromium's unrelated background verifier updates;
it does not disable certificate verification or change fixture assertions.

Put newly executed commands, versions, refs, outcomes and raw-evidence locations
in a separately dated report. Keep historical reports intact. PR descriptions
should explain the concrete behavior change, migration and validation limits.
Use [SECURITY.md](SECURITY.md) for vulnerability reports.
