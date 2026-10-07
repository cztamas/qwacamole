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

All requirements are initially unproven. No planned test or historical browser
report counts as newly executed acceptance.

| Requirement | Evidence needed | Status |
|---|---|---|
| Production boundaries | Four modules and dependency checks pass; production agent builds; real remote agent install pending | Partial |
| Shared lifecycle and Caddy locking | Shared core worker/delivery; cross-host stalled-signing and existing cleanup/generation race regressions pass; actual Caddy and native reruns pending | Partial |
| Agent operational/control interface | CLI implemented, production daemon used by real smoke; private socket/exclusive owner and HTTP delivery checked; security/failure audit remains | Partial |
| Certbot deployment | Real initial HTTP-01, normal automatic renew (no force), controlled immutable paths and fresh verified confirmation pass | Partial; final acceptance pending |
| Journal/reconciliation/failures | Journal, distinct-leaf rollback/retry, publication failures, ownership and restart tests pass; stronger crash/interruption and actual failure coverage pending | Partial |
| Nginx behavior/gate | Real headers/cache/conditional/redirect/error/binding semantics and agent-down gate pass; explicit expired-state and further fault checks pending | Partial |
| Real full lifecycle | Thirteen real checks pass: issuance, independent refresh, automatic renewal, original held stream, expired cached discovery, same-authority replacement, GC and restart; faults/capacity remain | Partial |
| Both native integrations | Fresh Caddy Chrome/Firefox and new Nginx Chrome/Firefox, initial/renewed/cache, unchanged trust, exact Chrome gap | Pending |
| Independent CI/install | All-module races, workspace and real pinned independent consumers, clean remote Caddy and agent runtime | Pending |
| Usable documentation | Installation/service/security/monitoring/backup/retirement and new dated report | Pending |
