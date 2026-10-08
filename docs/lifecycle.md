# Lifecycle contract

Qwacamole distinguishes **obtained** TLS material, **durably published** binding
coverage, and **actually served** TLS. An ACME/Certbot command or reload exit is
not activation proof. Production uses one explicit same-authority 2-QWAC signer
and one literal hostname per binding store. Supplied credentials are validated
for key matching, profile, hostname, dates and the configured chain. Importing
credentials, selecting their filesystem paths and activating them are separate
operations; see [the credential guide](byo-credentials.md).

## Integration comparison

Both integrations share signing, independent verification, immutable documents,
atomic manifest commit, retained TLS coverage, collection and one maintenance
worker. The supported deployment paths are Caddy's sole QWAC issuer and the
agent's controlled Certbot deployment. External certificate loaders, direct
Nginx certificate edits and unmanaged reloads bypass these guarantees.

| Operation and trigger | Caddy publication and activation | Certbot/Nginx publication and activation | Failure and recovery |
|---|---|---|---|
| Initial startup, including stored TLS | Provision validates credentials and recovers state without selecting a candidate. `SetConfig` discovers the stored TLS leaf. Managed-cache startup publishes its coverage and starts maintenance without HTTP traffic. The abortable pre-handshake event requires verified coverage before ordinary TLS; TLS-ALPN ACME validation remains separate. | Open validates files/state and takes exclusive locks; readiness starts closed. Reconcile compares the journal, selected immutable generation and a fresh normally verified served certificate. First deploy follows the transaction below. | Missing/malformed/inconsistent state rejects with a diagnosis; cache-event publication failure cannot authorize a handshake. Agent remains closed until reconciliation/deployment proves a usable deployment. |
| Automatic TLS renewal | Issuer obtains the candidate, publishes and independently verifies complete retained coverage, then returns the certificate to CertMagic. `cert_obtained` is not the gate. Actual serving is checked separately by fresh TLS in acceptance. | Certbot hook validates the leaf/key/chain/name, commits coverage, stages immutable files, selects them, validates Nginx, reloads and confirms the candidate through fresh verified TLS before recording committed activation. | Signing/verification/publication refuses candidate activation. Agent restores and freshly verifies the previous deployment after selection/validation/reload/probe failure. Uncertain restoration remains closed and journaled. |
| Periodic refresh, no TLS renewal or application traffic | One owner worker spans overlapping configurations and refreshes the committed retained set. | The daemon runs the same worker independently of request traffic and Certbot. | Bounded jittered retry; previous commit is usable only while valid. Publication/replacement wakes the worker and resets retry state. Repair permits automatic recovery without restart. |
| Same-authority QWAC replacement, same or new key | Force reload re-reads stable paths; candidate preparation does not change the signer. Live cache/handshake/HTTP/issuance activation publishes retained/stored TLS coverage before choosing the generation. Older generations cannot roll back the signer. | `replace-qwac` reads startup-configured paths, validates all material, publishes the complete retained set and then switches signer; it does not reload Nginx. | Rejection or signing/verification/commit failure preserves the working signer/document. Caddy can serve the old verified binding only if all live configurations' stored TLS is covered. Failed candidate remains pending; repair and retry activation. |
| Overlapping TLS leaves and repeated renewal | Every committed old/new leaf stays covered, including the leaf on an established connection after its `NotAfter`. | Same retention; Nginx reload preserves existing workers/streams while new handshakes use the confirmed generation. | No expiry-based pruning. Capacity rejects new additions rather than dropping old coverage. |
| Unchanged configuration reload | Force reload re-reads credentials; leases share one process owner/worker and exclusive state lock. Stored leaves from live generations join the same transaction. An idle reload may remain prepared until live activation. | Agent startup settings are immutable for an online daemon; repeated deploy/reconcile of the same valid generation is safe. Serving-policy changes use the existing offline migration. | Rejected provisioning does not activate credentials. Successful reload alone does not prove signer selection or served TLS; inspect current signed identity and activation logs/probe. |
| Clean restart or interrupted operation | Recover authenticates manifest, document hash/signature, signed times and retained IDs before cache/startup activation. Process lock is released only after owner operations and maintenance stop. | Journal intent is reconciled against immutable files and actual fresh TLS. Already served bound candidate can be committed; otherwise restore the verified previous generation. Repeated recovery is safe. | Staged/orphan documents are never advertised as current. Invalid state fails closed; repair/restore consistent state. A journal's intended phase is not evidence of activation. |
| Expiry, unavailable signer or publication failure | Verified availability is checked before protected response commitment and protocol hijacking. | Gate/auth_request and application proxy check availability, including delayed response commitment and WebSocket upgrade. | New protected responses/upgrades close when binding/chain validity ends; maintenance retries without extending validity. Repair recovers. If all retained TLS leaves expire, deploy fresh TLS; retain old coverage. |
| Certificate/document capacity and collection | Shared policy refuses oversized documents or added leaves over capacity before changing current state. | Same policy, before any candidate staging or activation. | Collection authenticates historical hash-named regular files and preserves current, unknown/staged artifacts and both expiry/mtime grace. It never removes TLS coverage. Drain connections, stop the owner, archive state and rebuild through the documented offline retirement procedure. |

## Validity, scheduling and stream limits

Effective signed expiry is the earliest of configured lifetime, **every supplied
QWAC chain certificate's** expiry, and the **latest** retained TLS leaf expiry.
The earliest TLS leaf expiry does not discard coverage needed by a connection
that already authenticated it. Signing and independent verification enforce the
same QWAC cap. Future/not-yet-valid or expired chain material cannot authorize
publication or protected delivery.

Refresh uses the committed signed lifetime, reducing the configured advance
margin proportionally. Under three seconds of remaining signed lifetime it waits
for expiry rather than repeatedly re-signing a document that cannot gain useful
validity. Failures retry exponentially with jitter, capped below one minute and
shortened while a usable commit has time remaining. New publication or signer
activation wakes this same worker; no watcher or second scheduler is added.

A response committed while valid may keep streaming after expiry. A previously
upgraded WebSocket remains established. Bytes already sent cannot be retracted.
TLS certificate expiry does not automatically close a TLS connection. Qwacamole
rejects **new protected commitments and upgrades**, and retains the certificate
coverage needed for existing connections; it does not promise to terminate all
in-flight traffic at a deadline. Normal verification still applies to new TLS
handshakes. Caddy's pre-handshake binding gate additionally prevents uncovered
stored TLS from entering the supported serving path.

Operator-owned QWAC files are not automatically rolled back. Following a rejected
replacement, restore the previous selected inputs **before restarting**, or a
restart may fail validation or intentionally reconcile the newly selected valid
credential. Trust stays outside the credential pointer and is never expanded by
import/replacement. Automatic QWAC renewal, authority migration and automatic
online TLS retirement remain outside scope. Follow [Caddy retirement](caddy-plugin.md)
and [agent offline migration/retirement](agent-nginx.md).

## Operational evidence

Use the authenticated local agent `status`, `check`, `reconcile` and `replace-qwac`
controls. Caddy emits activation, pending-replacement and shared maintenance
status logs. Shared public fields include:

- `active_qwac_sha256`: signer of the committed document; configured inputs alone
  are not evidence of activation. `configured_qwac_sha256` and
  `replacement_pending` distinguish a prepared candidate.
- `covered_tls_sha256`, binding/QWAC leaf/full-chain expiry, retained count,
  current document bytes and configured certificate/document capacities.
- `next_refresh`, `refresh_failures`, `refresh_retry_at`, last refresh/publication
  failure and storage counts. Retry state is process-local; signed state is durable.

Agent status separately reports journal phase, previous/candidate/active TLS,
activation error, readiness and the last replacement attempt's pending/error
state. A `bound`/`reloading`/`confirming` candidate is not confirmed active merely
because publication succeeded. Caddy pending logs identify the candidate signer;
its committed signer remains authoritative. For a quiet reload, trigger a fresh
verified protected connection and compare the current binding signer with the
inspected candidate. No local status claims browser acceptance, revocation
clearance or legal qualification. Keep private keys, profiles and raw sensitive
logs out of reports.

## Acceptance matrix

The dated lifecycle report records **fresh** commands/revisions/results. Prior
credential/agent reports remain inherited evidence, not substitutes for reruns.

| Guarantee | Focused synthetic evidence | Real integration/native evidence |
|---|---|---|
| Stored TLS publication and quiet startup maintenance | `TestStoredCaddyTLSGatedBeforeHandshakeAndMaintainedWithoutTraffic`, `TestHandlerActivationIncludesIssuerStoredTLSAcrossModuleContexts` | Caddy restart and fresh verified handshakes; independent refresh |
| Publication before activation; failure preserves old/new complete state | `TestActivationFailureIncludingUnretainedStoredTLSIsAtomic`, `TestAgentPublicationFailureNeverStagesOrReloads`, `TestPublicationFailuresPreserveLiveAndDurableCommit` | Caddy issuer failure recovery; Nginx sign/document/manifest, validation, reload and zero-exit nonactivation checks |
| Full-chain expiry cap, no near-expiry spin, bounded retries/recovery | `TestBindingValidityIncludesEntireQWACChain`, `TestOperationalPolicyValidationAndDefaults`, `TestRefreshWorkerFailsClosedAndRecoversWithoutReload`, `TestMaintenanceRetryDiagnosticsAndPublicationWake`, `TestRetryBoundsAndJitter` | Both independent refresh checks; actual signed-expiry rejection and repaired-signer recovery |
| Response and upgrade commitment; existing stream preservation | `TestCaddyDelayedProtocolUpgradeRejectsAtCommit`, agent response-gate tests | Real WebSocket and slow-stream checks; delayed expiry/gate recovery; native held HTTP/2 connection |
| Same/new-key replacement, stable discovery and restart | `TestSuppliedCredentialReplacementAndRestart`, `TestRecoveredStatusDistinguishesConfiguredAndCommittedSigner` | Both real harnesses exercise both key choices; native cached discovery and changed identities with unchanged trust |
| Repeated renewal/overlap and bounded storage | `TestExpiredTLSConnectionStaysBoundAfterRefreshAndRestart`, `TestCapacitiesRefuseAdditionsWithoutPruningAndMigrateLegacyState`, `TestCollectionAuthenticatesOwnershipProtectsGraceAndCurrent` | Two actual renewals, fresh served-leaf confirmation and original held connection/stream; collection and over-capacity rejection |
| Interruption and repeated recovery | `TestPublicationCrashBoundary`, `TestAgentProcessCrashReconciliation`, `TestAgentInterruptedJournalReconcilesServedState`, `TestLifecycleRecoveryMissingMalformedAndInconsistentState`, `TestAgentJournalDamageRemainsClosedAndRepairable` | Nginx offline migration/interruption/repetition; clean server/daemon restart |
| Concurrent renewal/refresh/replacement/reload/collection and ownership | `TestConcurrentReloadPublicationRefreshCollectionAndReaders`, `TestAgentConcurrentLifecycleAndReplacementDiagnostics`, `TestInFlightOperationRetainsOwnershipPastModuleCleanup`, conflicting-owner tests | Repeated Caddy reloads, multiple sites, rejected second owner and separate production binaries |
| Browser trust/identity rather than self-verifier only | Native fixture suites retain trust separation, signature/profile/name/date/critical-header negatives | Fresh Chromium and Firefox for both integrations; unchanged profiles/trust/cached discovery. Chromium's expired-`exp` negative remains a failing known incompatibility if reproduced; Firefox requires explicit documented compatibility mode. |
