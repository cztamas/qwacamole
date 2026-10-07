# Production agent and Certbot + Nginx integration

Objective: the user's 7 October agent objective. Base: remote main `3fee651`
(includes the hardened Caddy integration). Branch: `codex/certbot-nginx-agent`.
No merge, tags, publication, visibility or license changes are authorized.

## Design and sequence

1. Keep the root standard-library-only. Share binding delivery, expiry-driven
   refresh/retry/collection and diagnostic events there. Caddy keeps leases,
   generations and policy checks; change registry locking to short metadata
   operations plus per-owner serialization, with unchanged-generation fast paths.
2. Add one `agent/` module, executable `qwacamole-agent`. One exclusive owner,
   permission-protected Unix control socket, loopback binding delivery, strict
   configuration and supplied QWAC credentials. Commands validate, serve, deploy,
   certbot-hook, status and check. No arbitrary commands in control requests.
3. Use agent-owned immutable TLS generations and an atomic current pointer.
   Persist deployment intent before publication; publish complete binding before
   selecting staged TLS; validate Nginx, reload, confirm via fresh verified TLS.
   Journal stages distinguish bound/staged/activating/confirming/committed and
   rollback/reconciliation failure. Restart compares files and actual served
   leaf; successful reload exit alone never proves deployment. Retain old leaves.
4. Configure Nginx with HTTP webroot challenge, HTTPS reverse proxy, uncached
   binding proxy and internal auth_request availability gate. Preserve upstream
   headers/cache/body/status and add a separate Link. No auth_request cache or
   fail-open route. Reserve control paths; private socket is never proxied.
5. Isolated devtools toolchain: real Certbot certonly/webroot + Pebble validation,
   actual Nginx process, accelerated explicitly configured lifetimes, owned
   process cleanup. Test complete lifecycle, faults/retry/reconciliation,
   held versus fresh connections, cache, collection and capacity refusal.
6. Workspace and independently pinned consumer races/boundaries/CI; real
   immutable remote Caddy and agent installations without workspace replacements.
   Run fresh Caddy Chrome then Firefox, then Nginx native acceptance sequentially,
   maintaining ordinary TLS verification and separate authorities.
7. Document operation, permissions, hooks, backup/recovery, service setup and
   Nginx+agent full-stop retirement; dated evidence separates historical from
   newly executed proof. Audit every objective requirement before completion.

## Completion ledger

Every row below is backed by newly executed evidence in
[the dated acceptance report](evidence-agent-nginx-20261007.md). Historical browser
reports and planned tests were not substituted for current acceptance.

| Requirement | Evidence needed | Status |
|---|---|---|
| Production boundaries | Four modules; workspace/independent races and dependency checks pass; clean remote agent metadata is core + stdlib only | Verified |
| Shared lifecycle and Caddy locking | Shared worker/delivery; blocked-signing cross-host and cleanup/generation races; fresh actual/remote Caddy runtime and Chrome/Firefox pass established criteria | Verified |
| Agent operational/control interface | Fixed-config CLI and separate production daemon; private socket/exclusive owners, bodyless controls, no private file/control exposure on public listener | Verified |
| Certbot deployment | Real HTTP-01 initial and normal automatic renewal, durable binding before staging, fresh TLS confirmation; hook errors independently recorded even with Certbot exit 0 | Verified |
| Journal/reconciliation/failures | Five actual child-process crash boundaries, distinct leaves, two restarts; real signing/storage/config/reload/nonactivation/retry; expiry recovery without pruning | Verified |
| Nginx behavior/gate | Real application/cache/headers/status/streaming, binding semantics, agent-down and explicit expired-state gates; reproduced/fixed delayed response commit across expiry | Verified |
| Real full lifecycle | Final remote-installed 24-check real run and hosted CI pass, including original held stream, refresh, auto-renew, expired cache, replacement, collection, faults/retry/restart and capacity refusal | Verified |
| Both native integrations | Fresh Caddy Chrome/Firefox; final remote-installed Nginx Chrome/Firefox initial/renewed/cache/replacement with unchanged trust. Chrome's exact ignored-exp failure remains nonzero | Verified |
| Independent CI/install | Workspace/real pinned independent races, clean remote Caddy+agent runtime and hosted run 37684895890 on complete harness 2a4405f pass; uploaded 24 Nginx/22 Caddy results inspected | Verified |
| Usable documentation | Exact remote install, ordered manual setup/native checks, config/service/permissions/hooks/failure/monitoring/backup/reconciliation/Nginx-worker retirement and dated evidence; final requirement audit complete | Verified |
