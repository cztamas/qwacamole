# Sustained-use hardening evidence — 7 October 2026

Final production source: **`234f8d7e20a5863f3f2929e50a8e11f24650064e`** on
`codex/public-alpha-hardening`, based on
`8dd1dd7ee2e3cb96fa005938e98db6355a675c00`. Devtools timing controls are at
`4ef2caa2dd38e7bc74e8f025718f1603a90ca34a`; subsequent changes are documentation.
**All engineering acceptance gates passed with the exact documented Chrome
ignored-exp incompatibility retained.**
Historical browser reports remain unchanged. No merge, tag, release, visibility,
license choice, Caddy registration or hosted-builder publication was performed.

## Implemented behavior and migrations

Application responses preserve their own Link/cache headers and advertise stable
`/bindings/current.jose`. That uncached endpoint returns one complete verified
committed snapshot; immutable SHA-256 documents remain separate recovery/history
artifacts. GET/HEAD agree, other methods reject and binding responses have no
recursive Link. Legacy manifests gain additive `discovery_url` without losing
coverage. Legacy numeric durations and new readable duration strings are tested.
Previously cached immutable Links require invalidation; grace cannot extend exp.

Activation stages the candidate signer with every retained/stored/new TLS leaf,
signs and independently verifies the complete set, writes/fsyncs and reads back
its immutable document, then commits/fsyncs the manifest. Only then do live signer,
state and configuration generation switch. Ordinary failed post-rename commits
restore previous manifest bytes; rollback storage errors are explicit. Crashes
before commit recover old state, after durable commit recover new state, and
rename/fsync interruption permits a complete old or new commit. The issuer cannot
return a TLS replacement before this gate. Older generations cannot roll back
credentials. Separate issuer/handler module contexts share stored coverage through
the exclusive owner, including startup with TLS storage but empty binding state.

Defaults: lifetime **1h** (formerly 45s), refresh margin **lifetime/3**, immutable
document grace **24h**, enforced **256 retained leaves** and **1 MiB documents**.
Lifetimes below 3s reject; refresh margin must be 1s through half the lifetime.
Scheduling follows actual signed expiry, with notifications, bounded exponential
retry/jitter and fail-closed outage recovery. Structured diagnostics expose
QWAC/binding expiry, next refresh, errors, counts, bytes and storage growth.

GC holds the reader/publication lock and authenticates hash, signature, authority,
host/profile and signed-expiry/modification-time grace. Current, staged, grace-
protected, unknown, corrupt and symlink files are not deleted. TLS retention is
separate. Capacity refuses additions rather than removing coverage of potentially
held connections, including those outliving TLS NotAfter.

Pinned Caddy 2.10.2 `modules/caddyhttp/server.go:638–694` exposes HTTP server
ConnState/ConnContext/shutdown hooks; its HTTP/3 server is separate at 616–632.
Server-side Go TLS ConnectionState exposes peer certificates, not the selected
local server leaf. These hooks do not establish safe per-leaf retirement across
protocols/reloads/issuer-only use. There is no automatic TLS pruning. The tested
procedure fully stops the owning process, archives the whole binding directory,
keeps Caddy's normal TLS storage, and reconstructs current coverage on restart.
Capacity refusal can ultimately cause downtime. See [operations](guides/operations.md)
for migrations, grace, replacement, draining and recovery.

## Portable validation

Toolchain: Go **1.27.1**, `GOEXPERIMENT=nojsonv2`, xcaddy **0.4.5**, Caddy
**2.10.2**, CertMagic **0.24.0**, Pebble **2.8.0**. Commands ran at repository root:

```sh
make build
make test
make plugin-build
bin/qwacamole plugin-smoke -state .local/public-alpha-corrected-20261007 \
  -plugin-binary bin/caddy-plugin
git diff --check
```

All-module race tests and dependency boundaries exited **0**; final log:
`.local/public-alpha-final-tests.log`. The plugin-only smoke exited **0**, all
**22 checks**, state `.local/public-alpha-corrected-20261007/plugin-smoke-599230200/`,
console `.local/public-alpha-corrected-smoke.log`. Checks cover real challenge-
validated ACME renewal, streaming/header preservation, GET/HEAD/method/path/no-store
controls, conditional 304, redirect 307, real WebSocket 101/echo, a real caching
intermediary past signed expiry and across renewal/QWAC replacement, automatic
owned-artifact collection, reloads, exclusive ownership, rejected reload recovery,
process restart, file serving, two independently protected sites beside an
unprotected site, custom error-route 418/body/headers with paired middleware,
and full-stop/offline retirement retaining TLS storage and rebuilding coverage.

Meaningful core/lease regressions exercise absent stored TLS during candidate
replacement failure, separate module contexts, signing/verification/document-write/
readback/manifest/post-rename failures, subprocess crash boundaries, concurrent
readers/renewal/reload/refresh/collection, authenticated GC ownership/grace/orphans,
enforced capacities/legacy recovery, stable snapshot copies and a real network
cache. A refresh outage proves unchanged commits, expired document/protected
response refusal, bounded retries and recovery without reload. No assertion is
skipped; a sandbox-restricted listener check passed with approved socket access.

Hosted Linux validation passed at engineering head
`4ef2caa2dd38e7bc74e8f025718f1603a90ca34a`:
[run 37597323566](https://github.com/cztamas/qwacamole/actions/runs/37597323566).
All three module race suites, boundaries, plugin-only build and all 22 runtime
checks passed. The job scopes checkout's existing read-only credential to this
private repository for Go's separate VCS cache and removes it afterward; it does
not print or commit credentials. Portable CI includes the new regressions.

## Exact immutable remote installation

Core commit `d576a2929a0fc6105fd61668b7b31c18c1a20809` was pushed and Go remotely
resolved **`v0.0.0-20261007082742-d576a2929a0f`**. `caddy/go.mod` pins that real
version; GOWORK=off nested-module tidy/race tests passed.

```sh
# Existing authorized SSH authentication selected per-process.
GIT_CONFIG_COUNT=1 \
GIT_CONFIG_KEY_0=url.ssh://git@github.com/.insteadOf \
GIT_CONFIG_VALUE_0=https://github.com/ \
GOCACHE="$PWD/.tools/gocache" \
  scripts/check-remote-install.sh 234f8d7e20a5863f3f2929e50a8e11f24650064e
```

Exited **0**, fresh external module/VCS cache, `GOWORK=off`, no replacements,
workspace or devtools dependency/module. Installed plugin:
**`v0.0.0-20261007085522-234f8d7e20a5`**; core:
**`v0.0.0-20261007082742-d576a2929a0f`**. Binary: `bin/caddy-remote`. Authoritative
resolution, build metadata and module inventory: `.local/evidence/remote-install/`;
console `.local/public-alpha-accepted-remote-build.log`. This proves authenticated
private installation, not anonymous public distribution. After both sequential
browser runs completed, the installed binary passed runtime validation:

```sh
bin/qwacamole-final plugin-smoke -state .local/public-alpha-remote-runtime-20261007 \
  -plugin-binary bin/caddy-remote > .local/public-alpha-remote-runtime.log 2>&1
```

Exit **0**, all **22 checks**. State:
`.local/public-alpha-remote-runtime-20261007/plugin-smoke-3170208152/`.
Its `evidence/result.json` names the absolute `bin/caddy-remote` binary, lists all
22 checks and records no error. This includes the full-stop/offline retirement
regression and real cached discovery across signed expiry and QWAC replacement.

## Final fresh Chromium acceptance

```sh
bin/qwacamole integration-test -state .local/public-alpha-final-native-20261007 \
  > .local/public-alpha-final-chrome.log 2>&1
```

State: `.local/public-alpha-final-native-20261007/integration-20261007-105538/`.
Stock Chrome **154.0.8037.98**, copied genuine **PKIMetadata 1798**, native loaded
root-store version **1000000**. Exit **1**: exactly **10 passing cases and one**
`expired-binding` failure,
`BROWSER INCOMPATIBILITY: native verifier accepted expired exp header`.
All lifecycle checks and real TLS hostname/expiry, trust separation, signature,
TLS hash, QWAC hostname/profile and CT-only controls passed. The known strict
assertion and nonzero result remain visible; no unrelated failure was accepted.

Native initial/renewed/restarted/failure-recovery bindings verified to the exact
QWAC leaf/root, with signed owner **Qwacamole Synthetic Organization TEST**, HU,
NTRHU-TEST-0001. QWAC leaf SHA-256:
`62248663c2403737b90c76e015ec7f15c950c26d2433b97fa89ec939812f9425`; root:
`416ccfbbc5a2f3816fe96aca37a0e43e1f368077df33f711f29f0ae61785b967`.
This is native NetLog chain/binding proof, not page text or a newly captured
native-panel image. `chrome-native-audit.json` records exact native leaf/root,
root-store version and bound TLS IDs for all five positive lifecycle cases.

Automatic renewal took **65.106s**. Old TLS:
`813c1fcfac6fa1220a2608ee791bc98867c333b437eb6f395b776478c44994f3`;
replacement: `617048d7ffcb90410dffb68c49bf285f0b6798a39e7f712b10842a984c984ee2`.
The old HTTP/1 connection remained authenticated and both leaves were bound.
The same browser profile/trust then natively accepted the replacement.

The continuous HTTP/2 test held the replacement leaf through **09:02:50Z**
NotAfter until **11:03:37.030926 +02:00**, **47.031s after expiry**, beyond a whole
45s binding lifetime. Initial/final TLS IDs match; **19** native binding events
accepted the same leaf. Raw HTTP/2 sessions **208** (application) and **267**
(binding) began together and carried 40/19 requests for **394.649/394.623s**,
respectively, with no replacement sessions. `continuous-http2-audit.json` records
the raw session proof. New handshake expiry validation remained enforced.

Independent refresh advanced iat **1791363849 → 1791363879**, exp
**1791363894 → 1791363924**, changed immutable bytes/identifier, retained exact
TLS coverage and kept `/bindings/current.jose` stable. Both signing and
publication failures rejected replacement TLS while prior HTTPS/binding stayed
usable; automatic retry recovered and the native verifier accepted new TLS
`dac00d390396c4339546f15c132211d965800ee20b6655d1da51a2d20b9f591a`.
Raw `rotation.json`, `refresh-test.json`, `failure-test.json`, `integration.json`,
`browser-suite.json`, all result JSON/NetLogs remain under the state above.

## Final fresh Firefox acceptance

Started only after final Chrome exited, with the corrected production binary:

```sh
bin/qwacamole-final firefox-integration -state .local/public-alpha-final-native-20261007 \
  > .local/public-alpha-final-firefox.log 2>&1
```

State: `.local/public-alpha-final-native-20261007/firefox-integration-3138453836/`.
Stock Firefox **157.0**, geckodriver **0.37.1**. Exit **0**, all **eleven fixture
checks passed**, including expired binding and critical-header rejection. Native
chrome-context UI snapshots show `secure-etsi`, visible qualified text
`Qualified as specified in Regulation (EU) 2024/1183.`, and the exact expected
QWAC identity/fingerprint in all six lifecycle cases. They are native browser
security UI data, not application page text. Every run used the same lifecycle
profile and unchanged imported trust, `accept_insecure_certs: false`, and no
certificate override. `firefox-native-audit.json` independently audits these facts.

Initial and cached-initial owner: **Alpha Firefox TEST**; QWAC SHA-256:
`efd49a7bb2808da2ba8cfdb3124e662ab76bcbfd133e1ef91f4e284aebc9e186`.
TLS renewed automatically in **68.196s**, from
`434f4df0b8d17d7f018a0088ed8d82a3b19016f5a341c498129d3d297ca46b11` to
`a1530b2015ca31983271d52eafe4b35d6ec2e5b3ed9eb1f558263864dab13916`.
The original held connection kept its old leaf; both were covered. The same
profile natively accepted the new leaf through the cached discovery Link.

A real intermediary cached the application response for **600s**, longer than
its original **45s** signed binding lifetime. At cache ages **71s** (renewed TLS)
and **100s** (replacement QWAC), both responses were cache hits, preserving body
SHA-256 `feb11ba9cc313d5e8e63106cb18e7a7827b78266a4d145bc29b541ec7384bd34`
and Link `</bindings/current.jose>; rel="tls-certificate-binding"` unchanged.
Original binding exp was **1791364303**; fetched renewed binding iat/exp were
**1791364330/1791364375**, replacement **1791364361/1791364406**. Independent
verification covered the actual renewed TLS leaf, and native Firefox accepted
both after the original signed expiry.

The cached replacement case displayed **Alpha Firefox Replacement TEST**, QWAC
`729fe91be4fa78fcfabeaaa9594fe515265d8b5d7dd4b69a68fdaa27b16bac80`, while
keeping the same renewed TLS leaf. Restart recovered that identity and coverage.
Independent refresh advanced iat **1791364330 → 1791364360**, exp
**1791364375 → 1791364405**, keeping the discovery URL and TLS set unchanged.
Both injected signing/publication failures refused replacement TLS while prior
HTTPS/binding remained valid. Automatic retry recovered; the native security UI
then accepted TLS `da5ece33fd95d2699715a64308451df6255685383296eade96646c8dac54c4f7`.
Raw native snapshots, cache results, lifecycle and all eleven fixture results
remain under `evidence/` in the state directory above.

## PR review disposition

The two review findings on [PR #1](https://github.com/cztamas/qwacamole/pull/1)
were checked against their reviewed commit and current code:

- [Wildcard policy bypass](https://github.com/cztamas/qwacamole/pull/1#discussion_r4199044209)
  is relevant. The PR branch already contains its targeted fix at
  `f6261bde56fc8a536c8f8340be8e540915b2e46e`. This hardening branch additionally
  checks Caddy's effective subjects (including resolved placeholders/IDNA) and
  rejects decoded external certificate managers. Race and actual-Caddy checks
  prove first-matching wildcard policies cannot bypass the issuer gate.
- [Stale bundle during reload](https://github.com/cztamas/qwacamole/pull/1#discussion_r4199044199)
  does not apply: reload reads the standalone leaf/intermediate, while the bundle
  is consumed by a later file-server configuration and rebuilt before that use.
  Repeated real smoke and replacement/restart checks pass that path.

No merge or GitHub review reply was sent. PR #1's original branch is distinct
from the pushed `codex/public-alpha-hardening` feature branch recorded here.

## Intermediate failures and corrections

These are diagnostic history, not substitutes for the final runs:

- Initial 21-check smoke `plugin-smoke-1415605627` passed; the new offline check
  later exposed separate Caddy module contexts. Hosted `37595527410` showed a
  handler activating empty binding state while the issuer had a stored TLS leaf.
  Activation now gathers stored leaves from every live owner session; focused
  separate-context and actual-Caddy offline regressions pass unchanged.
- Hosted `37595163239` first failed private core-module authentication. Scoped
  read-only CI authentication fixed its separate Go VCS-cache resolution.
- Hosted `37596557417` renewed unusually early under randomized Pebble ARI.
  Cache tests now explicitly cross signed expiry and collection grace instead
  of assuming renewal elapsed long enough. Assertions remain intact.
- Shared lifecycle/failure helpers now select the explicit Firefox compatibility
  verifier only when its state marker exists; strict standard mode is default.
- Earlier sequential native runs `integration-20261007-102928` and
  `firefox-integration-18250276` passed their expected browser outcomes before
  the startup correction. They are preserved as intermediate raw evidence;
  final acceptance uses the corrected runs above.

## Release scope and owner gates

Guides/comments, complete Caddyfile examples, operations/migration/recovery,
[contribution instructions](../CONTRIBUTING.md), [security policy](../SECURITY.md),
[draft release notes](release-notes-alpha.md) and the
[concrete alpha checklist](alpha-release.md) are prepared. Repository inspection
still reports **PRIVATE** and **no license**.

Remaining engineering limits: one literal hostname per independent owner, RSA/
RS256, supplied credentials and exclusive local storage; no automatic TLS leaf
retirement, enrollment, HSM, distributed storage or authority migration. Bind the
actual browser-facing TLS leaf; arbitrary CDN/TLS termination is unsupported.
Strict bindings remain default; Firefox's explicit compatibility mode omits crit.
Chromium's ignored-exp gap remains visible. Synthetic authorities confer no
legal qualification.

The owner must decide license/visibility/version/support scope, activate a monitored
confidential reporting channel, publish root `vX.Y.Z` then nested `caddy/vX.Y.Z`
with the correct real core dependency, verify anonymous installation/runtime
after public publication, and register `github.com/cztamas/qwacamole/caddy` with
Caddy if desired. Hosted-builder Go/nojsonv2 compatibility must be tested separately.
Those publication/account actions remain outstanding by design.
