# Agent + Certbot + Nginx — work-in-progress evidence, 7 October 2026

This is preliminary executed evidence, not final acceptance. Feature branch:
`codex/certbot-nginx-agent`, base remote main `3fee651`. Shared core is pushed at
`3c3fc434d5e40dc378afd8decc29194db79b63b5`, remotely resolved as
`v0.0.0-20261007193456-3c3fc434d5e4`. Agent/Caddy source is still evolving;
final immutable refs and acceptance remain outstanding in the
[completion ledger](agent-nginx-plan.md). Historical evidence remains unchanged.

## Tools and executed proof

macOS arm64. Go 1.27.1 with `GOEXPERIMENT=nojsonv2`; Nginx 1.30.5 built under
`.tools/nginx` using existing OpenSSL 3.6.3 and PCRE2; Certbot 5.8.0 in an isolated
Python 3.14 virtual environment; Pebble 2.8.0. Source archive checksum and build
flags are pinned in `scripts/bootstrap-nginx.sh`. No system service was installed.
The bootstrap initially tried disabling PCRE, which rejected rewrite support;
it now uses the existing PCRE2 dependency. Generated credentials/logs stay ignored.

The first real daemon attempt exposed macOS's Unix socket path limit. Config now
supports a private runtime directory, validates the limit before startup, and
locks both state ownership and control-runtime ownership. The successful runs
use an owned short temporary runtime path, mode 0700, socket mode 0600.

```sh
# Repository-local toolchain/cache and authorized private-module authentication.
export GOEXPERIMENT=nojsonv2 GOPRIVATE=github.com/cztamas/qwacamole
export GOPATH="$PWD/.tools/gopath" GOCACHE="$PWD/.tools/gocache"
.tools/go/bin/go test -race ./... ./agent/... ./caddy/... ./devtools/...
make agent-build
scripts/check-boundaries.sh
bin/qwacamole nginx-smoke -state .local/agent-cache-real-20261007
make plugin-build
bin/qwacamole plugin-smoke -state .local/agent-caddy-regression-20261007
```

Four-module workspace races exit 0 (`.local/agent-all-modules.log`). Focused
agent configuration, publication faults, distinct-leaf rollback/retry, owner
locking, restart and stalled cross-host signing tests exit 0; latest focused
log `.local/agent-journal-config-races.log`. The shared lifecycle refactor preserves
the existing core/Caddy failure and concurrent lifecycle tests. The cross-host
regression deliberately blocks one signer and proves another hostname can acquire,
activate, deliver bindings, and release ownership without waiting on it. Same-
generation activation leaves the committed artifact unchanged.

The Caddy and agent modules also independently ran `go mod tidy` and
`GOWORK=off go test -race ./...` using the real pushed core pin, exit 0. This does
not yet prove clean-cache remote installation of the agent. Core, Caddy and agent
production dependency boundaries pass; core remains standard-library-only.

Real Nginx run: `.local/agent-cache-real-20261007/nginx-smoke-2306296001/`, console
`.local/agent-cache-real.log`, exit 0, **13 checks**. Uses a separate production
agent executable, isolated Nginx/Pebble processes and Certbot `certonly --webroot`.
Real challenge validation is enabled (`PEBBLE_VA_ALWAYS_VALID=0`). Initial issuance
uses the actual deploy hook; normal repeated `certbot renew` calls wait until the
certificate becomes due, without `--force-renewal`.

Observed: verified initial TLS activation, application Link/cache/status/body,
streaming, 307 redirect, 418 error and conditional 304; binding GET/HEAD/no-store/
method rejection/nonrecursive delivery; independent signed refresh before TLS
renewal; automatic renewal with fresh verified confirmation; original continuous
TLS stream retained across reload with old/new coverage; real 600s cache through
original signed expiry and QWAC replacement; authenticated document GC preserving
retained leaves; agent-down fail-closed rejection and restart reconciliation.

Earlier dedicated renewal state:
`.local/agent-renewal-real-20261007/nginx-smoke-3037490366/` records the original
connection receiving **121 chunks** while fresh TLS changed from
`1dc04ccebbace2ee64aaf24eb730b53b5f612b711d7318b02f1c6904d9e89373` to
`31f9e6256caedd5467b65d80d4d4a7a44aadc693f92ab1e1bb59f9d2e6dc448d`.
Its `renewal-held.json` proves both leaves bound and unchanged original connection.
A successful reconnect is separately checked and does not substitute for this
held-stream proof.

Rebuilt plugin-only Caddy also passes all **22 existing actual runtime checks**,
console `.local/agent-caddy-regression.log`. This is newly executed Caddy runtime
proof, not a native-browser rerun.

## Still outstanding

Stronger crash/interruption and actual signing/publication/deployment fault
coverage; explicit expired-gate and capacity refusal; fresh native Caddy Chrome
and Firefox; native Nginx Chrome and Firefox; final real pins for every module,
independent consumer/CI and clean-cache remote agent/Caddy installation/runtime;
final documented installation refs and requirement-by-requirement audit.

The Nginx config/service guide is prepared in [agent-nginx.md](agent-nginx.md).
No merge, tags, publication, visibility or license change has been performed.
