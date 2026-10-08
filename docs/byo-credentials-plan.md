# Bring your own 2-QWAC: assessment and implementation plan

Assessment at `502ff74` (8 October 2026), before implementation:

| Layer | Existing behavior | Gap and narrow change |
|---|---|---|
| Core | Standard-library RSA PKCS#1/PKCS#8 PEM loader, bundled certificates, key match, QNCP-w-gen, binding-only EKU, subject/issuer, QC compliance/web type, hostname, validity and explicit-root verification | Share root completion across integrations; reject weak signing keys, incompatible key usage, malformed/duplicate/conflicting required statements. Test failures with synthetic certificates. |
| Caddy | Explicit single anchor; appends omitted root; validates on provisioning; prepares replacement then atomically publishes complete TLS coverage on activation | Delegate loading to core; preserve configuration/reload and failure transaction. |
| Agent | Explicit single anchor; requires root already in chain; validate/replace-qwac controls; atomic binding publication and restart journal | Delegate loading to core. Add keygen, csr, inspect-qwac and import-qwac to existing production CLI, independent of Nginx configuration. |
| Devtools | Synthetic PKI, native browser trust, transactional/fault/restart tests and Caddy/Nginx acceptance | Retain fixtures and assertions. Add same-key replacement and shared validation consumer coverage. |

Engineering review: preserve all four module boundaries and RS256 support. One
explicit root remains the supported trust model because store ownership/recovery
already binds to one authority. No authority migration, enrollment, system/browser
trust writes, watcher, deployment journal or binding-store redesign is needed.
PEM chains stay leaf-first; root may be omitted and is completed only from explicit
trust configuration. A credential import creates a new private directory and
returns configuration paths; it never modifies active configuration or trust.
Key/CSR creation refuses existing paths and keeps keys local. Caddy reload and
agent replace-qwac remain the activation boundaries; failed candidates leave the
old in-memory material and committed binding usable. Operators retain and restore
the prior on-disk inputs after rejection before restarting.

Validation scope: local profile, key, names, dates and configured chain only.
Production has no independent EU trusted-list/revocation service; existing native
browser verification stays in devtools. Synthetic authorities never enter
production trust automatically. Standards checked against ETSI TS 119 411-5
V2.1.1, EN 319 412-4 V1.4.1 and EN 319 412-5 V2.6.1. Required compliance and
website type statements are distinguished from optional PDS statements.

Execution: implement/test core first; commit/push it and pin its real version in
agent/Caddy; implement CLI and integration tests; push/pin implemented consumers;
run four-module races, boundaries and independent modules, plugin-only build and
real smoke, then fresh sequential Chrome/Firefox demonstrations for both stacks.
Preserve Chrome's known ignored-exp failure separately. Record fresh evidence in
a dated report, review diff, commit, create PR against main, do not merge.
