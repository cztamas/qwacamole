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
| Production boundaries | Four modules, standard-library core, Caddy excludes agent/browser, production install | Pending |
| Shared lifecycle and Caddy locking | Shared worker/delivery, cross-host stalled-signing race, generation/cleanup safety | Pending |
| Agent operational/control interface | Validation/service/deploy/status/check, exclusive owner, protected socket, HTTP semantics | Pending |
| Certbot deployment | Documented hook env, real initial/automatic renewal, controlled paths, durable pre-activation binding and verified confirmation | Pending |
| Journal/reconciliation/failures | Every interruption boundary, invalid material/config, storage/sign/reload failure, rollback/retry and restart | Pending |
| Nginx behavior/gate | Complete validated config, prerequisite detection, real unavailable/expired/agent-down rejection and app behavior | Pending |
| Real full lifecycle | Challenge validation, refresh/replacement/cache/held connections/faults/restart/GC/capacity | Pending |
| Both native integrations | Fresh Caddy Chrome/Firefox and new Nginx Chrome/Firefox, initial/renewed/cache, unchanged trust, exact Chrome gap | Pending |
| Independent CI/install | All-module races, workspace and real pinned independent consumers, clean remote Caddy and agent runtime | Pending |
| Usable documentation | Installation/service/security/monitoring/backup/retirement and new dated report | Pending |
