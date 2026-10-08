# Lifecycle gap assessment and engineering plan

Assessment against merged main `43c8e16` on 8 October 2026. Existing evidence in
`evidence-byo-credentials-20261008.md` is inherited, not fresh lifecycle acceptance.

Existing shared manager serializes signed/independently verified immutable
publication and manifest commit, retains TLS indefinitely, authenticates collection,
and refuses capacity growth. One shared expiry-driven worker serves both
integrations. Caddy issuer publication precedes CertMagic persistence; generation
leases prevent older reloads from selecting older signers. Agent uses its existing
prepared/bound/activating/reloading/confirming journal and fresh verified probe,
with rollback and interrupted-operation reconciliation. Existing synthetic tests
cover transaction crashes, failed publication, reload overlap and collection.

Concrete gaps to close:

1. Signing/verification caps `exp` at QWAC leaf expiry only. Cap at every supplied
   QWAC chain certificate, retaining latest-TLS expiry behavior for overlap.
2. Caddy discovers stored TLS during provisioning but can cache/serve it before
   first HTTP activation, leaving maintenance dormant on a quiet cold restart.
   Use CertMagic's existing cached-certificate event to activate/start maintenance
   and its abortable pre-handshake event to enforce stored coverage. Preserve
   ordinary event forwarding, issuer gating and TLS-ALPN challenge behavior;
   `cert_obtained` remains insufficient. No second app/transaction/scheduler.
3. Caddy's response wrapper checks signed expiry only, and unwrap-based hijacking
   skips the response gate. Share current verified availability and gate hijacking;
   clear protected headers/body on rejection. Preserve already committed streams.
4. Public operational status lacks committed signer/coverage, chain deadline and
   maintenance retry state. Add public diagnostics to the shared manager/worker
   and bounded local replacement outcome diagnostics; no secrets or browser claims.
5. Fresh acceptance must prove repeated real renewals with a held original
   connection, both key choices through real integrations, quiet maintenance,
   delayed response/upgrade rejection, repair and repeated recovery/concurrency.

Engineering review: keep four modules and stdlib core. Extend current binding
validation, manager status, worker and integration hooks; preserve durable formats
and deployment journal. Add focused deterministic boundary tests where possible,
plus bounded real-time worker/expiry and real-server tests. Do not prune retained
coverage, expand trust, alter browser assertions or introduce broad clock/storage
abstractions. After core commit/push, pin its implemented immutable version in
consumers; then push consumers and pin devtools. Verify remote installations,
all module races/independence/builds, real lifecycle smokes and sequential native
Chrome/Firefox evidence for both integrations. Record exact final revisions,
commands, versions and the known Chromium exp incompatibility separately.

Deliver a lifecycle contract/comparison and acceptance matrix, operator recovery
and diagnostics documentation, a new dated evidence report, reviewed commits and
a PR against main. Do not merge or release.
