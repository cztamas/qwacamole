# Supported versions, browser compatibility and troubleshooting

Qwacamole is an experimental, unpublished alpha. Source installation requires
authorized repository access; authenticated binary candidates are available, but
public release tags and downloads are unavailable.
Support is best-effort only, with no response-time guarantee. Check these
constraints before installing, upgrading or interpreting a browser indicator.

## Versions and platforms

| Component | Tested configuration |
| --- | --- |
| Production agent and Caddy plugin | Exact-source packaged candidates and custom source builds; Go 1.27.1 with `GOEXPERIMENT=nojsonv2`, xcaddy 0.4.5, Caddy 2.10.2, CertMagic 0.24.0 |
| Agent runtime | Linux amd64 (Ubuntu 24.04, Nginx 1.24.0 and Certbot 2.9.0/5.8.0; also Nginx 1.28.0 with Certbot 5.8.0), macOS arm64 (Nginx 1.30.5, Certbot 5.8.0) |
| Native browser demonstrations | macOS arm64, stock Chrome 154 with genuine PKIMetadata 1798, Firefox 157.0.1 and geckodriver 0.37.1 |
| Synthetic ACME demonstration | Pebble 2.8.0; bundled bootstrap supports macOS arm64 only |

Linux arm64 agent builds are cross-compiled only. Windows, macOS notarization
and Caddy's hosted builder are untested. Automated setup targets Ubuntu 24.04 amd64 only;
manual service/platform arrangements require independent evaluation. Newer tools and
browsers need independent evaluation; these versions are compatibility facts,
not automatic recommendations to remain on old security releases.

## Supported deployment

Use RSA keys of at least 2048 bits, RS256 signatures, explicit single-authority
trust, one literal DNS hostname per state and exclusive local storage. Caddy can
run independent sites with separate states. The agent owns one dedicated Nginx
master and complete configuration, with HTTP SSL and auth_request enabled.

Automatic QWAC enrollment, EU trusted-list or revocation retrieval, authority
migration, HSMs, distributed state, shared multi-site Nginx and arbitrary CDN TLS
termination are unsupported. Ordinary TLS trust, supplied-authority validation,
legal qualification and native browser acceptance are separate checks.

## Browser profiles

| Browser/profile | What works and what fails | Explicit settings |
| --- | --- | --- |
| Chrome 154, strict bindings | Qualified identity is displayed when both ordinary TLS and QWAC trust succeed. **The tested verifier accepts an expired binding `exp`**, so it cannot be relied on to enforce binding expiry. Server-side expiry still rejects new protected responses. | Strict JAdES critical-header declaration is the default. No User-Agent selection or production trust override. |
| Firefox 157 | Native qualified identity, chain/signature/hostname/TLS hashes and expiry checks work in the compatibility profile. Its verifier rejects the strict `crit` declaration. | Set `"firefox_compatibility": true` in agent JSON or the bare `firefox_compatibility` switch in the Caddy qwac block. This omits the declaration and does **not** establish strict JAdES conformance. |

Browser QWAC authority lists and feature availability are independent of server
configuration. Neither profile installs browser trust. The [isolated demo](demonstration.md)
uses explicitly synthetic trust only in a disposable profile. A page label or
HTTP success is not native qualified-identity verification. Detailed investigations
and negative test matrices are retained in repository engineering documentation.

## Troubleshooting

| Symptom | Next step |
| --- | --- |
| Private Go module fetch fails | Use authorized Git access and scoped GOPRIVATE; public tags/downloads are unavailable. |
| Caddy JSON module decoding fails | Use the pinned toolchain with GOEXPERIMENT=nojsonv2. |
| Credential validation fails | Check key match, profile, hostname SANs, validity, leaf-first chain and independently selected authority. |
| Protected request returns 503 | Inspect expiry, signing/publication errors, state ownership, disk space and deployment readiness. Repair the cause rather than bypassing the gate. |
| TLS works but native identity is absent | Check browser QWAC authority trust and binding profile separately from ordinary TLS. |
| Certbot succeeds but new TLS is absent | Inspect deployment.last_error and active/candidate/phase, then reconcile/check; a hook failure may not fail Certbot itself. |
| State is already owned | Stop the conflicting owner or use independent state; never remove a running owner's lock. |
| Upgrade or restart fails | Preserve complete state and use a compatible binary and full-stop recovery; do not delete coverage to force readiness. |

Continue with [Caddy diagnostics](caddy-plugin.md#troubleshooting),
[agent recovery](agent-nginx.md#ordering-failure-retry-and-restart),
[maintenance](operations.md), or [migration and rollback](agent-migration.md).
For confidential issues, follow the actual [security policy](../../SECURITY.md).
