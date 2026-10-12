# Caddy quickstart

Deploy supplied organization credentials with packaged Caddy on **Ubuntu 24.04
amd64**. Have one literal DNS hostname pointing directly to this host, a reachable
HTTP application and ports 80/443 available for public ACME. No upstream CDN may
terminate TLS on a different certificate.

Supply a valid RSA 2-QWAC leaf-first chain, matching private key and an
**explicitly selected authority PEM**. [Prepare and inspect inputs](byo-credentials.md)
first; qwacamole does not enroll with an issuer. Replace the hostname, email,
upstream and `/path/to/*` inputs below.

## 1. Install

[Download, verify and install packaged Caddy](binary-installation.md) at
`/opt/qwacamole-caddy`. It already includes the qwacamole plugin; Go and xcaddy
are unnecessary. The installation uses a new directory and preserves existing
Caddy binaries/services. Setup also needs systemd on the supported Ubuntu host.
[Custom builds with xcaddy](source-installation.md#caddy-with-xcaddy) are an alternative.

## 2. Configure

```sh
sudo /opt/qwacamole-caddy/bin/caddy-qwac qwac-setup \
  --hostname qwac.example.com --upstream http://127.0.0.1:8080 \
  --chain /path/to/issued-chain.pem --key /path/to/signing-key.pem \
  --authority /path/to/selected-authority.pem --email admin@example.com \
  --dry-run
```

Review the plan, then repeat without `--dry-run`. The tool checks credentials,
listeners and configuration, copies private inputs and creates a dedicated root
service with owned writable state. It never stops or replaces an existing service.
An identical repeat preserves files/state; edits require explicit maintenance.

The generated `/etc/caddy-qwac/Caddyfile` is one complete configuration:

```caddyfile
qwac.example.com {
    qwac {
        hostname qwac.example.com
        chain_files /etc/caddy-qwac/chain.pem
        key_file /etc/caddy-qwac/key.pem
        trust_files /etc/caddy-qwac/authority.pem
        acme {
            email admin@example.com
        }
    }
    reverse_proxy 127.0.0.1:8080
}
```

The sole `qwac` directive configures the paired gated issuer and middleware.
Binding lifetime and state location use tested defaults; the service sets
`XDG_DATA_HOME=/var/lib/caddy-qwac`. Use the same single hostname in the site address and `qwac` block.
Do not add a fallback issuer, manual TLS leaf, wildcard or additional hostname.

## 3. Start

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now qwacamole-caddy.service
```

Caddy obtains ordinary TLS through its gated ACME issuer. State and private inputs
belong to the dedicated root service; credentials are not web-readable.

## 4. Verify

```sh
sudo /opt/qwacamole-caddy/bin/caddy-qwac qwac-check \
  --hostname qwac.example.com --authority /etc/caddy-qwac/authority.pem
```

Expected successful output:

```text
PASS HTTPS: ordinary certificate verification succeeded
PASS discovery: signed binding is available at /bindings/current.jose
PASS authority: selected authority validates the supplied signer and signature
PASS TLS binding: binding is current and covers the served TLS certificate
Ready: qwac.example.com serves verified HTTPS and a current signed TLS binding.
Browser acceptance and legal qualification are separate checks.
```

**Setup is complete when verification succeeds.** A failure names the check and
next action; add `--json` for automation.
Server verification is separate from native browser acceptance and legal qualification.

Use [the Caddy reference](caddy-plugin.md) for JSON/custom arrangements and
[operations](operations.md) for renewal, reload and recovery, and
[credential maintenance](byo-credentials.md#credentials-in-generated-setups) for generated setups.
Strict bindings remain default; [Firefox compatibility](site-support.md#browser-profiles)
is an explicit choice. Synthetic credentials belong in the separate
[isolated demonstration](demonstration.md).
