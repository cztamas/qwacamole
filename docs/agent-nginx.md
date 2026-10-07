# Supplied-QWAC agent with Certbot and Nginx

Acceptance of `codex/certbot-nginx-agent` is recorded in
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

Go 1.27.1 and authenticated repository access are required for the current private
feature branch. The independently tested production agent ref is
`3170cc6a6ad026362c54ab0a8fb9e5b70a3fb609`, resolved as
`v0.0.0-20261007214159-3170cc6a6ad0`. Use an independent consumer manifest:

```sh
agent_install_dir=$(mktemp -d)
cd "$agent_install_dir"
cat > go.mod <<'EOF'
module example.invalid/qwacamole-agent-install

go 1.27.1

require github.com/cztamas/qwacamole/agent v0.0.0-20261007214159-3170cc6a6ad0
EOF
GOWORK=off GOEXPERIMENT=nojsonv2 GOPRIVATE=github.com/cztamas/qwacamole \
  GOBIN="$agent_install_dir/bin" go install -mod=mod \
  github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent
go version -m "$agent_install_dir/bin/qwacamole-agent"
sudo install -m 0755 "$agent_install_dir/bin/qwacamole-agent" /usr/local/bin/qwacamole-agent
```

This is a real remote installation with no local replacements. Direct
`go install package@ref` currently fails its default-branch deprecation lookup
because the new nested module exists only on the feature branch. The explicit
consumer manifest avoids that lookup and pins both the agent and its real core
dependency. The checked installer is `scripts/check-remote-agent.sh REF`.

For development, from the feature branch:

```sh
make agent-build
```

Install the built binary under `/usr/local/bin` before following the service
commands. Create the supplied inputs and config below before rendering Nginx.

Supply a JSON file with absolute paths (example addresses are production ports):

```json
{
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
  "nginx_config": "/etc/nginx/qwacamole.conf"
}
```

The configured Nginx file must exactly match `render-nginx` output. It is a complete
config, not an include to paste into another configuration. Initially it serves
HTTP-01 challenges and HTTP 404 elsewhere. The agent installs the HTTPS include
after binding publication. `nginx_prefix` and webroot must exist with appropriate
master/Certbot access. The state/runtime directories must be mode 0700. A short
runtime path is required: Unix socket paths above 100 bytes reject at validation.
Do not share the runtime directory or state with another deployment. Persisted
deployment settings must remain identical across restart; changed settings require
the full-stop offline migration described below. Same-path QWAC replacement does
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

Request policy follows the generated Nginx configuration: request buffering stays
enabled, the default maximum client body is 1 MiB, and Nginx's default client-body,
proxy-send and proxy-read inactivity timeouts are 60 seconds. These are inactivity
limits, not a total request-duration limit. The agent allows five seconds for
request headers and thirty seconds between keep-alive requests. Application bodies
and committed response streams have no additional fixed duration deadline in the
agent; cancellation/disconnection and Nginx's inactivity limits still apply.
This permits forwarding a buffered upload when a slow origin applies backpressure
for longer than ten seconds. Binding/gate requests retain a ten-second read
deadline and their small responses have a ten-second write deadline; private
control retains its separate bounded policy. Changing body limits or per-site
timeouts requires configuration support beyond the current fixed template.

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

A Linux systemd service can use:

```ini
[Unit]
Description=Qwacamole controlled Nginx certificate deployment
After=network.target nginx.service

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

Configure your Nginx service to use exactly the prefix/config above, rather than
starting a second master. Create the private state directory before enabling the
service. The systemd example is an operator template; the macOS demonstration
uses owned foreground processes rather than system services.

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

The WebSocket fix changes both the top-level template and immutable HTTPS
generation templates. State created with the earlier agent cannot be reused
unchanged by this version: validation/reconciliation will reject its old config.
Use the full-stop reconstruction above when upgrading an existing demonstration
or deployment; preserve the archived state and supplied credentials. Do not patch
live immutable generations to bypass validation.
