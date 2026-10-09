# Rejected Caddy streaming responses: review follow-up, 9 October 2026

[The PR review](https://github.com/cztamas/qwacamole/pull/6#discussion_r4224403552)
and supplied second review identify the same resource defect in
`caddy/http.go`: a protected request admitted before binding expiry could receive
upstream headers afterward, select 503, then continue reading an endless or
stalled rejected body. Both the `Write` loop and `ReadFrom` needed correction.

## Implementation and boundaries

Production Caddy/test fix: `9322bd566e825a2c2c645e464a630e09d7aa68d6`, version
`v0.0.0-20261009072701-9322bd566e82`. Independent devtools pin:
`32287d270f45fa596f92846a8bf355476f054e56`. Final test dependency metadata
is at `d4489de938fed25df1299d2927165cff41975995`, adding the real proxy
package's existing transitive checksum/requirement and preserving prior checksum
ordering. Independent resolution initially exposed that missing checksum; after
recording it, all four independent race suites pass. The core stays at `8e62f4b8c340`
and the agent at `bdd733dc62ab`; no durable state or module boundary changes.

Protected downstream handlers receive a child request context. When commitment
rejects the binding, the writer clears protected headers, selects a bodyless
`503` with `Cache-Control: no-store`, and cancels that child context. The incoming
client context remains alive. This interrupts Caddy's HTTP transport even when
its upstream body is stalled before the first write. Rejected `Write` returns a
copy-stopping error; rejected `ReadFrom` closes a closable reader without reading
and returns that same error. There is no discarded-body drain.

Caddy closes the upstream body and raises `http.ErrAbortHandler` on failed body
copy. The QWAC middleware contains that abort **only after its gate has rejected
the response** so the replacement 503 completes normally instead of resetting
HTTP/2. Directly returned gate copy errors also complete the rejection normally.
Valid-response aborts and unrelated panics still propagate. Already committed
valid responses and upgraded streams keep their existing semantics.

## Regression and required checks

`TestCaddyExpiredReverseProxyCancelsUnboundedResponse` uses the real Caddy 2.10.2
reverse proxy, separate live origin/front listeners and normally verified TLS.
It covers both HTTP/1.1 and HTTP/2, each with an endless streaming body or a body
stalled after headers. Requests reach the origin before the signed three-second
binding expiry; a signing outage prevents maintenance extending that deadline.
Origin headers are released only afterward. The origin never ends its response
until cancellation (or failure cleanup).

All four cases fail against the previous writer with the intended diagnosis,
`rejected unbounded response did not complete promptly`; they pass with the fix
under the race detector. Each requires complete 503/body EOF within two seconds,
no protected headers/body, the actual expected client protocol, the serving
handler returning, and the upstream request observing cancellation. A client
fallback timeout cannot satisfy those assertions.

`TestRejectedResponseStopsBothCopyPaths` requires `ReadFrom` to close without ever
calling `Read`, both copy paths to stop, child-only cancellation and clean 503.
`TestCaddyResponseGatePreservesOtherPanics` checks that a valid response's abort
and an unrelated panic after rejection are not swallowed. The retained delayed
upgrade test still rejects headers/body and blocks underlying hijacking; its
finite-reader assertion now requires the intentional copy-stopping error.

Fresh local environment: macOS arm64, Go **1.27.1** with
`GOEXPERIMENT=nojsonv2`, Caddy **2.10.2**, CertMagic **0.24.0**, xcaddy **0.4.5**
and the pinned Pebble **2.8.0** module. Browser/driver versions are recorded below.

Fresh commands/results:

- Focused race regressions: pass; `.local/evidence/review-stream-rejection-after.log`.
- Before-fix real-proxy regression: four intended failures;
  `.local/evidence/review-stream-rejection-before.log`.
- `make test`: all four module races and dependency boundaries pass;
  `.local/evidence/review-stream-required.log`.
- `make build plugin-build`: pass; `.local/evidence/review-stream-build.log`.
- `scripts/check-independent-modules.sh`: all four pinned races pass with
  `GOWORK=off`, no replacements; `.local/evidence/review-stream-independent-final.log`.
- `bin/caddy-plugin list-modules` and `go version -m bin/caddy-plugin`: pass;
  `.local/evidence/review-stream-plugin-{modules,build-modules}.txt`.
- `scripts/check-remote-install.sh 9322bd566e825a2c2c645e464a630e09d7aa68d6`:
  pass with fresh caches, no local replacements/devtools and the unchanged core
  pin; `.local/evidence/review-stream-remote-install/` contains resolved module and
  build metadata.
- `git diff --check`: pass.

## Fresh remote and native acceptance

The immutable remotely installed production Caddy passes **26/26** runtime
checks, including actual credential expiry rejecting delayed HTTP response,
WebSocket upgrade and fresh TLS, recovery without restart, two automatic ACME
renewals with original held coverage, both replacement key choices, clean restart
and offline retirement. `.local/evidence/review-stream-remote-caddy-runtime.log`.

Stock Chrome **154.0.8037.99** with genuine PKIMetadata **1798** passes all native
lifecycle phases: initial/renewed identity, same original HTTP/2 session through
TLS expiry, cached renewed/replacement identity with unchanged trust, restart,
independent refresh, injected signing/publication failure preservation and native
automatic recovery. Its fixture suite remains **10/11, exit 1**, solely for the
preserved expired-binding `exp` incompatibility. No assertion is weakened.
Raw NetLog confirms all **14** held requests use HTTP/2 session **125**, with the
same original TLS fingerprint at `2026-10-09T07:34:11.026731Z`, **47.026 seconds**
after its `07:33:24Z` expiry. The normal profile and system trust are unchanged.
`.local/evidence/review-stream-caddy-chrome.log`.

Firefox **157.0.1**, geckodriver **0.37.1**, exits **0**: all six native lifecycle
phases and **11/11** fixtures pass, including expired-binding rejection. Every
positive phase reports `secure-etsi`, visible qualification, no certificate
override and `accept_insecure_certs: false`. Cached content/discovery, profile
preferences/trust, renewed/replacement identity, restart, independent refresh and
automatic failure recovery are preserved.
`.local/evidence/review-stream-caddy-firefox-final.log`.

The first Firefox launcher invocation used unsupported `-driver` and exited 2
before starting a demo or tests. It is retained in the command ledger and is not
browser acceptance evidence. The corrected invocation uses `-geckodriver`.
Browser/server runs are sequential with distinct state and isolated profiles;
only the documented CT development exception and Firefox compatibility/test
trust setup are used. No broad TLS bypass is added.

[Hosted Linux CI](https://github.com/cztamas/qwacamole/actions/runs/37899766458)
passes at exact executable/test/dependency source
`d4489de938fed25df1299d2927165cff41975995`: all **21 steps** succeed, including
both four-module race configurations, dependency boundaries, production/plugin
builds, default/custom-policy Nginx, immutable-baseline interrupted/repeated
upgrades and Caddy/Pebble. [Run metadata](evidence/caddy-stream-rejection-20261009/hosted-ci.json)
records that tested head and every step. The subsequent report/evidence commit is
documentation-only; this run does not claim to test that later PR head.

The [sanitized summary](evidence/caddy-stream-rejection-20261009/summary.json)
and [command ledger](evidence/caddy-stream-rejection-20261009/commands.json)
record exact source refs, commands, times, results, installed binary identities,
native trust/identity signals and the preserved Chromium failure.

This follow-up's browser/server executions are fresh. The earlier
[8 October lifecycle report](evidence-lifecycle-20261008.md) remains inherited
historical evidence and is not rewritten. Generated private keys, profiles and
raw browser logs remain ignored; public evidence contains only sanitized results.
