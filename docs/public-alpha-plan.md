# Sustained-use hardening and unpublished public alpha

Starting point: `8dd1dd7ee2e3cb96fa005938e98db6355a675c00` on
`codex/installable-caddy-plugin`. Working branch: `codex/public-alpha-hardening`.
Objective source: the user's 7 October 2026 pasted objective. No merge, tags,
release publication, privacy/license choice or Caddy registration is authorized.

## Engineering decisions and sequence

1. Keep `current.json` referencing an immutable verified document. Advertise a
   separate stable same-origin `/bindings/current.jose`, always uncached and
   served from one committed snapshot. Preserve application cache/Link headers;
   never add a discovery Link to binding responses. Test a real caching proxy
   and native Firefox across expiry, renewal and credential replacement. Old
   cached immutable Links require cache invalidation or sufficient migration grace.
2. Combine candidate credentials and every required retained/stored TLS leaf in
   one manager transaction. Prepare/sign/independently verify/write document/
   commit manifest before switching live material or configuration generation.
   Exercise failures with a stored TLS leaf absent from recovered coverage,
   concurrent readers, durable commit crash/restart old-or-new recovery, and
   the existing ACME pre-return gate.
3. Collect only validated plugin-owned immutable artifacts beyond a configurable
   fetch grace. Protect current/staged documents and readers under serialized
   state ownership. Keep TLS coverage independent of document collection.
   Investigate supported Caddy connection lifecycle hooks before retirement;
   otherwise retain coverage, enforce explicit capacity limits and document a
   full drain/offline retirement procedure. Never infer retirement from NotAfter.
4. Replace fixed polling/margins with expiry-driven refresh, validated deployment
   settings, bounded retry backoff and jitter. Initial deployment choices to
   validate: one-hour lifetime, proportional advance margin, 24-hour immutable
   grace, explicit retained-certificate/document-size capacity. Devtools select
   accelerated settings explicitly. Record expiry/refresh errors/count/storage
   diagnostics without adding a separate service or weakening failure behavior.
5. Add actual Caddy/application tests for independent protected sites and an
   unprotected site, caching/conditional responses, redirects/error routes,
   streaming/WebSockets, concurrent lifecycle/collection, disk failures/recovery
   and held connections. Preserve practical JSON/state compatibility. Extend
   portable CI and run all three modules' race tests and boundaries.
6. Push core changes, let Go resolve a real retrievable immutable core version,
   update the plugin dependency, push the implemented plugin, and verify exact-ref
   installation/runtime outside the checkout with GOWORK=off, fresh module cache,
   no replacements/devtools. Preserve the current dependency/tool pins.
7. Run fresh Chrome then Firefox sequentially. Chrome: native owner/status,
   unchanged-trust renewal, restart/failures, refresh, continuous authenticated
   HTTP/2 through leaf expiry, and only the exact known ignored-exp suite failure.
   Firefox: native owner/status, unchanged-profile/trust renewal, all eleven
   fixtures, and native cached-discovery acceptance across expiry/replacement.
   Browser unavailability blocks whole-goal completion, not independent work.
8. Update README/plugin guide/comments and prepare release notes, CONTRIBUTING,
   SECURITY and an actionable unpublished release checklist. Document browser-facing
   TLS binding and CDN termination limits, migrations, operations, remaining
   engineering limits, root/nested tags, anonymous install after publication,
   Caddy package registration and hosted-builder compatibility. Preserve fixture
   attribution and leave owner decisions explicit.

## Verification and completion ledger

All engineering requirements are complete. Exact commands, outcomes, source
versions and native evidence are in the [7 October report](evidence-hardening-20261007.md).
Publication and account actions remain explicit owner gates.

| Requirement | Current proof | Status |
|---|---|---|
| Cached discovery | Real 600s cache with unchanged Link/body at ages 71s/100s; signed expiry crossed; native Firefox accepts renewed TLS and replacement QWAC | Complete |
| Transactional activation | Absent stored-leaf and separate-module-context regressions, fault/crash old-or-new recovery, concurrent snapshots; both browsers prove pre-return failure gates/recovery | Complete |
| Safe document retention | Authenticated ownership/grace/current/staged/readers/orphan regressions and automatic actual-Caddy collection | Complete |
| TLS retention/capacity | Supported Caddy lifecycle investigation; enforced limits; Chrome continuous HTTP/2 47s beyond TLS expiry; tested full-stop/offline retirement | Complete |
| Refresh/diagnostics | Expiry-driven validated margins, backoff/jitter, fail-closed outage/recovery tests; both native browsers prove independent refresh and failure recovery | Complete |
| Caddy behavior | All 22 local and remotely installed actual-Caddy checks; multi-site/cache/conditional/redirect/error/streaming/WebSocket/held lifecycle; disk/concurrency race coverage | Complete |
| Compatibility and CI | Legacy state/numeric JSON migration; three module race suites and boundaries; hosted run 37597323566 passes all 22 runtime checks | Complete |
| Exact-ref installation | Remote production ref 234f8d7e20a5; real core d576a2929a0f; fresh external build with no replacements/devtools; installed binary passes all 22 checks | Complete (authenticated private install) |
| Both native browsers | Fresh Chrome then Firefox sequentially; exact owner/chain/native UI evidence; unchanged profile/trust; Chrome ten passes plus only documented ignored-exp failure, Firefox eleven passes | Complete with documented Chrome gap |
| Unpublished alpha | README/guide/comments, operations/migrations, draft release notes, CONTRIBUTING, SECURITY and owner release checklist; no publication actions performed | Complete |

Historical evidence remains unchanged. New results belong in a separate dated
hardening report. Completion requires auditing the original objective against
each item above and its authoritative current evidence.

New executed commands and artifact paths: [7 October hardening evidence](evidence-hardening-20261007.md).
