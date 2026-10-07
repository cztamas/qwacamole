# Supplied-QWAC agent with Certbot and Nginx

This feature is under acceptance on `codex/certbot-nginx-agent`; do not treat
historical Caddy evidence as proof of this integration. The production executable
is `qwacamole-agent`, in the single `agent/` module. It contains no synthetic PKI,
ACME issuer, browser driver or Caddy dependencies. The existing development
`qwacamole` executable remains separate.

## Trust and supported deployment

One literal DNS hostname, one local Nginx master, supplied RSA QWAC credentials,
RS256 bindings and exclusive local storage. Linux and macOS are supported by the
process-lock implementation. Development testing currently uses Nginx 1.30.5,
Certbot 5.8.0, Go 1.27.1 and Pebble 2.8.0; final acceptance is tracked in
[the plan](agent-nginx-plan.md). Nginx requires its HTTP SSL and auth_request
modules. `validate` detects missing build flags and modified top-level config.

Bind the TLS leaf actually presented to the browser. Arbitrary CDN TLS termination,
distributed deployment, QWAC enrollment, HSMs and automatic TLS retirement are
unsupported. Strict bindings are default; `firefox_compatibility` explicitly
selects the established compatibility profile. Synthetic devtools authorities
have no legal qualification.

The daemon has two separate listeners: loopback HTTP for binding bytes and the
bodyless availability gate, and a Unix control socket with mode 0600 in a real
0700 runtime directory. Public HTTP cannot deploy certificates, choose paths,
execute commands, inspect status or read keys. Local control callers can invoke
only predefined operations using configuration loaded at daemon startup. Protect
that configuration and credentials from untrusted writers. Running the daemon
as root permits reading Certbot keys and signaling a root Nginx master; only
trusted local operators may access its control socket. Nginx workers need no
access to that socket or the agent state directory.

## Installation and configuration

An immutable remote install command will be recorded after the implemented agent
revision is pushed and independently tested. For development, from the feature
branch:

```sh
make agent-build
bin/qwacamole-agent render-nginx -config /etc/qwacamole/agent.json > /etc/nginx/qwacamole.conf
bin/qwacamole-agent validate -config /etc/qwacamole/agent.json
```

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

Use the ordinary system TLS authorities for public ACME certificates. `tls_roots`
is an optional absolute PEM file for an explicitly configured private TLS authority;
it does not bypass hostname, chain or expiry validation. QWAC trust is separately
supplied and does not grant ordinary TLS trust.

Nginx proxies an ordinary HTTP origin, adds its discovery Link alongside upstream
Link fields, and preserves application body/status/cache headers. Its internal
`auth_request` checks the agent before protected application requests. Expired or
unavailable bindings, unconfirmed deployment and agent failure reject requests.
Bindings are served on `/bindings/`, always uncached and without recursive Link.
`/__qwac_gate` is internal, `/__qwac_control` reserved; private control is never
proxied. Keep these paths out of the application namespace.

## Service and Certbot setup

Run Nginx with this prefix/config and start the agent before initial issuance:

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
