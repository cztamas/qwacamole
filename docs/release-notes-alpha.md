# qwacamole 0.1.0-alpha.1

Experimental alpha, approved under the standard MIT license in
[PR #14](https://github.com/cztamas/qwacamole/pull/14). Security reports use
`tamas.czank@gmail.com` or GitHub private vulnerability reporting; support is
best-effort with no response guarantee. The verified release identity appended
below records the exact core and agent/Caddy sources, manifest and candidate run.

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

Agent packages target Linux amd64, Linux arm64 and macOS arm64, with templates,
installation information, a non-overwriting installer and notices. A separate
Linux amd64 package includes Caddy 2.11.7 and the plugin. The manifest records
exact source/core/plugin/dependency/toolchain identities and checksums. Linux
arm64 is cross-compiled only; runtime and native-browser evidence is recorded
separately. No Windows, notarization or hosted Caddy builder claim.

Ubuntu 24.04 amd64 supports dedicated fresh setup through agent `setup` or Caddy
`qwac-setup`, generated private configuration/services and clear verification
commands. Setup requires supplied credentials and selected authority, preserves
strict defaults, and refuses changed files/credentials on repeat. No Go or clone
is needed for packaged installation. Nginx/Certbot remain external prerequisites.

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
See [agent operations](guides/agent-nginx.md), [migration](guides/agent-migration.md),
[Caddy operations](guides/operations.md) and [credential handling](guides/byo-credentials.md).

Install using the [packaged installation guide](guides/binary-installation.md)
or [source/xcaddy guide](guides/source-installation.md). Go consumers request
`v0.1.0-alpha.1` for each module; Git tags are `v0.1.0-alpha.1`,
`agent/v0.1.0-alpha.1` and `caddy/v0.1.0-alpha.1`. GitHub's automatic source
archives follow the root/core tag; the nested module tags and binary manifest
identify the separate agent/Caddy source. Devtools has no supported release tag.

The reviewed dependency baseline is Caddy 2.11.7, CertMagic 0.25.6 and Go 1.27.2
with explicit `GOEXPERIMENT=jsonv2`. It replaces the vulnerable escaped-path
matcher and HTTP/3 QPACK baseline; patched transitive dependencies and the Go
security release are checked independently. The packaged-binary routing
regression denies both encoded-path case variants without configuring a QWAC
gate. Certificate events remain subscribed for the handshake gate despite the
new upstream event filtering. No binding/storage/journal migration is introduced.
