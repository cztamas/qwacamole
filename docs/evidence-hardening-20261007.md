# Sustained-use hardening evidence — 7 October 2026

This report records newly executed work on `codex/public-alpha-hardening`, based
on `8dd1dd7ee2e3cb96fa005938e98db6355a675c00`. Historical browser reports remain
unchanged. **Whole-goal acceptance is pending** until the ledger's native-browser,
exact-ref installation and remaining operational checks have authoritative proof.

## Implemented behavior and policy investigation

Stable `/bindings/current.jose` discovery is separate from immutable manifest
artifacts. Candidate credentials and complete retained/stored/new TLS leaves
activate in one transaction. Publication verifies signed bytes, checks capacities,
writes/fsyncs the document, verifies durable readability, then commits/fsyncs the
manifest. Ordinary failed commits restore the prior manifest if rename already
occurred; rollback storage errors are explicit. Signed metadata is decoded from
the verified document, rather than trusted from an injected signer's return value.

Expiry-driven refresh uses validated proportional margins, bounded exponential
retry jitter/backoff and publication notifications. Document GC authenticates
ownership and preserves current/staged/grace-protected files under the same
reader/publication lock. TLS retention is separate: count/size limits refuse
publication rather than drop potentially held connection coverage. Structured
diagnostics expose QWAC/binding expiry, refresh, failures, counts and storage.

Pinned Caddy 2.10.2 `modules/caddyhttp/server.go:638–694` exposes ConnState,
ConnContext and shutdown callbacks by configuring `http.Server`; HTTP/3 is created
separately at lines 616–632. Server-side Go TLS ConnectionState exposes peer
certificates, not the selected local server certificate. Those callbacks alone
cannot prove per-leaf connection retirement across protocols/reloads/issuer-only
use. The supported policy retains coverage, enforces capacities and requires the
full process drain/offline retirement procedure in [operations](operations.md).

## Commands completed

Toolchain: bundled Go 1.27.1, `GOEXPERIMENT=nojsonv2`, xcaddy 0.4.5, Caddy 2.10.2,
CertMagic 0.24.0, Pebble 2.8.0. Commands run at repository root unless noted.

```sh
make test
make plugin-build
GOEXPERIMENT=nojsonv2 GOPATH="$PWD/.tools/gopath" GOCACHE="$PWD/.tools/gocache" \
  .tools/go/bin/go build -o bin/qwacamole ./devtools/cmd/qwacamole
bin/qwacamole plugin-smoke -state .local/public-alpha-20261007 \
  -plugin-binary bin/caddy-plugin
git diff --check
```

All-module race tests and boundaries exited **0**; log:
`.local/public-alpha-tests.log`. The portable plugin-only Caddy smoke exited
**0**, with **21 checks**, including real challenge-validated Pebble renewal,
preserved proxy streaming/headers, GET/HEAD/no-store isolation, conditional 304,
307 redirect, a real WebSocket 101 plus echo traffic, an actual HTTP intermediary
whose unchanged cached Link outlives binding expiry and discovers renewal/QWAC
replacement, automatic document collection, three forced reloads, exclusive
ownership, rejected replacement/reload recovery, process restart, static serving,
two independent protected sites beside an unprotected HTTP site, and a paired
middleware custom Caddy error route preserving 418/body/Link/cache headers.

Successful smoke state:
`.local/public-alpha-20261007/plugin-smoke-1415605627/`; console:
`.local/public-alpha-smoke.log`. Raw `evidence/result.json` lists every check.
The earlier run `plugin-smoke-1187598986` failed at the final file-server reload
because the test mixed explicit 45s and accelerated 6s ownership settings; both
adapted issuer/middleware settings were aligned before the successful rerun.

Meaningful new regressions in `devtools/internal/coretest/hardening_test.go` cover
stored TLS absent from recovered coverage, complete-set signing failure after
preparation, immutable/current snapshot separation, caller-copy isolation,
publication failure at signing/independent verification/document write/readback/
manifest commit/post-rename error, process exits before/after durable manifest
commit, authenticated GC ownership/grace/current/orphan rules, enforced capacity
and legacy recovery, a real loopback caching intermediary across expiry/renewal/
replacement, and concurrent readers/renewal/reload/refresh/collection. The focused
race runs exited **0**. A first loopback cache run hit sandbox socket restrictions;
the same test passed with approved listener access. No assertion was skipped.

## Real core dependency

Core source commit pushed:
`d576a2929a0fc6105fd61668b7b31c18c1a20809`.

```sh
# In caddy/, GOWORK=off, using normal authorized private-GitHub authentication:
go list -m -json github.com/cztamas/qwacamole@d576a2929a0fc6105fd61668b7b31c18c1a20809
go get github.com/cztamas/qwacamole@v0.0.0-20261007082742-d576a2929a0f
go mod tidy
```

Remote Go resolution succeeded and the nested plugin pins
`v0.0.0-20261007082742-d576a2929a0f`. Final immutable plugin ref, fresh external
installation/runtime, hosted CI and native runs are still to be recorded below.

## Native and release gates still pending

A fresh Chrome lifecycle runner has started; its initial and renewed native
acceptance passed. Continuous HTTP/2-through-expiry, restart/refresh/failure and
the exact ignored-exp suite outcome remain pending until its live handle exits
and raw artifacts are audited. Do not count startup/progress as a complete run.

Firefox must follow sequentially: native owner/status, same unchanged profile and
trust across renewal, cached discovery after binding expiry and replacement,
restart/failure recovery, and all eleven fixtures. No prior screenshot or dated
run substitutes for this branch's evidence.

Release artifacts are drafts. License, repository visibility, tags/releases,
anonymous public installation, confidential reporting-channel activation and
Caddy registration/hosted-builder compatibility remain explicit owner actions
in [the alpha checklist](alpha-release.md). Nothing has been merged or published.
