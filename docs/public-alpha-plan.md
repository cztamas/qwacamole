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

All items start **unproven**. Replace these entries with exact current files,
commands, outcomes and dated evidence as each stage is completed; green tests
alone do not prove the full objective.

| Requirement | Required proof | Status |
|---|---|---|
| Cached discovery | Real cache outlives signed expiry; stable Link resolves current verified coverage after refresh/renewal/replacement; native Firefox acceptance | Unproven |
| Transactional activation | One candidate with absent stored leaf; boundary failures retain prior live state; crash/restart old-or-new; concurrent snapshots; issuer gate | New core/lease regressions and actual Caddy replacement/recovery passed; final native failure gate pending |
| Safe document retention | Current/staged/grace/readers protected, only owned validated files deleted, recovery and failure tests | Ownership/grace/current/orphan tests and automatic actual-Caddy collection passed |
| TLS retention/capacity | Supported lifecycle investigation, preserved expired-held coverage, enforced limits and justified drain/retirement procedure | Investigation, enforced limits, existing expired-held regression and operational procedure implemented; fresh native held check pending |
| Refresh/diagnostics | Expiry-driven validated margins, backoff/jitter, fail-closed recovery and useful expiry/error/count/size observations | Policy/duration/backoff tests, automatic refresh/cache regression and structured diagnostics implemented; native refresh/failure proof pending |
| Caddy behavior | Multi-site/unprotected, conditional/cache/redirect/error, streaming/WebSocket, concurrent lifecycle/disk/held connections | 21 actual-Caddy smoke checks and concurrent lifecycle race regression passed; fresh native held proof pending |
| Compatibility and CI | JSON/state migrations, all-module races/boundaries, portable new regressions | Local races/boundaries/migrations passed; hosted hardening-head CI pending |
| Exact-ref installation | Real resolved core/plugin versions, fresh external build with no replacements/devtools, installed runtime | Unproven |
| Both native browsers | All named Chrome and Firefox requirements, unchanged trust/profile, exact expected suite outcomes | Unproven |
| Unpublished alpha | Updated guides/comments, release/contribution/security artifacts and checklist with owner decisions; no unauthorized publication | Unproven |

Historical evidence remains unchanged. New results belong in a separate dated
hardening report. Completion requires auditing the original objective against
each item above and its authoritative current evidence.

New executed commands and artifact paths: [7 October hardening evidence](evidence-hardening-20261007.md).
