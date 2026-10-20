# Security reporting and scope

The published **0.1.0-alpha.1** release is experimental; it has no production
support guarantee.
Support is best-effort only, with no response-time guarantee. The synthetic
demonstrations do not establish legal qualification, QTSP status, EU Trusted List
registration or production revocation handling. Do not use development test
keys/authorities for real identity claims.

The primary contact for security reports is
[tamas.czank@gmail.com](mailto:tamas.czank@gmail.com).
Do not post private credentials, exploitable details or browser profiles in public
issues. GitHub private vulnerability reporting is enabled; you can also use
this repository's [**Security → Report a vulnerability** flow](https://github.com/cztamas/qwacamole/security/advisories/new).

Include the immutable source ref, Go/Caddy/browser versions, affected
configuration, expected versus observed behavior and a minimal synthetic
reproduction. Redact keys, tokens, owner data and unrelated browsing/network
history. Describe whether ordinary TLS verification, the QWAC issuance gate,
document verification, state recovery or dependency isolation is affected.
Coordinate disclosure and a fix privately with the owner.

For the Certbot/Nginx agent, also identify Nginx/Certbot versions, journal phase,
last confirmed TLS leaf, whether rollback/reconciliation completed, and whether
the issue crosses the loopback delivery/application proxy or private Unix control
boundary. Redact Certbot accounts, private keys and runtime socket credentials.
See [the agent guide](docs/guides/agent-nginx.md) for local permissions, response gates,
independent deploy-hook status and Nginx-worker-aware offline retirement.

Known boundaries: supplied RSA credentials, one literal hostname per state,
local exclusive storage, browser-specific native support, explicit Firefox
compatibility, and no automatic QWAC enrollment/authority migration/distributed
state/HSM integration. Chromium's documented ignored binding-exp behavior remains
visible in tests. See [operations](docs/guides/operations.md) for capacity refusal,
draining, recovery, and browser-facing TLS termination requirements.
