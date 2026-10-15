# Certbot / Nginx quickstart

Deploy supplied organization credentials on **Ubuntu 24.04 amd64**, with one
hostname and one dedicated Nginx master. Have a reachable HTTP application,
an A record pointing directly to this host's public IPv4 address, and ports 80/443
reachable over IPv4 by clients and public ACME. The generated listeners are
IPv4-only: the hostname must have **no AAAA record**. IPv6 requires a separately
configured and tested deployment.
A shared Nginx master or a CDN terminating TLS elsewhere is unsupported.

You need a valid RSA 2-QWAC leaf-first chain, matching private key and an
**explicitly selected authority PEM**. [Prepare and inspect inputs](byo-credentials.md)
first. Qwacamole does not obtain qualified credentials. Replace the example
hostname, email, upstream and `/path/to/*` inputs below.

## 1. Install

[Download, verify and install the agent package](binary-installation.md) at
`/opt/qwacamole-agent`. On the supported Ubuntu host, install prerequisites:

```sh
sudo apt update
sudo apt install nginx certbot
```

Ubuntu's Nginx includes HTTP SSL/auth_request and Certbot includes webroot support.
Setup uses `/usr/sbin/nginx`, `/usr/bin/certbot` and systemd;
[supported versions](site-support.md) lists tested pairs.

Use a dedicated host. If package installation started the default Nginx service,
inspect it and explicitly stop/disable it before continuing. On a fresh dedicated
host with no other Nginx deployment or Certbot lineages:

```sh
sudo systemctl disable --now nginx.service certbot.timer
```

The qwacamole timer installed below owns this deployment's renewal. Setup detects
existing masters and listener conflicts; it never stops another service.

## 2. Configure

```sh
sudo /opt/qwacamole-agent/bin/qwacamole-agent setup \
  --hostname example.com --upstream http://127.0.0.1:8080 \
  --chain /path/to/issued-chain.pem --key /path/to/signing-key.pem \
  --authority /path/to/selected-authority.pem --email admin@example.com \
  --dry-run
```

Review the planned paths and commands, then repeat without `--dry-run`.
Setup validates inputs and capabilities, creates private files, generates the
agent/Nginx configuration and installs dedicated services and the Certbot hook.
The service identity is root; Nginx workers have a separate owned upload directory.
No full JSON editing or repository clone is needed.

An identical repeat preserves files and binding state. Changed credentials or
operator-edited files are refused: use [replacement and recovery](byo-credentials.md#credentials-in-generated-setups).
Setup does not start services or contact ACME. Strict bindings remain the default.

## 3. Start

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now qwacamole-agent.service qwacamole-renew.timer
sudo certbot certonly --config /etc/qwacamole/certbot.ini --agree-tos \
  --webroot -w /var/lib/qwacamole-acme -d example.com --cert-name example.com \
  --deploy-hook /etc/qwacamole/deploy-hook --no-directory-hooks
```

The agent service starts the dedicated Nginx master. The installed hook publishes
binding coverage before activating TLS. The timer renews this lineage through
its saved hook. Private runtime state is recreated by systemd after boot.

## 4. Verify

```sh
sudo /opt/qwacamole-agent/bin/qwacamole-agent verify -config /etc/qwacamole/agent.json
```

Expected successful output:

```text
PASS HTTPS: ordinary certificate verification succeeded
PASS discovery: signed binding is available at /bindings/current.jose
PASS authority: selected authority validates the supplied signer and signature
PASS TLS binding: binding is current and covers the served TLS certificate
PASS Nginx deployment: dedicated deployment is ready
Ready: example.com serves verified HTTPS and a current signed TLS binding.
Browser acceptance and legal qualification are separate checks.
```

**Setup is complete when verification succeeds.** A failure names the check and
next action; add `-json` for automation. Certbot success alone does not prove a
successful deployment hook. See [operations](operations.md),
[the agent reference](agent-nginx.md), [upgrades](agent-migration.md) and
[browser limitations](site-support.md#browser-profiles).
Try synthetic credentials only in the separate [isolated demonstration](demonstration.md).
