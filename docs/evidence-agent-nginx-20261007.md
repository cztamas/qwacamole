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

## Production acceptance added later on 7 October

The final production boundaries are now immutable remote modules:

| Module | Implemented ref | Resolved real version |
|---|---|---|
| Core | `3c3fc434d5e40dc378afd8decc29194db79b63b5` | `v0.0.0-20261007193456-3c3fc434d5e4` |
| Caddy | `2c78d367c557736d60c8d5197283ade3a2d35111` | `v0.0.0-20261007200511-2c78d367c557` |
| Agent | `a73695c94a9d85cfe79620567259358b931d7030` | `v0.0.0-20261007204144-a73695c94a9d` |

Subsequent work changes devtools, real dependency pins, CI and documentation.
Caddy code is unchanged from the ref above; agent code now includes a response-
commit gate and explicit `needs_deploy` recovery after retained TLS expiry.

`make test` passed all four workspace race suites and boundaries, exit 0, log
`.local/agent-final-workspace-races.log`. Actual child-process crash tests exit
without cleanup at five boundaries: before signing; after durable binding
manifest but before journal update; after selected staged files but before reload;
after server reload but before return; and at confirmation before commit. The
server adapter persists its loaded leaf separately from the agent's intent/files.
Old and candidate leaves genuinely differ. Two consecutive restarts confirm the
proper old/new served leaf, readiness and retained coverage. Tests also prove
that complete TLS expiry closes delivery and permits fresh deployment without
pruning expired coverage. Logs: `.local/agent-process-crash-tests.log`,
`.local/agent-crash-expiry-races.log`, `.local/agent-gate-crash-final-races.log`.

The real Nginx test exposed an admission-only gate gap: a request authorized just
before binding expiry returned a delayed protected **201** after expiry. Baseline
`.local/agent-nginx-failures-before.log` exits 1 for exactly that regression.
Nginx now forwards the application through the agent's stdlib reverse proxy,
which checks again at header/protocol-upgrade commitment. Rejected upstream
streams are closed instead of drained; already committed streams continue.
An initial intermediate attempt exposed the binding service's 10s write timeout
on long application streams. It was corrected by limiting small binding/gate
responses separately; no overall application-stream write deadline is imposed.
The final suite includes the original held stream across the full renewal.

Final actual lifecycle/fault run:
`.local/agent-nginx-failures-final-20261007/nginx-smoke-1044410626/`, console
`.local/agent-nginx-failures-final.log`, **22 checks, exit 0**. In addition to the
13 earlier lifecycle checks, it proves invalid TLS/QWAC keys, signing/document/
manifest failures before staging, reload failure, a zero-exit reload that does
not activate the candidate, actual invalid config/repair/reconciliation/safe
retry, response commitment after expiry (503), new expired-state requests (403),
expired binding delivery (503), signer repair, and real ACME capacity refusal
without pruning or staging. Fault adapters live only in devtools. The installed
production agent exposes no fault marker or injection command.

Certbot's failed signing and capacity deploy hooks both returned overall success
(`command_error: "<nil>"`) while agent status recorded the separate deployment
error and preserved prior usable TLS. Raw hook logs and exit records are retained.
Normal automatic renewal used no force flag; extra fault candidates explicitly
used `certonly --force-renewal` only after that acceptance passed.

The final original held connection received **121 chunks**, TLS leaf
`bf4f9aa8d9f6186072746279c426c64c63bb90da22bc41936d41591e63073203`.
Fresh TLS changed to
`a87db53ab72c4ca6d59dbc10dc23901d03a496b6aed8a0b7180b5078c67b5e67`.
The overlapping signed binding verifies both, independently of reconnect proof.

## Fresh native browser runs

Chrome **154.0.8037.98**, genuine PKIMetadata component **1798**, injected root
store version **1000000**; Firefox **157.0.1**, geckodriver **0.37.1**. Ordinary
chain, hostname and expiry verification remain active. Chrome uses the same
process-confined CT-only exception as the established demonstration, and distinct
TLS/QWAC trust entries. Firefox uses the established separate test QWAC authority
preference and NSS TLS root with explicit compatibility bindings. No system trust
or normal profile was changed.

```sh
bin/qwacamole integration-test -state .local/agent-native-caddy-20261007
bin/qwacamole firefox-integration -state .local/agent-native-caddy-20261007
bin/qwacamole nginx-browser-integration -native-browser chrome \
  -agent-binary bin/qwacamole-agent-remote -state .local/agent-final-nginx-chrome-20261007
# Firefox runs sequentially after Chrome; final outcome recorded below.
bin/qwacamole nginx-browser-integration -native-browser firefox \
  -agent-binary bin/qwacamole-agent-remote -state .local/agent-final-nginx-firefox-20261007
```

Fresh Caddy Chrome state `integration-20261007-220956` passes initial ACME, native
renewal, continuous original HTTP/2 through TLS expiry and a complete binding
refresh lifetime, restart, independent refresh, signing/publication failures and
recovery. The eleven-fixture suite has exactly ten passes and the accepted
`expired-binding` failure: **“BROWSER INCOMPATIBILITY: native verifier accepted
expired exp header”**. It remains **exit 1**. Native hostname, TLS expiry, profile,
signature, wrong-leaf, missing-trust and CT assertions were not weakened. Raw
NetLogs and `browser-suite.json` are retained in that state's `evidence/`.

Fresh Caddy Firefox state `firefox-integration-4134962586` passes initial/renewed/
replacement/restarted native owner verification, cached discovery past original
signed expiry, unchanged trust, independent refresh, injected failures and all
eleven fixtures, **exit 0**. Console `.local/agent-native-caddy-firefox.log`.

Final Nginx Chrome uses the **remote-installed final agent** in
`nginx-smoke-3566627978`, **exit 0**. It passes native initial, renewed cached
discovery after expiry and QWAC replacement, plus all 13 ordinary lifecycle
checks. Raw NetLog END events are respectively sources **237**, **169**, **155**,
each `CERT_VERIFY_PROC_2QWAC_BINDING` with `is_valid_2qwac_binding: true`.
The verified QWAC chain is checked against the exact separate injected root and
expected leaf; binding BEGIN records the exact expected TLS certificate.
Root-store events confirm **1000000** loaded.

Chrome TLS changes from
`0287083d2a499a12987d811fce329479e16cee719d2b13a47b62e4c1ff897502` to
`b430376b6d2d79d58a5ef6281022e2535e5dfc2e0915abc93100405cdcf6f6fe`.
The same profile's trust SHA-256 remains
`50ef9bb9a1bd3d2397073bb18c72226d28ef9131263f8b7845c9eef7de6ff43f`.
The cache returns age **62s**, original signed expiry **1791405819**, unchanged
body SHA-256 `b2686420bbf510877a6a779804c46f7e2a950f4a7730c33fcce3219adf108a68`
and the same discovery Link. Native verification uses that cached application
response, rather than a newly rendered diagnostic page.

Earlier Nginx Chrome/Firefox runs also passed but preceded the response-commit
fix; they are intermediate evidence, not the final production acceptance.

Final remote-installed Nginx Firefox also passes all three native phases and all
13 lifecycle checks, **exit 0**, console `.local/agent-final-nginx-firefox.log`.
Native Firefox chrome-context assertions verify the expected QWAC owner,
qualified status, exact QWAC leaf and actual TLS fingerprint at each phase.
NSS trust grants, root and profile preferences remain identical across renewal
and replacement. Raw results and trust identities are under
`.local/agent-final-nginx-firefox-20261007/` in the `native-firefox-*` evidence.
Exact state: `nginx-smoke-3717378100`. Native UI reports `connection: secure-etsi`,
`qualified_visible: true`, `certificate_overridden: false`, and WebDriver
`accept_insecure_certs: false`. The owner changes from **Nginx Agent TEST** to
**Nginx Replacement TEST**. TLS fingerprints change from
`140413b9f3f5d1a590cd6c40779437cba72b3de9335b0e0144c360ace423aedb` to
`418e34e819698ff9f9a2f0e702f68fd05334165a5eb4a1f73671af584fb767e9`.
Trust hash is unchanged at
`8d20c503e524acae97e2fdc3e449681fa120e4328f9a097c2abf0f6af160e13a`.

## Remote consumers and hosted CI

```sh
scripts/check-remote-install.sh 2c78d367c557736d60c8d5197283ade3a2d35111
bin/qwacamole plugin-smoke -plugin-binary bin/caddy-remote \
  -state .local/agent-remote-caddy-runtime-20261007
scripts/check-remote-agent.sh a73695c94a9d85cfe79620567259358b931d7030
scripts/check-independent-modules.sh
```

Both clean-cache remote installations exit 0, outside the workspace, using real
immutable versions and no replacements. Caddy inventory contains only its
production modules, no agent/devtools/browser dependencies; agent metadata has
only core plus stdlib. Logs `.local/agent-remote-caddy-install.log` and
`.local/agent-remote-final-install.log`; metadata in `.local/evidence/remote-install/`
and `.local/evidence/remote-agent/`. The remotely installed Caddy then passes all
**22 actual runtime checks**, exit 0, console `.local/agent-remote-caddy-runtime.log`.
The remotely installed agent is the binary used by the final native Nginx runs.

All four final-pinned independent race suites pass, **exit 0**, using
`GOWORK=off` and no replacements; console `.local/agent-final-independent-races.log`.
Devtools independently resolves the actual final agent version above, Caddy's
actual `2c78d36` version and core's actual `3c3fc43` version. `go.work` contains
only workspace `use` entries and cannot hide fake versions.

Intermediate [CI run 37679368783](https://github.com/cztamas/qwacamole/actions/runs/37679368783)
at `2c78d36` passed. Run `37680289426` passed workspace races, independent pinned
races and boundaries, then stalled downloading Ubuntu indexes from the runner's
Azure HTTP mirror and was cancelled for diagnosis. Its logs show successful
official HTTPS metadata but repeated Azure package-index ignores. CI now selects
the official HTTPS mirror, bounds dependency network setup, and separates Nginx,
Certbot and binary-build steps. Race/runtime assertions remain intact.

## Still outstanding

Hosted CI on the complete harness/configuration and the final requirement audit.

The Nginx config/service guide is prepared in [agent-nginx.md](agent-nginx.md).
No merge, tags, publication, visibility or license change has been performed.
