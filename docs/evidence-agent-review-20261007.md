# PR #3 review follow-up, 7 October 2026

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

Production fix: `3170cc6a6ad026362c54ab0a8fb9e5b70a3fb609`, resolved remotely as
`github.com/cztamas/qwacamole/agent v0.0.0-20261007214159-3170cc6a6ad0`.
The devtools manifest pins this actual version, with no replacement. Core remains
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
