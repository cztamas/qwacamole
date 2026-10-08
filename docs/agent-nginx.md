# Supplied-QWAC agent with Certbot and Nginx

The merged single-host agent baseline is recorded in
[the dated report](evidence-agent-nginx-20261007.md), including real remote
installation, hosted CI and fresh native checks for both integrations. The production executable
is `qwacamole-agent`, in the single `agent/` module. It contains no synthetic PKI,
ACME issuer, browser driver or Caddy dependencies. The existing development
`qwacamole` executable remains separate.
Subsequent PR review fixes and their separate regression evidence are recorded in
[the review follow-up](evidence-agent-review-20261007.md).

## Trust and supported deployment

One literal DNS hostname, one local Nginx master, supplied RSA QWAC credentials,
RS256 bindings and exclusive local storage. Linux and macOS are supported by the
process-lock implementation. Development testing currently uses Nginx 1.30.5,
Certbot 5.8.0, Go 1.27.1 and Pebble 2.8.0; final acceptance is tracked in
[the plan](agent-nginx-plan.md). Nginx requires its HTTP SSL and auth_request
modules. `validate` detects missing build flags and modified top-level config.
This is a dedicated Nginx instance owning one hostname and the complete top-level
configuration. Existing multi-site configurations and arbitrary custom includes
are unsupported; this module cannot be added to an existing shared Nginx master.
Fresh native checks use Chrome 154.0.8037.98 / PKIMetadata 1798 and Firefox
157.0.1; exact commands, trust setup and raw-evidence locations are in
[the dated report](evidence-agent-nginx-20261007.md).

Bind the TLS leaf actually presented to the browser. Arbitrary CDN TLS termination,
distributed deployment, QWAC enrollment, HSMs and automatic TLS retirement are
unsupported. Strict bindings are default; `firefox_compatibility` explicitly
selects the established compatibility profile. Synthetic devtools authorities
have no legal qualification.

The daemon has two separate listeners: loopback HTTP for binding bytes, the
bodyless availability gate and a proxy to the configured application, and a Unix control socket with mode 0600 in a real
0700 runtime directory. Public HTTP cannot deploy certificates, choose paths,
execute commands, inspect status or read keys. Local control callers can invoke
only predefined operations using configuration loaded at daemon startup. Protect
that configuration and credentials from untrusted writers. Running the daemon
as root permits reading Certbot keys and signaling a root Nginx master; only
trusted local operators may access its control socket. Nginx workers need no
access to that socket or the agent state directory.

## Installation and configuration

Go 1.27.1 and authenticated access are required while the repository is private.
The implemented production source is
`72b69f4e37afee5712a183ea7d2010dfbca0a0d5`. Ordinary exact-ref installation works
now that `agent/` exists on `main`; no consumer-manifest workaround is required:

```sh
agent_install_dir=$(mktemp -d)
cd "$agent_install_dir"
GOWORK=off GOEXPERIMENT=nojsonv2 GOPRIVATE=github.com/cztamas/qwacamole \
  GOMODCACHE="$agent_install_dir/modules" GOBIN="$agent_install_dir/bin" \
  go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@72b69f4e37afee5712a183ea7d2010dfbca0a0d5
go version -m "$agent_install_dir/bin/qwacamole-agent"
sudo install -m 0755 "$agent_install_dir/bin/qwacamole-agent" /usr/local/bin/qwacamole-agent
```

Use your authorized HTTPS credential helper or SSH URL rewrite. This is
**authenticated private-repository installation**, not evidence of anonymous
public installation. `scripts/check-remote-agent.sh REF` runs outside the checkout
with a fresh module cache, `GOWORK=off`, no replacements, and inspects binary
metadata. The installed binary is also used in actual Nginx acceptance.
For development on the feature branch, use `make agent-build`.

Install the built binary under `/usr/local/bin` before following the service
commands. Create the supplied inputs and config below before rendering Nginx.

Supply a JSON file with absolute paths (example addresses are production ports):

```json
{
  "config_version": 1,
  "hostname": "example.com",
  "state_directory": "/var/lib/qwacamole/example.com",
  "runtime_directory": "/run/qwacamole-example",
  "qwac_chain_files": ["/etc/qwacamole/qwac-chain-with-root.pem"],
  "qwac_key_file": "/etc/qwacamole/qwac-key.pem",
  "qwac_trust_file": "/etc/qwacamole/qwac-root.pem",
  "certbot_lineage": "/etc/letsencrypt/live/example.com",
  "binding_listen": "127.0.0.1:18081",
  "http_listen": "0.0.0.0:80",
  "https_listen": "0.0.0.0:443",
  "probe_address": "127.0.0.1:443",
  "application": "http://127.0.0.1:8080",
  "challenge_webroot": "/var/lib/acme",
  "nginx_binary": "/usr/sbin/nginx",
  "nginx_prefix": "/var/lib/nginx/qwacamole",
  "nginx_config": "/etc/nginx/qwacamole.conf",
  "serving": {
    "max_request_body_bytes": 1048576,
    "client_body_timeout": "60s",
    "upstream_send_timeout": "60s",
    "upstream_read_timeout": "60s",
    "request_buffering": "on",
    "redirect_https": true
  }
}
```

The configured Nginx file must exactly match `render-nginx` output. It is a complete
config, not an include to paste into another configuration. Initially it serves HTTP-01 challenges. Other HTTP requests return 404 with
redirects disabled, or uncached 503 with redirects enabled until HTTPS deployment
is confirmed and bindings are available. The agent installs the HTTPS include
after binding publication. `nginx_prefix` and webroot must exist with appropriate
master/Certbot access. The state/runtime directories must be mode 0700.
The generated config keeps buffered request-body temporary files under
`nginx_prefix/client_body_temp`, rather than a package's shared system directory.
Nginx creates it during configuration validation/startup; its master needs
permission to create/manage the directory and its worker must be able to write
there. Keep the prefix traversable by the worker. These temporary files contain
application request bodies and belong to Nginx, outside the agent's private state.

A short runtime path is required: Unix socket paths above 100 bytes reject at validation.
Do not share the runtime directory or state with another deployment. Persisted deployment settings are versioned.
Compatible serving changes use the full-stop migration below without reconstructing
state or deleting retained coverage. Same-path QWAC replacement does
not change deployment settings.

Run the configuration, service and Certbot commands below from a root shell
(`sudo -i`). Prepare the example directories before rendering/starting services:

```sh
umask 077
install -d -m 0700 /etc/qwacamole /var/lib/qwacamole/example.com /run/qwacamole-example
install -d -m 0755 /var/lib/nginx/qwacamole /var/lib/acme
```

Place the supplied QWAC chain/key/root and `agent.json` under `/etc/qwacamole`
with root ownership and mode 0600. Render the complete Nginx config only after
those inputs are present. HTTP challenge files must be readable by the Nginx
worker; the webroot above is traversable and Certbot writes the public tokens.
Keep the agent state/config/keys private; do not make them worker-readable.

```sh
/usr/local/bin/qwacamole-agent render-nginx -config /etc/qwacamole/agent.json > /etc/nginx/qwacamole.conf
/usr/local/bin/qwacamole-agent validate -config /etc/qwacamole/agent.json
```

Use the ordinary system TLS authorities for public ACME certificates. `tls_roots`
is an optional absolute PEM file for an explicitly configured private TLS authority;
it does not bypass hostname, chain or expiry validation. QWAC trust is separately
supplied and does not grant ordinary TLS trust.

Nginx adds its discovery Link alongside upstream Link fields and forwards the
application through the agent's `/application/` proxy to the configured HTTP
origin. This extra local hop supplies a response-commit gate: Nginx's internal
`auth_request` checks availability at request admission, and the agent checks
again before forwarding response headers or a protocol upgrade. An upstream
response delayed past binding expiry is rejected with 503 and `no-store`, without
its protected body/cache headers. Already committed streaming responses continue;
subsequent requests must pass the gate again. Application body/status/cache headers
are otherwise preserved. Expired or unavailable bindings, unconfirmed deployment
and agent failure reject requests.
WebSockets use explicit upstream HTTP/1.1 and forwarded Upgrade/Connection headers
through both local proxy hops, including on Nginx 1.24. The availability gate also
applies to new upgrades. An established tunnel may continue after binding expiry,
just like an already committed application stream.

## Client metadata and serving policy

The path is **browser → dedicated Nginx TLS boundary → loopback agent → HTTP
application**. Nginx accepts only the configured hostname and supported port
(case-insensitive hostname). HTTP also accepts the hostname without a port and
its HTTP listener port; HTTPS uses the public authority, allowing an omitted
`:443` only when the external HTTPS port is 443. Unsupported authorities receive
421. `public_https_port` selects the external port when port mapping differs from
`https_listen`; it must describe the same directly terminated TLS service.

Nginx discards `Forwarded`, replaces `X-Forwarded-For` with `$remote_addr`, replaces
`X-Forwarded-Host` and application `Host` with the configured public authority,
and sets `X-Forwarded-Proto: https`. It does not append any browser-supplied chain
or accept another reverse proxy's claimed IP/protocol. The agent uses
`ReverseProxy.Rewrite`, leaves `Forwarded` absent, and deliberately copies these
three trusted headers instead of reporting the local HTTP hop through
`SetXForwarded`. Applications can use them for secure cookies and absolute URLs;
encoded paths and raw query strings are preserved through both hops. Origin
responses retain their headers, cookies, status, bodies, streaming and upgrades,
with the availability checks at admission and response commitment.

**Local processes are trusted.** The loopback delivery listener does not
authenticate Nginx. Any process that can reach it can supply the same metadata;
a loopback peer alone is not Nginx identity. Use a host without untrusted local
tenants and restrict origin access to trusted local callers. The private 0600
control socket is a separate permission boundary and cannot be reached through
application or binding requests. This deployment does not support an upstream
TLS terminator or a trusted-forwarder IP list.

`serving` is typed operator configuration, never request-controlled:

| JSON setting | Omitted value | Explicit values |
|---|---|---|
| `max_request_body_bytes` | 1048576 (1 MiB) | 0 means unlimited; 1 through 1099511627776 bytes (1 TiB) |
| `client_body_timeout` | `60s` | Inactivity between client body reads; whole milliseconds, `1ms` through `24h` |
| `upstream_send_timeout` | `60s` | Inactivity while Nginx sends to the agent; same range |
| `upstream_read_timeout` | `60s` | Inactivity between Nginx reads from the agent; same range |
| `request_buffering` | `on` | `on` buffers before forwarding, spilling under the dedicated prefix; `off` streams with backpressure |
| `redirect_https` | false | true enables fixed-authority HTTP 308 redirects after confirmed deployment |
| `public_https_port` | HTTPS listener port | 1–65535; 0 explicitly selects the listener port; 443 omits the port from the authority |
| `hsts` | disabled | Object requiring explicit `max_age_seconds`: 0–63072000; optional `include_subdomains` defaults false |

Timeout zero/unlimited values reject. These are **inactivity limits**, not total
request-duration limits. A periodic stream can continue indefinitely. With
request buffering off, Nginx can record 408 and close an incomplete upload
without returning a response body. Upstream timeouts can terminate a committed
stream, or return 504 if headers have not committed. The agent keeps a five-second
request-header deadline and thirty-second keep-alive idle limit, clears the
application-body read deadline, and applies no total application-response timeout.
Binding/gate requests retain separate ten-second deadlines; private control uses
its own five-second read and two-minute write limits. Origin backpressure beyond
ten seconds remains supported. Send/read settings govern the **Nginx-to-agent**
hop: origin stalls propagate through the agent's transport and backpressure.

HTTP-01 routes are reserved with `^~` and remain available during bootstrap,
redirects and binding outages. Redirects preserve escaped path and query and use
only the configured public authority. Neither incoming Host nor forwarding
headers choose their destination. An unavailable/unconfirmed deployment returns
503 instead of redirecting to unusable protected HTTPS.

HSTS appears only on HTTPS responses, including binding and error responses.
The generated configuration owns this header and hides an origin's HSTS header.
No `preload` option is supported or enabled. `include_subdomains` covers every
subdomain in the browser, so enable it only when all those services support HTTPS.
Browsers retain HSTS after server rollback/removal; reverting the binary or
configuration cannot erase a stored policy. An explicit `max_age_seconds: 0`
over working verified HTTPS requests clears policy only for browsers that receive
it. HSTS may make an HTTP bootstrap or rollback unavailable to previous visitors;
plan that recovery before enabling it.

Bindings are served on `/bindings/`, always uncached and without recursive Link.
`/__qwac_gate` is internal, `/__qwac_control` reserved; private control is never
proxied. Keep these paths out of the application namespace.

## Service and Certbot setup

Run Nginx with this prefix/config and start the agent before initial issuance:

Configure the owning Nginx service to use these paths, or stop its existing master
before a manual start. Ensure ports 80/443 are available; start only one master.

```sh
/usr/sbin/nginx -p /var/lib/nginx/qwacamole -c /etc/nginx/qwacamole.conf
/usr/local/bin/qwacamole-agent serve -config /etc/qwacamole/agent.json
```

Use separate Linux services for the dedicated master and agent; do not reuse the
system's multi-site `nginx.service`. Example
`/etc/systemd/system/qwacamole-nginx.service`:

```ini
[Unit]
Description=Dedicated Qwacamole Nginx master
After=network.target

[Service]
Type=simple
User=root
UMask=0077
ExecStart=/usr/sbin/nginx -p /var/lib/nginx/qwacamole -c /etc/nginx/qwacamole.conf -g 'daemon off;'
ExecReload=/usr/sbin/nginx -p /var/lib/nginx/qwacamole -c /etc/nginx/qwacamole.conf -s reload
Restart=on-failure
RestartSec=5
KillMode=control-group
TimeoutStopSec=120

[Install]
WantedBy=multi-user.target
```

`/etc/systemd/system/qwacamole-agent.service`:

```ini
[Unit]
Description=Qwacamole binding and deployment agent
Wants=qwacamole-nginx.service
After=network.target qwacamole-nginx.service

[Service]
Type=simple
User=root
RuntimeDirectory=qwacamole-example
RuntimeDirectoryMode=0700
UMask=0077
ExecStart=/usr/local/bin/qwacamole-agent serve -config /etc/qwacamole/agent.json
Restart=on-failure
RestartSec=5
TimeoutStopSec=120

[Install]
WantedBy=multi-user.target
```

Nginx starts its HTTP-only bootstrap before the agent. On agent restart, readiness
stays closed until reconciliation validates coverage and freshly served TLS.
Nginx can start with retained HTTPS files while the agent is down; protected
application requests fail closed until the agent returns. If Nginx restarts after
a failed reconciliation, explicitly run `reconcile` and inspect private status.
Certbot's renewal service should order `After=qwacamole-agent.service` and
`Wants=qwacamole-agent.service`; invoke the same saved deploy hook and monitor
its result independently. Install/enable after supplying credentials and rendering
configuration:

```sh
systemctl daemon-reload
systemctl enable --now qwacamole-nginx.service qwacamole-agent.service
systemctl enable --now certbot.timer
journalctl -u qwacamole-agent.service -u qwacamole-nginx.service
# Generated Nginx logs (keep root-readable and rotate under your retention policy):
tail -f /var/lib/nginx/qwacamole/error.log
```

Protect request-body temporary storage as well as keys: the root master must
create/chown its body directory, and its worker needs write/traverse access under
the prefix. Do not point it at the agent's 0700 state. Monitor available disk
space; temporary buffered bodies can exceed memory and contain private uploads.
Do not log request bodies, Authorization/Cookie values or credentials in hooks.
Nginx's standard request/error logs may contain URLs; keep them private and avoid
putting credentials in URLs. The agent's structured logs/status contain no keys
or request bodies. The Linux runtime tests exercise foreground ownership, real
renewals, temporary upload files, socket permissions, failure and restart. The
selected environment's PID 1 is not systemd, so these unit examples have **not**
been validated by running systemd services.

Create an executable root-owned deploy hook:

```sh
#!/bin/sh
exec /usr/local/bin/qwacamole-agent certbot-hook -config /etc/qwacamole/agent.json
```

Use `certonly --webroot`, with a single hostname and explicit lineage name:

```sh
certbot certonly --webroot -w /var/lib/acme -d example.com --cert-name example.com \
  --deploy-hook /etc/qwacamole/deploy-hook --no-directory-hooks
certbot renew --no-directory-hooks
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
```

Certbot stores the hook for later renewals. Use the normal Certbot renewal timer.
The hook validates documented `RENEWED_LINEAGE` and `RENEWED_DOMAINS` against the
configured single-host lineage and submits a bodyless local deployment request.
The daemon reads `fullchain.pem` and `privkey.pem` once and validates hostname,
key match, validity and chain before publication.

For manual validation, `status` must show `ready: true` and `phase: committed`,
and `check` must succeed using a fresh normally verified TLS connection. Open
`https://example.com` in a browser that trusts the supplied ordinary TLS and QWAC
authorities. Chrome: site controls → **Connection is secure** → **EU Qualified
Status**, then compare the displayed owner/country with the supplied QWAC.
Firefox: padlock → **Connection secure**, then compare the owner and qualification
text. Browser trust and native verification are separate from agent `check`.
After normal renewal, require a changed `deployment.active`, run `check` again,
and freshly navigate with the same browser profile/trust. The isolated devtools
commands in the README automate these steps with explicitly short test lifetimes.

Do not use Certbot's Nginx installer, an additional certificate-install hook, or
point Nginx at Certbot's moving `live/` paths. The generated configuration uses
only agent-controlled immutable generations. An ordinary Nginx reload cannot
select a newly renewed Certbot certificate by itself. Review preexisting hook
and service automation before adopting this deployment.

Certbot documents that a failed hook need not make its overall exit nonzero.
Monitor agent deployment status independently; `ready` alone may still describe
the prior usable deployment. Alert on `deployment.last_error` and inspect
`deployment.phase`, `active`, `candidate`, `previous` and timestamps.
[Certbot hook documentation](https://eff-certbot.readthedocs.io/en/stable/using.html#renewing-certificates).

## Ordering, failure, retry and restart

The journal records prepared intent, bound candidate, activation, reload,
confirmation and committed deployment. The binding includes all retained leaves
before any staged generation can be selected. Generation files and the HTTPS
include are staged completely, synced and installed immutably. The include
pointer switches atomically; Nginx validates and reloads. A fresh normally verified
TLS connection must present the expected candidate before success is recorded.
Filesystem publication and Nginx reload are separate operations; there is no
claim that the whole deployment is one atomic transaction.

Validation/reload/confirmation failures restore the prior include, validate/reload
it and confirm the prior leaf. Extra bound candidates remain covered. If rollback
cannot be confirmed, readiness stays closed and the journal retains intent for
reconciliation. A disconnected control caller does not cancel necessary rollback.
On restart, readiness starts closed until journal, selected files, binding coverage
and the actually served certificate agree. Successful reload exit alone is not
accepted as confirmation.

If every retained TLS leaf has expired, or rollback can only restore an expired
prior leaf, the journal records `needs_deploy` and keeps readiness closed. There
is no usable prior HTTPS deployment to claim in this state. Obtain a fresh valid
Certbot candidate and retry `deploy`; its binding must still publish before
activation, and all expired leaves remain retained. `reconcile` reports this
condition as an error instead of claiming readiness or deleting coverage.

After correcting credentials, disk capacity, Nginx config or process state:

```sh
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent reconcile -config /etc/qwacamole/agent.json
qwacamole-agent deploy -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
```

Manual deploy uses the same fixed Certbot lineage and validation as the hook.
Replace supplied QWAC leaf/key files safely, then invoke `replace-qwac`; same-authority
validation and transactional publication preserve complete TLS coverage. Authority
migration is unsupported. Never edit deployment.json or the include pointer to
force readiness. Before an initial successful deployment there is no prior TLS
service to restore; HTTP challenges remain available and protected HTTPS stays
unavailable.

If both the daemon's loaded QWAC credentials and all retained TLS certificates
have expired, first supply valid same-authority QWAC credentials and restart the
agent to load them. Readiness remains closed until a fresh valid Certbot lineage
is deployed through the normal publication/confirmation path. An expired old
deployment cannot provide a usable rollback target.

## Monitoring, backup and retirement

Defaults match Caddy: 1h signed lifetime, proportional advance refresh margin,
24h authenticated immutable-document grace, 256 retained TLS leaves and 1 MiB
maximum binding. Config fields `binding_lifetime`, `refresh_before` and
`document_grace` accept duration strings; count/size limits are explicit numeric
fields. Short lifetimes belong in devtools only. Maintenance uses actual signed
expiry with bounded exponential retry/jitter, fail-closed expiry, collection and
structured diagnostic events.

Monitor QWAC/binding expiry, next refresh, publication errors, retained count,
current bytes, total storage and deployment errors. Check independently with
`status` and `check`; these diagnostics do not replace native browser acceptance.
Back up the complete private state directory (journal, generations, binding
manifest and immutable artifacts), supplied QWAC credentials and configuration,
plus Certbot account/lineage state. Exclude runtime sockets. Restore consistent
snapshots and reconcile before readiness; do not restore a manifest without its
referenced artifacts.

A capacity refusal preserves required coverage and may cause renewal failure and
eventual downtime. Automatic TLS retirement is intentionally unsupported. TLS
expiry does not prove that Nginx no longer holds authenticated connections.

For offline retirement, stop the Certbot renewal timer and agent; fully stop the
owning Nginx master and every old worker, including workers retained by graceful
reloads. A graceful quit may wait indefinitely on held streams; enforce your
operator-approved drain deadline and verify all owning processes/connections have
ended. Only then archive the complete agent state and reconstruct a new empty
state, regenerate the HTTP-only config, start Nginx/agent and submit the current
valid Certbot lineage through `deploy`. Restart the renewal timer after verified
activation. Never prune binding coverage or generations while any Nginx worker
can still serve a held connection.

For upgrades and compatible serving changes, use
[the versioned migration procedure](agent-migration.md). Unlike offline TLS
retirement, migration preserves every retained certificate, binding document and
recoverable generation. Do not reconstruct state to make a new template pass.
