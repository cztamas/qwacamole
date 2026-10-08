# Dedicated Nginx serving and upgrade acceptance, 8 October 2026

Starting merged `main`: `4a93ce6c43bf4a728a69625ac7ed567d54db53f1`.
Feature branch: `codex/agent-serving-upgrades`. Repository instructions, current
agent/core code, operational guides, original acceptance and PR-review follow-up
were read before implementation. No release, tags, merge, repository-policy or
Caddy-registration changes are authorized or performed.

Production source: `72b69f4e37afee5712a183ea7d2010dfbca0a0d5`, agent version
`v0.0.0-20261008153622-72b69f4e37af`. Core remains
`v0.0.0-20261007193456-3c3fc434d5e4`; Caddy remains byte-identical to
`2c78d367c557736d60c8d5197283ade3a2d35111`. Its independently installed final
consumer uses source `acad18713597b461d5e0a1cefcd47a37f67fd7b7`, version
`v0.0.0-20261008152059-acad18713597`. No replacement appears in installed
production metadata. Devtools pins real retrievable versions, with no replacements.

## Environment and verification scope

Isolated Debian 13 amd64, Go 1.27.1 with `GOEXPERIMENT=nojsonv2`, Nginx 1.28.0
built locally with SSL/auth_request, Certbot 5.8.0, Pebble 2.8.0, Caddy 2.10.2,
xcaddy 0.4.5. Fresh stock Chrome 155.0.8059.39 with genuine PKIMetadata 1798;
Firefox 157.0.1, geckodriver 0.37.1, system NSS certutil.

Private GitHub retrieval required this cloud host's per-command URL rewrite to
its authenticated `.git` route. The first naked private-module retrieval failed
with “could not read Username”; the supported authenticated route succeeds.
This is authenticated private installation, not anonymous public installation.
No credential values were printed or retained in evidence.

The container cannot run browser process sandboxes that need namespace/proc
writes. Chrome uses a local launcher adding `--no-sandbox`; Firefox uses writable
XDG directories and process-scoped `MOZ_DISABLE_*_SANDBOX=1` settings. These are
explicit test-environment limits, with no TLS bypass or verifier-policy change.
The established isolated Chrome root-store/CT-only exception and Firefox explicit
compatibility/test-authority setup remain unchanged within each lifecycle.
Initial Firefox startup failures were diagnosed and do not count as acceptance.
PID 1 is not systemd. Linux foreground processes, renewal, ownership, temporary
storage, private socket and restart are exercised; systemd units are documented
operator examples, not claimed systemd execution.

## Requirement mapping

| Requirement | Implementation and evidence |
|---|---|
| 1. Four modules and single-host scope | Root remains stdlib-only; agent has no Caddy/browser/PKI dependencies; full generated config, one master/hostname, private fixed Unix controls. Boundary and independent module checks. |
| 2. Trusted metadata and normal proxying | Nginx replaces forwarding headers, rejects Host with 421, fixes authority/port. Go Rewrite copies trusted metadata and exact raw query/path. Real origin inspection, forged headers, secure cookies/URLs, encoded semicolon queries, redirect, status/cache/Link, streams and WebSocket echo; admission/commit tests retained. Local-process trust is explicit. |
| 3. Validated serving settings | Typed bytes/timeouts/buffering/redirect/HSTS in `agent/serving.go`; explicit defaults/ranges. Actual default and custom Nginx checks cover body rejection/larger upload, all inactivity timeouts, bootstrap/challenge, fixed redirects and HTTPS-only HSTS. Existing >10s uploads/held streams retained. |
| 4. Formats and upgrades | Explicit input/state/journal/Nginx versions; read-only future rejection and classified changes. Offline `migrate`, restricted synced backup, staged real Nginx validation, startup-blocking durable intent and payload/pointer checks. Merged binary upgrade, kill before/after configuration activation, repeated execution and preserved journal recovery; versioned serving migration preserves three expired retained generations. |
| 5. Installation/operations | Ordinary exact-ref Go install outside checkout/fresh module cache/GOWORK off/no replacements, metadata and installed binary exercised. Linux service/Certbot/permissions/temp-storage/log examples and private monitoring; systemd execution limitation explicit. |
| 6. Lifecycle/native/consumer regression | Full fault/capacity assertions remain; normal verified TLS, original held connection separate from reconnect, retained expired coverage, cache discovery, commit/upgrade rejection. Four-module races, independent pins and boundaries; actual Caddy/Nginx and fresh native checks recorded below. |
| 7. Reviewable result | README, agent guide, config example, migration/operations guide, CI extension and this report; small redacted machine-readable evidence, production/harness refs and commands. |

## Execution ledger

Final production installation:

```sh
scripts/check-remote-agent.sh 72b69f4e37afee5712a183ea7d2010dfbca0a0d5
# Internally, outside checkout with a fresh GOMODCACHE and GOWORK=off:
go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@72b69f4e37afee5712a183ea7d2010dfbca0a0d5
```

Exit 0. Binary metadata contains the exact agent/core versions above and no
replacements or demo dependencies. Baseline exact-ref installation at merged
`4a93ce6` also exits 0. Installed `render-nginx` was exercised outside checkout.
Runtime commands use the independently installed production executable.

Intermediate runtime runs at `acad187` and `0c658a2` passed all default/custom
and upgrade checks; both new interruption points passed. Fresh Nginx Chrome
and Firefox phases passed at those refs. They are intermediate evidence; the
final production migration adds bottom-up backup directory syncing and exact
selected-pointer preservation. Final-ref evidence is recorded separately below.

`agent-migration-policy-check` against final `72b69f4` exits 0 on a fully stopped,
completed actual Nginx state: three retained generations, all three expired,
every TLS/QWAC key and binding byte unchanged, journal unchanged, real `nginx -t`
and repeated serving migration validated; no activation attempted. Raw summary:
`.local/evidence/implemented-policy-migration/result.json`.

Final workspace/independent race, runtime/native command outcomes and redacted
summaries are being collected before final acceptance is recorded. This draft
is not a completion claim.
