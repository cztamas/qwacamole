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

## Fresh Chromium acceptance completed

```sh
bin/qwacamole integration-test -state .local/public-alpha-native-20261007 \
  > .local/public-alpha-chrome.log 2>&1
```

Run: `.local/public-alpha-native-20261007/integration-20261007-102928/`.
Stock Chrome **154.0.8037.98**, genuine copied **PKIMetadata 1798**, native loaded
root-store version **1000000**. The integration exited **1**, exactly as required:
**10 passing cases and only** `expired-binding` failing with
`BROWSER INCOMPATIBILITY: native verifier accepted expired exp header`.
No unrelated TLS failure or timeout was accepted. Real TLS hostname/expiry,
TLS/QWAC trust separation, signature/hash/profile/hostname and CT-only controls
passed. The strict assertion and nonzero outcome remain visible.

Initial, automatic renewal, restart and failure-recovery native events all
accepted the binding and exact generated QWAC leaf/root. The event-chain audit
recorded signed owner **Qwacamole Synthetic Organization TEST**, HU,
NTRHU-TEST-0001; leaf SHA-256
`ad381001faaaaa1ad6c1014f4987f56d27d26c4c810367ec53a982c785ebae64`, root
`471906d046e5be2906173bd8a1f53359e01380b559b089146e9912f600b7a4d7`.
Raw `evidence/chrome-native-audit.json` records this audit. This is native NetLog
chain/binding evidence, not a page label or a newly captured native-panel image.

Automatic renewal took **61.188s** while the existing HTTP/1 connection retained
TLS leaf `a3c3a2c233f93d6c314b247ee6c228eef1615b46d0188c3862513335f20e4a49`.
A fresh handshake presented
`d90fd0834434d186edcd9a2584d021137e4c8c800e9ac3fb07741f6e5b56013c`, and the
binding independently covered both. The continuous native HTTP/2 regression
held that second leaf until **10:37:28.05575 +02:00**, **47.056s after** its
**08:36:41Z** NotAfter and longer than the explicit 45s binding lifetime. Its
initial/final TLS IDs match, with **19** accepted native binding events. New
handshake expiry validation remained enforced by the separate TLS negative.

Independent refresh advanced signed iat **1791362259 → 1791362289** and exp
**1791362304 → 1791362334**, changed immutable document bytes/identifier, kept
`/bindings/current.jose` stable and preserved the exact retained TLS set.
Both signing and manifest-publication faults rejected new TLS leaves while
previous HTTPS/binding remained valid; automatic retry and native acceptance
then recovered. `rotation.json`, `refresh-test.json`, `failure-test.json`,
`integration.json`, every `*-result.json` and NetLog remain in the run's evidence.

## Exact immutable remote build completed

```sh
# Existing authorized SSH authentication selected per-process; no token in logs.
GIT_CONFIG_COUNT=1 \
GIT_CONFIG_KEY_0=url.ssh://git@github.com/.insteadOf \
GIT_CONFIG_VALUE_0=https://github.com/ \
GOCACHE="$PWD/.tools/gocache" \
  scripts/check-remote-install.sh 1ecccf0df725a9ae2a33c91a57375f97029618e3
```

Exited **0** in a fresh external module/VCS cache, `GOWORK=off`, no replacements
or devtools dependency/module. Installed plugin:
`v0.0.0-20261007083852-1ecccf0df725`; core:
`v0.0.0-20261007082742-d576a2929a0f`. Binary: `bin/caddy-remote`.
Build metadata/inventory/resolution: `.local/evidence/remote-install/`; console:
`.local/public-alpha-remote-build.log`. This proves authenticated private remote
installation. Anonymous public installation remains an owner-publication gate.
Installed-binary runtime smoke follows the sequential browser runs.

The final all-module race/boundary run exited **0** with log
`.local/public-alpha-tests-final.log`, including a real expiry-driven refresh
outage: unchanged committed state, expired document/protected response refusal,
bounded retries and recovery without reload. The focused race regression also
passed in `.local/public-alpha-refresh-worker.log`.

Hosted run `37595163239` failed because the newly real private core version was
resolved from Go's separate VCS cache without credentials. The follow-up scopes
checkout's existing read-only credential to this repository during the job and
removes it afterward; no credential is printed or committed. The rerun has passed
all-module races and boundaries; its final portable smoke result is pending.
Shared lifecycle helpers were also corrected to honor only the explicit Firefox
compatibility state marker; strict standard verification remains default.

## Offline retirement regression found by hosted smoke

Hosted `37595527410` passed all races, boundaries and the first 21 actual-Caddy
checks, but the new 22nd offline-retirement check failed. Its downloaded Caddy
logs showed a handler activating an empty state despite the issuer having
recovered a stored TLS leaf. Caddy module contexts are distinct: the previous
activation read only its own session's stored leaf. Ordinary restart with an
existing manifest concealed that omission. After a later renewal, the live held
connection still used an uncovered older leaf, which the new assertion rejected.

Activation now gathers stored leaves from every live session of the same
exclusive directory owner and commits them with the selected candidate signer
and newly issued leaf in one transaction. This cannot reinstall older credentials;
additional stored leaves only preserve coverage. A focused regression uses two
separate module contexts and empty recovered state. It and the atomic-activation/
concurrent lifecycle races passed (`.local/public-alpha-module-context.log`).
The full-stop check is retained unchanged. This production correction requires a
new immutable remote build/runtime and fresh browser runs before acceptance.

The first sequential Firefox run completed all native lifecycle/cache/eleven
fixture checks with exit 0 in `firefox-integration-18250276`; its captured panel
was `secure-etsi` with owner `Alpha Firefox TEST`, then
`Alpha Firefox Replacement TEST`. It is intermediate evidence preceding the
module-context correction; final runs will be recorded separately.
