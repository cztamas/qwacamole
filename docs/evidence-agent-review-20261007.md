# PR #3 review follow-up, 7–8 October 2026

This report supplements the [original acceptance](evidence-agent-nginx-20261007.md).
It does not attribute new Nginx coverage to the earlier Caddy WebSocket test.

## Findings and corrections

The [inline review](https://github.com/cztamas/qwacamole/pull/3#discussion_r4212094938)
and the separately supplied review identify the same missing Nginx upgrade
headers. Both are relevant. The complete template now maps the incoming Upgrade
header to an appropriate Connection header; the application location explicitly
uses HTTP/1.1 and forwards both headers. This follows the official
[Nginx WebSocket guidance](https://nginx.org/en/docs/http/websocket.html), including
compatibility with the CI's Nginx 1.24.

The body deadline concern is also relevant and is now reproduced, rather than
left as a hypothetical follow-up. Go's
[Server.ReadTimeout](https://pkg.go.dev/net/http#Server) covers the body, including
reads delayed by origin backpressure. The application route clears that read
deadline before entering its reverse proxy. The five-second header timeout and
non-application/control deadlines remain bounded. Nginx retains its default
[request buffering and proxy inactivity limits](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_request_buffering)
and [1 MiB client body limit](https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size).
The guide now states this policy explicitly. This does not claim that all slow
client uploads previously failed after ten seconds.

The third finding accurately describes an intentional scope boundary. The agent
owns one dedicated Nginx master and its complete configuration. Supporting a
shared multi-site master would require a separate design; weakening exact-config
validation would permit unmanaged routes to bypass the binding gate. No such
scope expansion was made. The guide and README now make the boundary prominent.
Because the fix changes both generated templates, existing state requires the
documented full-stop reconstruction; old immutable generations cannot be reused
unchanged or patched live.

## Reproduction and local runtime proof

Tools: Go 1.27.1 (`GOEXPERIMENT=nojsonv2`), Nginx 1.30.5, Certbot 5.8.0,
Pebble 2.8.0, macOS arm64. Each command owns and stops its isolated services.

```sh
bin/qwacamole nginx-smoke -state .local/review-before-20261007 \
  -agent-binary bin/qwacamole-agent-remote
# Before applying the fixes: the independently installed a73695c agent.

bin/qwacamole nginx-smoke -state .local/review-fixed-20261007
# After rebuilding the production agent and harness with the fixes.
```

Before: `.local/review-before-20261007/nginx-smoke-110804979/`, exit 1.
The WebSocket handshake returned HTTP **400**. A 64 MiB request directly through
the loopback application proxy and a default-buffered 1 MiB request through
Nginx both returned **502** after the origin paused consumption for twelve
seconds. The agent log records a body read `i/o timeout` on the buffered Nginx
request. JSON response evidence records status, elapsed time and expected hash.

After: `.local/review-fixed-20261007/nginx-smoke-3278007118/`, exit 0,
**28 checks**. The real TLS → Nginx → agent → origin WebSocket handshake returns
**101**, preserves the application preload and binding discovery Link, validates
the handshake accept value and echoes a masked client text frame back through
both proxy hops. The 64 MiB direct request returns **200** after 12.073 seconds;
the 1 MiB Nginx-buffered request returns **200** after 12.006 seconds. Both origin
byte counts and SHA-256 hashes exactly match the sent bodies.

The full run also verifies normal Certbot renewal, independent binding refresh,
the original held TLS stream across reload, cached discovery after signed expiry,
credential replacement, restart, storage/signing/activation faults, rollback,
actual expiry and capacity refusal. A new upgrade attempted after actual signed
expiry returns **403**. A focused agent race test additionally delays an origin
101 response, makes the binding unavailable while it waits, and verifies **503**
with no upgrade or protected cache headers at the response-commit gate.

Raw evidence is ignored under the run directories: `evidence/result.json`,
`nginx-websocket.json`, `agent-backpressure-upload.json`, `nginx-buffered-upload.json`
and `agent.log`. The CI Nginx smoke executes these new checks; its prior passing
WebSocket test only covered Caddy.

## Real remote module

Initial production fix: `3170cc6a6ad026362c54ab0a8fb9e5b70a3fb609`, resolved remotely as
`github.com/cztamas/qwacamole/agent v0.0.0-20261007214159-3170cc6a6ad0`.
At that stage devtools pinned this actual version, with no replacement. Core remains
`v0.0.0-20261007193456-3c3fc434d5e4`; Caddy remains
`v0.0.0-20261007200511-2c78d367c557`.

```sh
scripts/check-remote-agent.sh 3170cc6a6ad026362c54ab0a8fb9e5b70a3fb609
scripts/check-independent-modules.sh
scripts/check-boundaries.sh
```

The fresh-cache, outside-workspace remote installation exits 0 and its binary
metadata contains the exact agent/core versions above. Console evidence:
`.local/review-remote-agent-20261007.log`. Production changes contain only the
agent; no shared-core, Caddy, credential, trust or binding-profile changes were
needed.

The final harness also ran the **complete 28-check suite** with that remotely
installed production binary, exit 0:

```sh
bin/qwacamole nginx-smoke -agent-binary bin/qwacamole-agent-remote \
  -state .local/review-nginx-remote-final-20261007
```

Raw final run: `.local/review-nginx-remote-final-20261007/nginx-smoke-2878585548/`;
console `.local/review-nginx-remote-final-20261007.log`. This includes all signing,
storage, reload, expiry, upgrade rejection, restart and capacity checks after the
deterministic refresh-observation correction.

All four independently pinned module race suites pass with `GOWORK=off` and no
replacements (`.local/review-independent-races-20261007.log`). Production
dependency boundaries pass (`.local/review-boundaries-20261007.log`). Agent and
devtools workspace race suites also pass, including the new delayed-upgrade test.

## Harness timing

The first native Chrome rerun passed initial native verification and renewal but
failed the independent-refresh observation assertion: the new 24 seconds of
upload checks plus browser startup delayed the observation until Certbot's next
call had already replaced the single-leaf binding with an overlap binding.
Raw run: `.local/review-nginx-chrome-20261007/nginx-smoke-1128897920/`, exit 1.
This run does not count as successful native acceptance. Slow uploads now run
after the independent-refresh/renewal observation. A subsequent Firefox run
(`.local/review-nginx-firefox-final-20261007/nginx-smoke-3891751001/`, exit 1)
exposed the other side of the same observation race: Certbot's first normal call
was already due, as recorded alongside Pebble's ARI suggested window in
`certbot-logs/letsencrypt.log`. It renewed before the first scheduled refresh.
The harness now waits for the independent refresh before making any renewal call.
It still requires later publication/expiry, identical
single-leaf coverage, distinct renewed TLS and uninterrupted original streaming;
no acceptance assertion was removed.

## Fresh native verification of the fixed remote agent

Both successful runs use `bin/qwacamole-agent-remote` installed at the exact
production ref above. They create new isolated profiles, retain unchanged trust
through renewal/replacement, and preserve ordinary TLS verification.

```sh
bin/qwacamole nginx-browser-integration -native-browser chrome \
  -agent-binary bin/qwacamole-agent-remote \
  -state .local/review-nginx-chrome-final-20261007
bin/qwacamole nginx-browser-integration -native-browser firefox \
  -agent-binary bin/qwacamole-agent-remote \
  -state .local/review-nginx-firefox-verified-20261007
```

Chrome: `.local/review-nginx-chrome-final-20261007/nginx-smoke-1056424109/`,
exit 0, **18 lifecycle checks plus three native checks**. Chrome 154.0.8037.98
loads injected root-store version 1000000. Initial, renewed-cached-after-expiry
and replacement-cached-after-expiry binding END events (sources 227, 168 and 160)
each record `is_valid_2qwac_binding:true`; the harness matches the expected QWAC
leaf/root and deployed TLS leaf. Owner changes from `Nginx Agent TEST` to
`Nginx Replacement TEST` without profile/trust changes. This successful run uses
the pre-renewal observation correction; production is identical to the final
agent, and the later harness correction only waits for refresh before Certbot.

Firefox: `.local/review-nginx-firefox-verified-20261007/nginx-smoke-249454170/`,
exit 0, **18 lifecycle checks plus three native checks**, using the final staged
refresh observation. Firefox 157.0.1 records `accept_insecure_certs:false`,
`connection:secure-etsi`, `qualified_visible:true`, and
`certificate_overridden:false` in all three native results. The native owner and
QWAC fingerprint match the expected original/replacement credential; its TLS
fingerprint changes on renewal. Both runs also pass the new real WebSocket echo
and slow-origin upload checks.

These are positive regression checks of the changed Nginx integration. The
previously recorded stock Chrome ignored-binding-expiry incompatibility remains
unchanged and visible in its full Caddy negative suite; this follow-up does not
claim to fix it or rerun unrelated browser negatives.

## Ubuntu upload-buffer directory correction

[Hosted run 37691951916](https://github.com/cztamas/qwacamole/actions/runs/37691951916)
at `bf3f16c82d30ba9c506f968aa1a74efb8b2020b8` passed workspace/independent races,
boundaries, the actual **Nginx 1.24.0** WebSocket upgrade/frame echo, independent
refresh, automatic renewal, held-stream coverage and the 64 MiB agent upload.
The 1 MiB buffered Nginx upload then returned HTTP **500**: the package's absolute
default temporary path `/var/lib/nginx/body/0000000001` was not writable by the
isolated runner. The raw `nginx/error.log` records `Permission denied`.
Artifacts are retained in `.local/review-ci-20261007-failed-artifacts/`; console
`.local/review-ci-20261007-failed.log`. Caddy runtime was skipped in this failed run.

The generated complete configuration now explicitly sets `client_body_temp_path`
under its own Nginx prefix. Nginx manages this directory with its own permissions;
the guide explains master/worker access and the temporary request bodies. This
avoids writing a distribution's shared system directory and preserves the real
buffered upload, its size limit and the twelve-second slow-origin assertion.
No sudo workaround, larger in-memory buffer or skipped test was introduced.

Final production agent: `072fa57bb58b1171fc21cf241c5822af018df3d4`, version
`v0.0.0-20261007220001-072fa57bb58b`. Core and Caddy pins remain unchanged. Earlier
native evidence above is explicitly for `3170cc6`; only the Nginx temporary-path
directive changes between those production refs.

The final ref installs from a fresh remote consumer with no replacements, exit 0
(`.local/review-remote-agent-20261008.log`). All four independent module race
suites and production dependency boundaries pass again with this pin, logs
`.local/review-independent-races-20261008.log` and
`.local/review-boundaries-20261008.log`.

Both native integrations were then rerun with the **final remotely installed
072fa57 agent** and the final harness, sequentially:

```sh
bin/qwacamole nginx-browser-integration -native-browser chrome \
  -agent-binary bin/qwacamole-agent-remote -state .local/review-final-chrome-20261008
bin/qwacamole nginx-browser-integration -native-browser firefox \
  -agent-binary bin/qwacamole-agent-remote -state .local/review-final-firefox-20261008
```

Both exit 0, each with **18 lifecycle checks and three native checks**:

| Browser | Final raw run | Native observations |
|---|---|---|
| Chrome 154.0.8037.98 | `.local/review-final-chrome-20261008/nginx-smoke-2645877469/` | Initial, renewed and replacement binding accepted; exact expected QWAC/TLS; same profile and trust |
| Firefox 157.0.1 | `.local/review-final-firefox-20261008/nginx-smoke-1484077698/` | Initial, renewed and replacement native qualified status/owner; no certificate override; same profile and trust |

Both also pass WebSocket echo and the twelve-second slow-origin body/hash checks
with the new owned Nginx temporary directory. The same trust, expiry and native
evidence requirements described above apply; no browser or trust-policy changes
were made for this correction.

The **complete final 28-check** Nginx smoke also exits 0 with that remote binary:

```sh
bin/qwacamole nginx-smoke -agent-binary bin/qwacamole-agent-remote \
  -state .local/review-final-remote-nginx-20261008
```

Raw run `.local/review-final-remote-nginx-20261008/nginx-smoke-3810142252/` and
console `.local/review-final-remote-nginx-20261008.log` include all fault, rollback,
expiry/upgrade rejection, restart and capacity checks. Every owned process is
stopped; no demo listener remains on 8443, 18081, 14000 or 15000.

## Final hosted CI

[Run 37693473249](https://github.com/cztamas/qwacamole/actions/runs/37693473249)
passes at exact source commit `29e6f33096d85563c0060a9fd6647a32f57ca125`.
Workspace races, independently pinned races, dependency boundaries, tool builds,
**28 real Nginx checks** and **22 Caddy runtime checks** all pass. Downloaded
`result.json` files report `<nil>` errors. The Nginx log identifies **1.24.0
(Ubuntu)** and records the 1 MiB body buffered successfully into the owned
`nginx/client_body_temp/0000000001` path, rather than `/var/lib/nginx/body`.

Raw artifact download: `.local/review-ci-final-20261008-artifacts/`:
`ci-nginx/nginx-smoke-3709785150/` and `ci/plugin-smoke-2594785230/`. Full log:
`.local/review-ci-final-20261008.log`; exact source/job metadata:
`.local/review-ci-final-20261008.json`.

The subsequent evidence commit records these results and removes obsolete
checksums for the superseded agent version; it changes no production code, test
code or dependency selections. No merge, tag, release or trust-store change was
made.
