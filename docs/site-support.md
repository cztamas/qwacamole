# Support, limitations and troubleshooting

Qwacamole is an experimental unpublished alpha. There is no stable production
SLA. The [release contract](alpha-release.md#components-and-support) records
tested tools and platforms; a newer dependency or browser is not automatically
validated.

## Supported deployment

Supplied RSA keys (2048 bits or stronger), RS256 signatures, explicit single
authority trust, one literal DNS hostname per state and local exclusive storage.
Caddy supports independent sites/states. The agent owns one dedicated Nginx
master and complete configuration. Native demonstrations run on macOS arm64;
portable integration is tested on Linux amd64. Linux arm64 packages are
cross-compiled only. No Windows or macOS notarization claim.

Automatic QWAC enrollment, EU trusted-list/revocation retrieval, authority
migration, HSMs, distributed state, shared multi-site Nginx and arbitrary CDN
TLS termination are unsupported. Ordinary TLS trust, supplied QWAC authority
validation, legal qualification and native browser acceptance are separate checks.

## Browser profiles

Strict bindings are default and retain the JAdES critical-header declaration.
Firefox 157's tested verifier rejects that declaration. The explicit
`firefox_compatibility` option omits it, while preserving chain, signature,
hostname, bound TLS hashes and expiry checks. It is a compatibility variant,
not proof of strict profile conformance; it is never selected by User-Agent.

Stock Chromium in the recorded native test accepts an expired binding `exp`.
The eleven-case Caddy suite therefore must retain its nonzero result: ten cases
pass and the expired-binding assertion fails. Server-side expiry still closes
new protected responses. Firefox rejects the expired binding and passes the
eleven-case suite. Browser trust/features remain version-dependent.

The [development guide](demos.md) separates synthetic profile trust from real
browser authority lists and legal identity. [Firefox research](firefox.md) is
dated evidence, not a promise about all future releases. No website page label
establishes native qualified identity.

## Troubleshooting

| Symptom | Next step |
| --- | --- |
| Private Go module fetch fails | Use authorized Git access and scoped GOPRIVATE. Public alpha tags/downloads do not exist yet. |
| Caddy JSON module decoding fails | Use the pinned toolchain with GOEXPERIMENT=nojsonv2. |
| Credential validation fails | Inspect key match, supported certificate profile, all hostname SANs, validity, leaf-first chain and independently selected authority. |
| Protected request returns 503 | Inspect expiry, signing/publication errors, state ownership, disk space and deployment readiness. Repair the cause rather than bypassing the gate. |
| TLS works but native identity is absent | Check browser QWAC authority trust and binding profile separately from ordinary TLS. |
| Certbot returns success but new TLS is absent | Inspect deployment.last_error and active/candidate/phase, then reconcile/check. A hook failure may not fail Certbot itself. |
| State is already owned | Stop the conflicting owner or use independent state. Never remove a running owner's lock. |
| Upgrade or restart fails | Preserve complete state, use a compatible binary and the full-stop recovery procedure; do not delete coverage to force readiness. |

Use the [Caddy reference](caddy-plugin.md#troubleshooting),
[agent diagnostics](agent-nginx.md#ordering-failure-retry-and-restart),
[maintenance policy](operations.md) and
[migration/rollback guide](agent-migration.md). Confidential reporting setup
remains described honestly in [SECURITY.md](../SECURITY.md).
