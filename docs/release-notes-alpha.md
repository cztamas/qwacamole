# qwacamole 0.1.0-alpha.1 — proposed, unpublished

This is a review draft, not a release announcement. Version, licence, visibility,
security reporting and publication require owner approval. The
[maintainer checklist](alpha-release.md) defines the exact future source/tag
sequence. Finalize the approved generic notes before selecting E; the publication
script appends verified C/E source SHAs and manifest/run identity without editing
or retagging that source. No release tags have been created here.

The root core provides strict supplied RSA 2-QWAC validation, local key/CSR
preparation support, RS256 TLS bindings and durable lifecycle policy. The
installable Caddy module gates ACME TLS activation and protected response
commitment. The production qwacamole-agent manages one dedicated Nginx instance
and Certbot deploy hook, independently of development authorities/browsers.

Credential key/CSR, inspection and import are available through the agent CLI for
both integrations. Same-authority replacement validates and publishes complete
TLS coverage before changing the active signer. Rejected candidates preserve
working state. Cached discovery, expiry-driven independent refresh, bounded
retry, capacity refusal, crash/restart recovery and held-connection coverage
remain part of the lifecycle. Rejected delayed streaming proxy responses cancel
upstream work while completing a clean bodyless 503 over HTTP/1.1 and HTTP/2.

Agent packages target Linux amd64, Linux arm64 and macOS arm64, containing only
the production executable and legal/install documents. Source and target identity,
exact toolchain/experiment and SHA256s are recorded in the manifest. `version`
works without config/credentials. Nginx, Certbot and Caddy are external packages;
Caddy installation uses xcaddy. Linux arm64 is cross-compiled only. Runtime and
native-browser combinations are recorded in the dated preparation report, not
inferred from compilation. No Windows, notarization or hosted Caddy builder claim.

This first alpha is experimental. Scope remains supplied RSA/RS256, explicit
trust, one literal hostname per independent state, exclusive local storage,
same-authority replacement and browser-facing TLS coverage. It does not establish
EU trusted-list qualification, production revocation handling or browser trust.
No QWAC enrollment, HSM, distributed state, authority migration, shared multi-site
Nginx or arbitrary CDN TLS termination is included. Strict bindings are default;
Firefox compatibility is explicit. Stock Chromium's expired-exp assertion stays
nonzero; all other fixture and lifecycle checks must pass. Firefox rejects expiry.

Breaking alpha changes must document migration and rollback before release.
Current state/config migration remains additive and versioned; use the existing
full-stop migration for incompatible changes, preserving complete private state,
TLS history and recovery generations. Do not rebuild journals or drop coverage.
See [agent operations](agent-nginx.md), [migration](agent-migration.md),
[Caddy operations](operations.md) and [credential handling](byo-credentials.md).

Current executable installation refs are core 8e62f4b8c340, Caddy 9322bd566e82 and
agent bb911a8f69a8. They are authenticated private installations and are not
proposed release tags. Final published source identities will be core C and nested
agent/Caddy E, with nested requirements pointing to the real root tagged version.
The package manifest must agree with E and that dependency. Anonymous availability
is a post-publication gate; no private installation result substitutes for it.
