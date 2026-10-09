# Certbot / Nginx quickstart

The production agent manages supplied credentials and binding-first TLS
deployment for **one dedicated Nginx master and one hostname**. It cannot be
inserted into a shared multi-site Nginx configuration.

## 1. Check prerequisites and install

Use a Linux host with Nginx's HTTP SSL and auth_request modules, Certbot,
Go **1.27.1**, authorized private-repository Git access, and a reachable HTTP
application on `127.0.0.1:8080`. Ports 80/443 must be available. Your real DNS
hostname must point directly here; public ACME HTTP-01 must reach port 80.
Run privileged deployment commands from an administrator shell.

Supply a current RSA 2-QWAC chain, matching unencrypted key and separately
selected authority PEM. See [preparation and inspection](byo-credentials.md).
All `example.com`, `/path/to/*`, application and Nginx binary paths below
are placeholders to review and replace. Qwacamole does not obtain qualified
certificates.

```sh
(
  agent_install_dir=$(mktemp -d)
  cd "$agent_install_dir"
  GOWORK=off GOEXPERIMENT=nojsonv2 GOPRIVATE=github.com/cztamas/qwacamole \
    GOMODCACHE="$agent_install_dir/modules" GOBIN="$agent_install_dir/bin" \
    go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@bb911a8f69a8743faab70967232ff166c828c4b7
  go version -m "$agent_install_dir/bin/qwacamole-agent"
  "$agent_install_dir/bin/qwacamole-agent" version -json
  sudo install -m 0755 "$agent_install_dir/bin/qwacamole-agent" /usr/local/bin/qwacamole-agent
)
git clone https://github.com/cztamas/qwacamole.git
cd qwacamole
git checkout main
```

The clone supplies the canonical configuration example; installation itself
uses the immutable ref and no local replacements. Public tags and package assets
are **not available yet**. [Packaged installation](agent-nginx.md#packaged-agent)
is a future procedure after publication.

## 2. Prepare the dedicated deployment

From the repository root, prepare these paths. The agent/control directories and
keys are private. Nginx's prefix and ACME webroot must be traversable by workers.

```sh
sudo install -d -m 0700 /etc/qwacamole /var/lib/qwacamole/example.com /run/qwacamole-example
sudo install -d -m 0755 /var/lib/nginx/qwacamole /var/lib/acme
sudo install -m 0600 /path/to/issued-chain.pem /etc/qwacamole/qwac-chain-with-root.pem
sudo install -m 0600 /path/to/signing-key.pem /etc/qwacamole/qwac-key.pem
sudo install -m 0600 /path/to/selected-authority.pem /etc/qwacamole/qwac-root.pem
sudo install -m 0600 config/agent.json.example /etc/qwacamole/agent.json
```

Edit `/etc/qwacamole/agent.json` before continuing. The
[complete shared JSON example](../config/agent.json.example) specifies every
deployment path, hostname, Certbot lineage, listener/probe address and application.
Replace `example.com` everywhere, select your real Nginx binary path, and
review serving/body/redirect policy. Keep one hostname per state. The selected
trust file is distinct from ordinary TLS roots. Use
[the reference](agent-nginx.md#installation-and-configuration) for each field.

Run the following from a root shell (`sudo -i`), after providing valid inputs:

```sh
umask 077
qwacamole-agent render-nginx -config /etc/qwacamole/agent.json > /etc/nginx/qwacamole.conf
qwacamole-agent validate -config /etc/qwacamole/agent.json
/usr/sbin/nginx -p /var/lib/nginx/qwacamole -c /etc/nginx/qwacamole.conf
qwacamole-agent serve -config /etc/qwacamole/agent.json
```

The generated file is the complete top-level configuration. Start only its
dedicated master. Leave the agent running; use another root terminal for issuance.
The agent-controlled include selects immutable TLS generations after publication.

## 3. Connect Certbot's deploy hook

Create `/etc/qwacamole/deploy-hook`, root-owned, with this content:

```sh
#!/bin/sh
exec /usr/local/bin/qwacamole-agent certbot-hook -config /etc/qwacamole/agent.json
```

Then issue ordinary TLS for your real single hostname:

```sh
chmod 0700 /etc/qwacamole/deploy-hook
certbot certonly --webroot -w /var/lib/acme -d example.com --cert-name example.com \
  --deploy-hook /etc/qwacamole/deploy-hook --no-directory-hooks
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
```

Use Certbot's webroot mode, not its Nginx installer. Do not add an independent
certificate-install hook or point Nginx at the moving Certbot live paths.
A failed hook may leave Certbot's exit code successful: status must show
`ready: true`, `phase: committed`, and no deployment error for the new
candidate. `check` makes a fresh verified TLS connection and must succeed.

## 4. Verify discovery and sustained operation

```sh
curl --fail --show-error --include https://example.com/
curl --fail --show-error --include https://example.com/bindings/current.jose
certbot renew --no-directory-hooks
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
```

Replace the hostname and keep ordinary TLS verification enabled. The application
advertises the stable binding Link; binding responses are uncached JOSE with no
recursive Link. After a real renewal, confirm a changed active deployment and
the new served TLS leaf. Configure the usual renewal timer and separate owned
services using the [service guide](agent-nginx.md#service-and-certbot-setup);
its systemd units are untested examples. Recreate private `/run` state on boot.

Native browser identity is separate from `check`. See
[browser profiles](site-support.md#browser-profiles) and
[isolated demonstrations](demos.md) before interpreting qualified UI.

## 5. Replace credentials and recover

Import/inspect the new same-authority credential into a new directory and select
its chain/key together using the [atomic pointer procedure](byo-credentials.md#replacement-and-rejected-update-recovery).
Keep startup-configured paths stable, then:

```sh
qwacamole-agent replace-qwac -config /etc/qwacamole/agent.json
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
```

No Nginx reload is needed for QWAC-only replacement. Repair reported errors;
a rejected candidate preserves the prior valid signer. Restore its on-disk inputs
before a restart. Configuration upgrades require the
[full-stop migration and rollback procedure](agent-migration.md), with all old
workers/connections stopped and complete state preserved.
