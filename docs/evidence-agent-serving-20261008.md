# Dedicated Nginx serving and upgrade acceptance, 8 October 2026

Starting merged `main`: `4a93ce6c43bf4a728a69625ac7ed567d54db53f1`.
Feature branch: `codex/agent-serving-upgrades`. Repository instructions, current
agent/core code, operational guides, original acceptance and PR-review follow-up
were read before implementation. No releases, release tags, merge, repository-policy or
Caddy-registration changes were performed.

Final Nginx harness: `4f34269cb7e90178b5f963b9bc8925925953fa4e`. Caddy
Firefox/runtime were executed with `61d5942f03443d69d9d2a3a68574df27af40a46b`;
the fresh Caddy Chrome lifecycle uses the browser initialization fix at
`745a21e379c4cb640d910c4806c44bf206beacbf`. The later harness change only adds
a distinct browser-side client address to the Nginx metadata assertion.

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
storage, private socket and restart are exercised as the session user on isolated
HTTP/HTTPS ports 5002/8443. Root-owned production directories, privileged ports
80/443 and systemd service ordering are documented operator examples, not claimed
executed deployment. Certbot and Nginx run as this same isolated owner.

## Requirement mapping

| Requirement | Implementation and evidence |
|---|---|
| 1. Four modules and single-host scope | Root remains stdlib-only; agent has no Caddy/browser/PKI dependencies; full generated config, one master/hostname, private fixed Unix controls. Boundary and independent module checks. |
| 2. Trusted metadata and normal proxying | Nginx replaces forwarding headers, rejects Host with 421, fixes authority/port. Go Rewrite copies trusted metadata and exact raw query/path. Real origin inspection from `127.0.0.2` versus the agent delivery hop `127.0.0.1`, forged headers, secure cookies/URLs, encoded semicolon queries, redirect, status/cache/Link, streams and WebSocket echo; admission/commit tests retained. Local-process trust is explicit. |
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
and repeated serving migration validated; no activation attempted. The
[versioned migration summary](evidence/agent-serving-20261008/versioned-policy-migration.json)
retains the resulting counts and settings.

`make test` exits 0: race tests in root, agent, Caddy and devtools, including
transaction/crash/cache/retention/concurrency checks, followed by dependency
boundaries. `scripts/check-independent-modules.sh` exits 0: all four modules,
`GOWORK=off`, real pins, no replacements. The final distinct-address harness
change additionally passes `GOWORK=off go test -race ./cmd/qwacamole` in devtools.
The production files are unchanged after `72b69f4`.

Fresh Caddy production runtime smoke exits 0 with **22** assertions, using the
independently installed plugin-only binary. Fresh Caddy Firefox integration exits
0 with **11/11** fixtures and native initial, cached-renewed, cached-replaced,
restart and fault-recovery phases. Every phase retains the same isolated trust;
`accept_insecure_certs` is false, certificate overrides are false, and native
qualified UI identifies the synthetic owner. See the redacted
[Firefox summary](evidence/agent-serving-20261008/caddy-firefox.json).

Hosted [CI run 37803778081](https://github.com/cztamas/qwacamole/actions/runs/37803778081)
at `745a21e` completed **success**: workspace/independent races, boundaries,
default/custom real Nginx, merged-main remote installation and interrupted upgrade,
and actual Caddy lifecycle. [Job/step results](evidence/agent-serving-20261008/hosted-ci.json)
are saved. The artifact download returned HTTP 403 from the cloud host; no hosted
artifact contents are claimed. Local runtime summaries below supply the reviewable
evidence.

Fresh Caddy Chrome integration exits **1**, with **10/11** fixtures passing and
only `expired-binding` failing: “BROWSER INCOMPATIBILITY: native verifier accepted
expired exp header.” Chrome 155 therefore retains the same known incompatibility
as the earlier 154 runs. No unrelated failure is accepted. Initial/renewed native
verification, unchanged lifecycle profile/trust, restart, independent refresh and
signing/publication failure recovery pass. An original stream crosses renewal
with its old TLS ID while fresh probes see the new ID. A separate native HTTP/2
connection keeps the same certificate fingerprint through its `16:01:15Z` expiry
and is checked at `16:02:02Z`, beyond one complete 45-second binding lifetime.
See [the Chrome summary](evidence/agent-serving-20261008/caddy-chrome.json).

The early Caddy Chrome attempt failed renewed native verification because TLS
warm-up ran before isolated root-store initialization; it is excluded from
acceptance. Waiting before **any** TLS warm-up fixes the harness race, without
changing trust, verifier flags or any acceptance assertion. The fresh full run
above supplies the accepted evidence.

Final installed-agent Nginx runs now exit 0:

| Run | Assertions | Result summary |
|---|---:|---|
| Default serving/lifecycle/faults | 32 | [nginx-default.json](evidence/agent-serving-20261008/nginx-default.json) |
| Nondefault serving/lifecycle/faults | 35 | [nginx-custom.json](evidence/agent-serving-20261008/nginx-custom.json) |
| Merged-main offline upgrade and complete lifecycle | 36 | [nginx-upgrade.json](evidence/agent-serving-20261008/nginx-upgrade.json) |

Both normal and forged-header requests reach the origin with browser address
`127.0.0.2`, HTTPS protocol and `qwac.example.com:8443`, no `Forwarded`, unchanged
encoded path and semicolon query, secure cookies and the correct absolute origin
redirect. Unsupported authorities receive 421. Default 2 MiB upload rejection is
413; with a 128 MiB maximum, the same upload reaches the origin intact. Custom
settings are 1500 ms client-body/send inactivity, 15 s read inactivity, buffering
off, fixed HTTPS redirects and HSTS `max-age=300; includeSubDomains`. Real stalls
produce Nginx's 408 close or 504; default/nondefault runs preserve slow-origin
uploads beyond ten seconds, committed streams and WebSocket echo. HTTP-01 remains
reachable before and after confirmation; redirects are 503 before confirmation
and then 308 to the fixed authority with escaped path/query retained. HSTS is
absent on HTTP and disabled by default on HTTPS, including suppression of an
origin-supplied preload policy.

Each complete lifecycle verifies old/new signed coverage before TLS staging,
fresh normally verified activation, the original held stream across reload,
independent refresh, stable cached discovery beyond signed expiry, same-authority
QWAC replacement, retained coverage/restart, signing/document/manifest faults,
rollback and zero-exit reload nonactivation. Admission, delayed response commitment
and new WebSocket upgrades fail closed when bindings expire. Capacity refusal
does not stage the unbound candidate or prune prior TLS coverage. Shared core race
checks separately retain crash recovery and coverage beyond leaf expiry/restart.

The upgrade starts the remotely installed merged `4a93ce6` binary and its exact
unversioned templates, issues real Certbot TLS, and fully stops the owned processes.
Missing full-stop assertion rejects. SIGKILL at staged validation and again after
configuration activation leaves startup blocked by the durable intent. Resume and
repeat preserve every artifact and the original `confirming` journal meaning;
real Nginx validation and fresh TLS reconciliation succeed. Two restricted backups
remain. The separate versioned-policy migration preserves all three expired
retained generations without activation. No elapsed-time or listener-drain claim
is used.

Fresh Nginx Chrome and Firefox integrations each exit **0**, with **22** runtime
assertions and three native phases: initial identity, renewed TLS discovered from
the cached response after signed-binding expiry, and same-authority QWAC
replacement through the same cached discovery. Each lifecycle uses one isolated
profile and one unchanged trust digest. Chrome reports root-store version 1000000,
ordinary TLS verification and valid native 2-QWAC binding in every phase. Firefox
reports `secure-etsi`, qualified identity visible, no certificate override and
`accept_insecure_certs: false`; its explicit compatibility profile remains enabled.
See [Chrome](evidence/agent-serving-20261008/nginx-chrome.json) and
[Firefox](evidence/agent-serving-20261008/nginx-firefox.json). Their original held
renewal streams are recorded independently of fresh TLS/native checks.

## Reviewable archive and outcome

[Redacted summaries](evidence/agent-serving-20261008/README.md) retain actual
assertions, configured settings, received origin metadata, version markers,
public fingerprints, unchanged trust, journal/interruption results, installation
metadata, commands and exit statuses. [The command ledger](evidence/agent-serving-20261008/commands.json)
records exact arguments, completion timestamps and harness source per run. No
keys, full profiles, NetLogs, request bodies or credential-bearing logs are
committed. Raw ignored state for these final runs is under
`.local/implemented-nginx-{default,custom,upgrade,chrome,firefox}-20261008`,
`.local/implemented-caddy-chrome-20261008`,
`.local/final-caddy-{runtime,firefox}-20261008`, and `.local/evidence/`.

The requested implementation and exercised acceptance are complete. The dedicated
single-host architecture and existing lifecycle/failure guarantees remain intact.
The required Chrome expired-binding assertion remains nonzero; all other freshly
executed browser fixtures pass. Installation is authenticated while private.
Migration requires an operator-established complete shutdown, and the root/systemd
Linux service example remains unexecuted in this container. No release tags,
releases, merge, visibility/licensing changes or Caddy package registration were
performed. The feature branch and immutable production/harness commits are pushed
for review and remote consumption.
