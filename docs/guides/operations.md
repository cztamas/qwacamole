# Binding discovery and sustained operation

Use these procedures to monitor binding freshness, maintain coverage for held
connections, and recover without bypassing the activation gate. Start with the
[Caddy](caddy-quickstart.md) or [Certbot/Nginx](nginx-quickstart.md) quickstart.

For custom Caddy configurations upgraded from 2.10, review HTTPS upstream Host
handling: Caddy 2.11 sets that header to the upstream address automatically.
Configure an explicit `header_up Host` policy if your application requires the
original host, while retaining ordinary upstream TLS verification. See the
[upstream release notes](https://github.com/caddyserver/caddy/releases/tag/v2.11.1).
The standard quickstarts use HTTP applications.

## Discovery, caching and migration

Application responses preserve their own cache policy and Link fields and add
`Link: </bindings/current.jose>; rel="tls-certificate-binding"`. The stable,
same-origin endpoint serves one committed, independently verified document with
`Content-Type: application/jose` and `Cache-Control: no-store`. GET/HEAD agree;
HEAD omits the body, other methods return 405, and binding responses never add a
recursive discovery Link. An unavailable/expired current binding fails closed.
Reserve `/bindings/*` and the binding relation for this plugin.

`current.json.url` remains the immutable SHA-256 artifact identifier. The additive
`discovery_url` field is the public URL. Recovery checks the immutable artifact's
hash/signature/profile, signed timestamps and exact retained certificate set;
it never treats the mutable endpoint as a durable artifact. Legacy manifests
without `discovery_url` recover without dropping TLS leaves. No offline manifest
rewrite is needed. Numeric JSON durations remain supported; readable strings
are supported for all duration fields. Explicit lifetimes below 3s now reject.
An omitted lifetime changes from 45s to 1h. Changing policy settings requires a
full stop/restart because overlapping configurations must share one policy.

Previously cached responses containing immutable discovery Links cannot be
retroactively repaired by changing the server's headers. Purge/invalidate them
or wait for their application cache lifetime to end before relying on stable
discovery. Increasing document grace preserves access but cannot extend a signed
document's expiry. Never redirect an immutable identifier to different content.
Ensure intermediaries honor no-store on the stable endpoint; exclude that path
from forced caching, stale serving and application rewrites. Application cache
lifetimes can exceed binding lifetimes once their cached Link is stable.

The binding must cover the certificate actually presented to the browser.
Putting a CDN, load balancer or TLS-terminating proxy in front of Caddy changes
that certificate: a binding to Caddy's upstream certificate is insufficient.
Only use direct browser-facing Caddy TLS or a deliberately designed certificate
and binding integration at the termination point. This plugin supplies no CDN
certificate synchronization or distributed state.

## Policy and diagnostics

| Setting | Default | Constraint and effect |
|---|---|---|
| `lifetime` | `1h` | At least 3s; signed validity also capped by QWAC/latest retained TLS expiry |
| `refresh_before` | One third of lifetime | At least 1s, at most half the lifetime; reduced to one third of the actual signed lifetime when capped |
| `document_grace` | `24h` | At least 1s; both modification-time grace and signed-expiry grace must pass |
| `max_certificates` | `256` | Positive enforced ceiling for additions, never a pruning instruction |
| `max_document_bytes` | `1048576` | At least 1024; candidate bytes checked before any document/manifest write |

Zero selects a default. Legacy state above the certificate-count ceiling recovers
and may refresh its existing coverage, but cannot add leaves until capacity is
raised or coverage is retired offline. A document-size ceiling below existing
size preserves recovery/read access but blocks new publications, including
refresh; choose a limit that accommodates current state before restarting.

The owner has one expiry-driven worker across overlapping Caddy reloads. New
commits wake it. Failed refreshes retry with exponential backoff and randomized
0–25% delay, bounded below one minute. While validity remains, retries are capped
at half the remaining time when at least two seconds remain. Backoff never
extends signed validity. When TLS/QWAC expiry leaves less than three seconds of
effective validity, avoid continuously signing the same near-expiry document;
renewal/replacement wakes the worker, otherwise refresh fails at expiry.

The deployed defaults normally produce approximately 36 documents per day per
host (40-minute refresh cadence). Shorter lifetimes increase storage and signing
load; choose them only for isolated evaluation.
Collection runs at grace/4, bounded between one second and one hour. It holds
the same lock as publication and reads, protects the committed current artifact,
and only removes regular, correctly hash-named files whose signature, authority,
hostname, profile and hash-list structure authenticate plugin ownership. Verified
orphans from failed transactions can be collected after grace. Unknown files,
symlinks, corrupted documents and `.atomic-*` staging files are preserved for
operator inspection; automatic deletion must not guess ownership. Future-dated
files defer collection. Protect state from other writers; the exclusive owner
lock does not make a directory safe against external modification.

Structured activation/collection/failure logs include hostname, QWAC expiry,
binding expiry, next refresh, retained leaf count/capacity, document size, storage
file count/bytes, and last publication failure. Core consumers can call
`Manager.Status`. Watch storage growth and expiry well in advance; this is not
a global disk quota. Unknown/staging files require offline inspection. Logs
report collection errors and refresh recovery separately. No diagnostic label
establishes native browser acceptance.

## Publication, replacement and recovery

Preparation validates supplied material without publishing. Activation combines
the candidate signer with retained, stored and newly issued TLS leaves. It signs,
independently verifies, writes/fsyncs an immutable document, then atomically
commits/fsyncs the manifest. Only then do live signer, snapshot and configuration
generation change. The ACME issuer returns the new leaf only after this gate.
Older generations cannot reinstall old credentials. Readers see a complete old
or new document. No private key is persisted in the binding manifest.

On a normal failed manifest commit, restore prior manifest bytes if a rename
already occurred. If failing storage also prevents rollback, report that fact
explicitly and repair storage before restart; the prior in-memory binding remains
available until expiry. No filesystem protocol can guarantee durability against
arbitrary hardware/data corruption. Supported local filesystems must implement
atomic rename and file/directory fsync. Network/shared storage is unsupported.

A process crash before manifest commit recovers the previous commit and ignores
staged/orphan candidates. A crash after durable commit recovers the complete new
commit. A crash during rename/fsync may recover either complete commit; it must
never recover a mix. Restart verifies the selected artifact and exact TLS set
before advertisement. Restore an intact matching manifest/document backup if
recovery fails; do not bypass verification, edit hashes, or delete individual
retained leaves. Fix credentials, disk permissions/space or capacity, then retry
gated issuance/reload. Same-authority leaf/key replacement remains explicit;
authority migration needs a separate browser-trust transition.

## Offline TLS retirement

Document GC and certificate retirement are different operations. Retain expired
TLS leaves because authenticated HTTP/2/WebSocket connections can outlive leaf
NotAfter. Caddy 2.11.7 exposes `RegisterConnState`, `RegisterConnContext`, and
shutdown callbacks for `http.Server`; its HTTP/3 server is separate. A server-side
`tls.ConnectionState` does not expose the selected local certificate. These hooks
alone cannot prove zero surviving connections for each retained leaf across
protocols/reloads, including issuer-only configurations. The plugin therefore
implements no automatic TLS pruning and makes no warning-only bounded-retention
claim. Capacity refusal can eventually block renewal and cause HTTPS downtime.

Before reaching the limit:

1. Schedule maintenance and take a restricted backup of supplied credentials,
   the complete binding directory and Caddy's TLS storage/configuration.
2. Stop the entire owning Caddy process and wait for it to exit. Verify that all
   its listener/connection resources are closed. A reload, certificate expiry,
   elapsed grace or a guessed idle timeout is insufficient. If graceful shutdown
   waits on long-lived connections, follow your service manager's explicit drain/
   termination procedure and confirm process exit before continuing.
3. Move the entire binding directory to a restricted offline archive. Recreate
   the configured empty directory with the Caddy account's ownership and 0700
   permissions. Keep Caddy's normal TLS storage: startup must recover its current
   ordinary TLS leaf and publish coverage before serving protected responses.
   Do not perform this step while any process holds the state-owner lock.
4. Restart with valid supplied QWAC credentials. Confirm the structured binding
   activation, current certificate coverage, uncached stable endpoint, ordinary
   TLS verification and native qualified identity. If it fails, stop and restore
   the matching backup as a whole; investigate rather than bypassing the gate.
5. Retain the archive for your recovery policy; remove it only as a separate
   operator action. Old immutable Links may no longer resolve, so complete the
   cached-response migration above first. New stable cached Links survive the
   maintenance restart when the browser reconnects to the current TLS leaf.

Raising capacity instead preserves all old coverage but increases document size,
CPU and storage costs. It is an explicit availability/resource decision.

## Dedicated Nginx agent operations

The production `agent/` integration preserves the same shared storage/lifecycle
invariants with a separate fixed private Unix control interface. Its complete
single-host Nginx configuration and retained deployment journal are versioned.
See [serving settings and Linux installation](agent-nginx.md) and
[offline migration](agent-migration.md). The migration preserves certificate
history; it does not claim to detect old-connection drain.

Use private `qwacamole-agent status` and structured service logs to alert on:

| Condition | Evidence and response |
|---|---|
| Failed deploy hook even with Certbot exit 0 | `deployment.last_error`, `phase`, `active/candidate/previous`, `updated`; successful publication alone is insufficient. Require committed/fresh checked activation. |
| Binding refresh failure or impending expiry | `binding.last_publication_failure`, `binding.binding_expires`, `binding.next_refresh` and `binding maintenance` failure/retry events; repair signing/storage and alert before signed expiry closes availability. |
| QWAC expiry | `binding.qwac_expires`; supply a validated same-authority replacement before expiry and invoke `replace-qwac`. |
| Retained-certificate capacity pressure | `binding.retained_certificates/max_certificates`, `binding.current_document_bytes`, and the configured `max_document_bytes`; capacity refusal must preserve coverage. Increase limits only with a reviewed plan or retire offline after full shutdown. |
| Recovery needed | Journal `prepared/bound/activating/reloading/confirming/rolling_back` needs reconciliation; `needs_deploy` requires a fresh valid lineage. Never force `ready` or delete old IDs. |
| Version/migration failure | Startup/validate/migrate stderr is nonzero and actionable; `migration.json` blocks startup until retry/consistent restoration. Unknown future versions require a compatible binary. |

Status is only on the permission-protected local socket. Do not proxy it into the
public site. The daemon's structured events report maintenance failures and
recovery without logging private keys, request bodies or credentials. Monitor
Nginx process/log and disk state separately; service-manager examples remain
untested examples requiring adaptation to your service manager.
