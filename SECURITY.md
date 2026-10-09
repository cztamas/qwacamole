# Security reporting and scope

This is an unpublished experimental alpha, with no supported production release
or response-time guarantee. The synthetic demonstrations do not establish legal
qualification, QTSP status, EU Trusted List registration or production revocation
handling. Do not use development test keys/authorities for real identity claims.

Do not post private credentials, exploitable details or browser profiles in public
issues. If GitHub private vulnerability reporting is enabled, use this
repository's **Security → Report a vulnerability** flow. Its availability has not
been verified. Before public publication, the owner must enable that flow or
publish a monitored confidential reporting channel and response policy. Until
then, authorized collaborators should contact the owner through their existing
private channel; do not invent an address or assume an unmonitored inbox.

The release path fails closed until `release/publication.json` records approved
legal/publication policy and either a real confidential URL present here or a
live API check confirms GitHub private vulnerability reporting. That prerequisite
has not been completed by this task. The owner must verify routing/monitoring and
approve the response policy before making source or packages public; no SLA is
inferred. See [the exact launch checklist](docs/alpha-release.md).

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
See [the agent guide](docs/agent-nginx.md) for local permissions, response gates,
independent deploy-hook status and Nginx-worker-aware offline retirement.

Known boundaries: supplied RSA credentials, one literal hostname per state,
local exclusive storage, browser-specific native support, explicit Firefox
compatibility, and no automatic QWAC enrollment/authority migration/distributed
state/HSM integration. Chromium's documented ignored binding-exp behavior remains
visible in tests. See [operations](docs/operations.md) for capacity refusal,
draining, recovery, and browser-facing TLS termination requirements.
