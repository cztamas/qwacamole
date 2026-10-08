# Lifecycle guarantees acceptance, 8 October 2026

Assessment started from merged `main` `43c8e16`. The prior supplied-credential
report is inherited context; the executions below are fresh lifecycle acceptance.
The [assessment](lifecycle-plan-20261008.md) records concrete gaps before changes.
The [lifecycle contract and acceptance matrix](lifecycle.md) compare triggers,
publication/activation boundaries, failure/recovery, retention and operational
limits for Caddy and Certbot/Nginx.

## Changes and immutable sources

Reusable core: `8e62f4b8c340e572584f6d71355a8f89a7d0be42`, version
`v0.0.0-20261008203612-8e62f4b8c340`. Signing and independent verification cap
validity at every supplied QWAC chain certificate, retaining the latest-TLS-leaf
cap needed for overlapping connections. Protected availability verifies the
committed document instead of relying on its timestamp alone. The existing
manager/worker now report committed/configured signer, retained TLS IDs, chain
deadline and retry state; activation wakes the same worker.

Agent: `bdd733dc62ab94e01d17837070c1f286a5ceeb3c`, version
`v0.0.0-20261008203929-bdd733dc62ab`. Authenticated status distinguishes failed or
pending replacement from committed identity. The existing deployment journal,
fresh normally verified activation probe and rollback/reconciliation stay intact.

Caddy: `2089dd5b673e3358924179aa4700a56475356633`, version
`v0.0.0-20261008204731-2089dd5b673e`. Stored managed-certificate caching activates
coverage and maintenance without application traffic. An abortable pre-handshake
event requires verified coverage; ordinary event forwarding and the exact
TLS-ALPN validation exception are preserved. The original issuer gate still
publishes before CertMagic persistence. Response commitment and hijacking check
verified availability and suppress protected headers/body on rejection. Rejected
body writes are discarded so Caddy's reverse proxy delivers the 503 instead of
aborting HTTP/2. Older generation leases cannot select an older signer.

Primary harness/test implementation:
`85a7335b9de98427520c3aed7a7318fdecbb49c8`. All nested modules pin real implemented
core/consumer versions. Four module boundaries and durable journal/state formats
are preserved; no second transaction system, scheduler or remote issuance is added.
Additional recovery-matrix tests are at
`a80cf58427d0232b33dad38327382b87478445c2`; production and executable harness
code are unchanged in that commit. The final Chromium fixture revision is
`934f4f9749763a8c265cdbb73441b853ebf522d0`; it adds a bounded synthetic TLS
lifetime without changing production or browser verification/assertion code.
The command ledger distinguishes these harness revisions.

## Environment and commands

macOS arm64; Go 1.27.1 with `GOEXPERIMENT=nojsonv2`; Caddy 2.10.2,
CertMagic 0.24.0, xcaddy 0.4.5, Nginx 1.30.5, Certbot 5.8.0, Pebble 2.8.0.
Stock Chrome 154.0.8037.98 with genuine PKIMetadata 1798; Firefox 157.0.1,
geckodriver 0.37.1 and NSS certutil. Fresh isolated profiles use the documented
CT-only development exception and Firefox compatibility/test authority. No broad
TLS bypass, normal-profile/system trust change or commercial issuer is used.

Required commands: `make build agent-build plugin-build`, `make test`,
`scripts/check-independent-modules.sh`, `bin/caddy-plugin list-modules`,
`go version -m bin/caddy-plugin`, and `git diff --check`. Raw final test logs are
`.local/evidence/lifecycle-final934-{required-checks,independent}.log`;
build output is `.local/evidence/lifecycle-final-build.log`.

Remote production installations use fresh caches outside the workspace, with
`GOWORK=off` and no local replacements:

```sh
scripts/check-remote-agent.sh bdd733dc62ab94e01d17837070c1f286a5ceeb3c
scripts/check-remote-install.sh 2089dd5
```

The Caddy script resolves the abbreviated immutable ref to the full commit above.
Installed build metadata confirms the implemented core pins and excludes
devtools/browser/PKI dependencies. Retrieval uses existing repository
authentication while private; this does not claim anonymous installation.
Raw records: `.local/evidence/remote-agent/`, `.local/evidence/remote-install/`.
The [public installation summary](evidence/lifecycle-20261008/installations.json) records resolved identities and dependencies.

The [command ledger](evidence/lifecycle-20261008/commands.json) records exact native/runtime arguments, UTC start/end times,
source identities, exits and raw evidence paths. Browser and service runs are
sequential in fresh state. Public summaries omit generated keys, profiles and raw
NetLogs. [Tool versions](evidence/lifecycle-20261008/tool-versions.json) and
[required-check results](evidence/lifecycle-20261008/checks.json) retain the
corresponding public provenance.

## Required checks and recovery matrix

All four workspace race suites and dependency boundaries pass. Independent
`GOWORK=off` races pass for all four modules with no local replacements. The
final build, production-only registrations and build metadata checks pass.
The complete check runs include the final recovery matrix, not only the earlier
implementation tests.

New focused tests cover intermediate/root validity caps in standard and Firefox
bindings, retained expired TLS alongside a live leaf, quiet stored-Caddy startup,
cache-event publication failure followed by handshake rejection, the exact ACME
ALPN exception, response-copy/upgrade rejection, retry diagnostics/wakeup,
concurrent agent deploy/reconcile/refresh/replacement/collection, rejected
replacement diagnostics and committed-versus-configured signer after restart.

The additional state-damage matrix repeats recovery after missing/malformed
manifests, mutable URLs, missing/corrupt documents, inconsistent signed times and
TLS identities. An absent manifest authorizes no orphan document. Missing agent
intent leaves existing HTTPS closed; malformed/unsupported journal intent
rejects. Restoring consistent inputs allows repeated verified recovery, including
ownership reacquisition after failed opens. Existing process-crash tests rerun
publication boundaries and all deployment phases against actual served state.

The [acceptance matrix](lifecycle.md#acceptance-matrix) maps each guarantee to
these and the retained failure, concurrency, retention and crash tests.

[Hosted Linux CI](https://github.com/cztamas/qwacamole/actions/runs/37843988816)
passes at implementation/test source
`57fa86ed74e0a9e2725030cdb72d3e39cd36f7b5`. All 21 steps succeed, including
workspace and independently pinned races, dependency boundaries, plugin build,
default/custom-policy Nginx, immutable-baseline interrupted/repeated upgrades,
and Caddy/Pebble. [Run metadata](evidence/lifecycle-20261008/hosted-ci-implementation.json)
records the exact tested head and step results. Subsequent executable source
`934f4f9` changes only Chromium's synthetic lifetime from two to four minutes;
its full required local checks and builds pass as recorded above. Documentation
and sanitized evidence are added afterward; this implementation run does not
claim to test that later PR head.

## Fresh native and real-server evidence

Caddy Chromium completes every lifecycle phase at harness source `934f4f9`:
initial, renewal, held expiry, replacement, restart, independent refresh,
signing/publication failure preservation and automatic recovery. Five direct
native phases and three cached identity/trust phases report native binding
acceptance with root-store version 1000000. Cached content and discovery remain
identical while renewed TLS and replacement QWAC identities change; profile
trust digests remain identical.

The original TLS fingerprint is unchanged through `2026-10-08T21:26:21.031485Z`,
**47.031 seconds after** its `21:25:34Z` expiry. Raw NetLog independently confirms
all 14 held requests use the same original HTTP/2 session (126), spanning
272.160 seconds. This proves connection continuity beyond comparing fingerprints.
The fixture suite is **10/11**, command exit **1**, solely because Chromium still
accepts the expired binding `exp` header. The required assertion remains failing;
the suite is **not reported as passing**. All trust, signature, identity, profile,
TLS name/date and CT negatives pass. [Chromium summary](evidence/lifecycle-20261008/caddy-chrome.json)
retains that failure and the held/native/cache evidence.

Caddy Firefox exits **0**, with all **11/11** fixtures passing. Six native phases
verify initial/direct and cached identity, renewal, replacement, restart and
automatic failure recovery. Every positive phase reports `secure-etsi`, visible
qualification, `certificate_overridden: false` and `accept_insecure_certs: false`.
Profile trust/preferences and cached content/discovery remain unchanged.
[Firefox summary](evidence/lifecycle-20261008/caddy-firefox.json).

The remotely installed production agent passes Nginx native demonstrations in
**both Chromium and Firefox**, **24 checks each**. Each verifies initial, renewed
and replacement identities through unchanged cached discovery after its original
binding expired, with identical trust digests across native phases. Chromium
records native binding acceptance and root-store version 1000000; Firefox shows
visible native qualification without certificate overrides. Both runs perform
two real Certbot renewals while the original active TLS stream remains held and
all three TLS leaves remain bound. Same-key and new-key replacement are exercised,
followed by authenticated collection, unavailable-agent rejection and restart
reconciliation. [Nginx Chromium](evidence/lifecycle-20261008/nginx-chrome.json),
[Nginx Firefox](evidence/lifecycle-20261008/nginx-firefox.json).

The fresh remotely installed Caddy build passes **26/26** runtime checks. Two
automatic ACME renewals are each confirmed by fresh normally verified TLS while
the original connection fetches a binding covering its old leaf and the newly
served leaf. Same-key/new-key replacement preserves cached discovery and retained
coverage. Actual QWAC expiry rejects delayed HTTP commitment, WebSocket upgrade
and a fresh ordinary handshake; restoring the configured credential recovers
service without restarting. Repeated reloads, conflicting ownership, clean
restart, multiple protected sites and offline retirement also pass.
[Remote Caddy runtime](evidence/lifecycle-20261008/remote-plugin-runtime.json).

The fresh remotely installed agent passes **34/34** real Certbot/Nginx runtime
and fault checks. Repeated renewal/held coverage, independent refresh, both
replacement key choices and restart pass. Signing, document and manifest failures
prevent staging/activation. Reload failure and a zero-exit reload that serves the
wrong leaf both restore the freshly verified prior deployment. Invalid Nginx
configuration reconciles and safely retries an already-bound candidate. Actual
signed expiry rejects delayed protected responses and new upgrades; signer repair
republishes and reopens the gate. A real over-capacity ACME candidate refuses
before staging and preserves all retained TLS coverage.
[Remote Nginx runtime](evidence/lifecycle-20261008/remote-nginx-runtime.json).

## Corrections found during acceptance

The expanded Nginx capacity fixture initially filled its old three-certificate
limit before the intended signing-fault test. It now allows four retained leaves:
initial, two real renewals, and one fault-recovery candidate. The subsequent
over-capacity test still rejects the next candidate and requires preservation of
the current deployment and full retained set. No production limit or assertion
was weakened.

The initial Caddy expiry fixture waited for a superseded binding's expiry while
automatic refresh legitimately extended validity. It now waits beyond the QWAC
credential's hard deadline. That corrected scenario exposed a real response
defect: the gate's write error caused Caddy to reset HTTP/2 before delivering 503.
The production fix discards rejected writes, and the focused test requires empty
protected content, removed headers, no hijack and successful rejection writes.
Both delayed HTTP and real WebSocket checks remain required.

The first fresh Chromium run failed the held-connection assertion and stopped
before the fixture suite: Chrome recreated certificate verifiers and retired
the original HTTP/2 session after a network-time response. No Caddy GOAWAY was
observed for that session. This attempt is preserved as a failure, not native
held-coverage proof. [Reconnect attempt](evidence/lifecycle-20261008/caddy-chrome-reconnect-attempt.json).
An initial two-minute fixture passed held coverage and native replacement, then
correctly failed the independent-refresh assertion: renewal every 24 seconds
continually reset the 30-second binding refresh deadline. That failed attempt is
also retained. [Short-fixture attempt](evidence/lifecycle-20261008/caddy-chrome-short-attempt.json).
The final four-minute fixture leaves a 48-second renewal interval, permitting
independent refresh while keeping the held check below the observed six-minute
background update. The interactive demo retains its six-minute default. Certificate verification,
the identical-fingerprint assertion and the complete 45-second binding lifetime
beyond TLS expiry are unchanged. No background verification feature is disabled.

## Limits

Guarantees apply to the supported QWAC issuer and controlled agent deployment.
External certificate loaders, unmanaged Nginx edits/reloads and operator-owned
credential file changes are outside the transaction. Restore prior selected
inputs before restarting after a rejected update. Same-authority replacement
never expands trust. Local validation/status does not establish revocation,
trusted-list legal qualification or native browser acceptance.

Full-chain caps intentionally reject a historical document whose signed expiry
overruns an intermediate/root deadline. Recovery fails closed with a validation
diagnosis; use the documented offline drain/archive/rebuild procedure for invalid
state. Existing documents already within every chain deadline remain compatible.

Retained TLS leaves are not silently pruned, including expired leaves needed by
established connections. Capacity rejects growth; collection removes only
authenticated historical documents while preserving current/grace/unknown files.
Retirement remains the documented offline drain/stop/archive/rebuild procedure.
Already committed responses and upgraded streams may continue after expiry;
sent bytes cannot be retracted. A TLS leaf's expiry does not close its connection.
No merge, release, authority migration or automatic QWAC renewal is performed.
