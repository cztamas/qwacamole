# Website audience and deployment ordering — 10 October 2026

Review baseline: PR #8 head `65256556aac234c38f638727c3b988d4eedcfe8e`.
Implementation reviewed at `3471a7c13f173c594361019b8d060fbece02f34d`;
subsequent documentation-only polish removes legacy merge wording from the
migration table. This is new website validation, not a Go runtime acceptance run.
PR remains https://github.com/cztamas/qwacamole/pull/8. No production deployment,
merge, DNS/account change, release or repository visibility change is authorized.

## Ordering decision and new regression evidence

The serialized deploy job runs the freshness guard before Wrangler. It reads
Pages canonical production identity and every retained production-history page,
then compares full Git ancestry using the repository's contents-read token.
Successful production history remains the high-water mark across rollback. It
requires the checked source to be an ancestor of current main, allowing later
Go-only commits. It never orders SHAs or build completion times and never cancels
an active upload to establish order. Unknown identities, ancestry or unfinished
writes fail before upload. Superseded/already-deployed sources return explicit
skips and a workflow summary reason. Checked artifact/immutable URL verification,
setup rejection and refusal to retry an uncertain upload remain intact.

The **14 script tests pass**, including eight deployment tests with subcases:
B deploys while older A checks, then delayed A/rerun/manual dispatch make **zero
additional Wrangler calls**. Hash lexical order intentionally opposes ancestry.
A later Go-only C does not suppress B. Already-deployed reruns skip; rollback
history and later API pages prevent regression. First deployment, unknown source,
diverged ancestry, missing canonical metadata, failed main/history/compare reads,
incomplete pagination and unfinished write cases are covered. Existing setup,
artifact/project/production identity, post-upload failure and no-retry tests pass.
Only external GitHub/Pages/CLI boundaries are mocked; real deploy() executes.
Raw log: `.local/website-fix-check.log`.

GitHub's live read-only compare API was also checked for baseline→PR head and
reverse: ahead/behind and base/merge-base SHAs match guard assumptions. No live
Cloudflare token/project/upload verification was run; account setup remains a
maintainer action. API fields were checked against primary
[Pages history documentation](https://developers.cloudflare.com/api/resources/pages/subresources/projects/subresources/deployments/methods/list/)
and [GitHub comparison documentation](https://docs.github.com/en/rest/commits/commits#compare-two-commits).

## Every public page audited

Canonical user manuals moved to `docs/guides/`; root docs remain engineering and
maintainer material. Root SECURITY.md remains GitHub's real security policy.
`website/docs-map.mjs` explicitly publishes 12 pages plus allowlisted assets.
Detailed browser research/acceptance procedures, old reports and their results
remain in the repository; only links to moved manuals were corrected.

| Public page | Reader question and final content |
| --- | --- |
| Home / overview | What is bound to what, and which integration should I choose? Ordinary TLS vs supplied credentials, explicit trust, private/unpublished availability and links into both quickstarts. |
| Caddy quickstart | How do I reach a working Caddy deployment? Prerequisites, exact immutable build, owned paths, full configuration, foreground start, normal TLS/discovery verification, maintenance. |
| Nginx quickstart | How do I deploy with Certbot? Dedicated master, exact source install, complete shared config, agent/Nginx startup, hook/issuance, status/check, renewal and replacement. |
| Caddy reference | How do I configure and diagnose the plugin? Settings, sole issuer/middleware ordering, credentials, reload/recovery and HTTP behavior; no release chronology or acceptance-run instructions. |
| Agent reference | Which commands/settings and permissions do I need? Current install, version identity, serving policy, service examples, hook, diagnostics/recovery/rollback. Dead proposed-package download commands removed. |
| Credentials | How do I prepare, inspect/import and replace supplied material? Local key/CSR, exact supported profile/trust, protected import, atomic pointer, activation/recovery. |
| Operations | How do I renew, refresh, monitor, recover and retire? Defaults/capacity/expiry, recovery and complete shutdown rules; engineering counts/history removed. |
| Migration | How do I upgrade or roll back compatible agent state? Supported legacy formats, full stop, backups, migration and consistent rollback; release-development history removed. |
| Support | Which platforms/browsers work, and what fails? Tested versions, runtime vs cross-compilation, unsupported features, strict defaults and explicit Firefox profile. Known Chromium ignored binding-exp limitation preserved without internal case counts. |
| Demonstration | How do I try and interpret isolated native identity? Prerequisites, Chrome/Firefox commands, synthetic trust boundaries, curl/renewal/refresh, interpretation and cleanup; detailed negative matrices remain repository-only. |
| Licence | What terms and notices apply? Actual MIT LICENSE/NOTICE, third-party attribution and fixture MPL; no owner-decision or publication workflow. |
| Security | How can I report safely now? Existing private channel for authorized collaborators, unverified GitHub reporting availability, no invented contact/SLA, concrete diagnostic details. |

README, CONTRIBUTING, deployment guide, alpha checklist and Firefox investigation
produce **no public documentation routes**. Footer contribution link goes to the
repository. No agent completion claims, owner approval prompts or account setup
remain in published manuals. Installation stays honestly private and unpublished.

## Newly executed local checks

Pinned Node 24.21.0 / npm 11.19.0, Astro 7.3.8 / Starlight 0.42.6; macOS arm64.
Locked `npm --prefix website ci` freshly installs 402 packages. Root start script
runs on a Node-only PATH without Go/Cloudflare configuration. Exact commands:

```sh
npm --prefix website ci
npm --prefix website run check
npm --prefix website run build
./scripts/start-website.sh
npm --prefix website run preview -- --ignore-lock
npm --prefix website run test:browser
```

- Astro: zero errors/warnings/hints; all 14 script tests pass.
- Static build: **14 HTML pages, 601 internal links/anchors/assets**; exact
  allowlisted route set, no private/executable output; **12 search pages, 27
  complete nonempty Pagefind assets**. `.local/website-fix-build.log`.
- Production preview: all **12 Playwright checks pass**, no retries. Desktop/mobile
  quickstarts, native keyboard skip/navigation/search focus, code scrolling,
  dark/light axe, 404 and CSR static search. Boundary test verifies six excluded
  routes return 404 and enumerates **every real Pagefind result**, requiring exactly
  12 public routes and no maintainer setup/approval/completion strings.
  `.local/website-fix-browser.log`. A fuzzy search initially matched legitimate
  product words; full-index inspection replaces that indirect assertion.
- Root `./scripts/start-website.sh` prints/serves **127.0.0.1:4321** in foreground.
  Real Chromium 156.0.8078.4 review uses that server for six layouts (1440×1000/390×844),
  zero overflow/focused axe violations, navigation and keyboard/code focus.
  A canonical overview edit automatically appears and disappears in the open
  browser after restoration, without manual reload. Source remains unchanged.
  [Sanitized review metadata](evidence/website-audience-20261010/review.json).
  All [six screenshots](evidence/website-audience-20261010/landing-1440.png) were
  visually inspected; they deliberately include Astro's development toolbar.
- Absolute script invocation from `/private/tmp` with `PORT=4406 --port 4405`
  serves only loopback 4405; PORT=4406 alone serves loopback 4406. HTTP 200 verified.
  Ctrl+C releases every owned server port. Missing Node/dependencies and invalid
  arguments continue to pass the existing script tests.
- All **25 unchanged release-tool tests pass**; `git diff --check` passes.
  `.local/website-fix-release-tests.log`. No Go/module/runtime/Makefile/start-script
  changes relative to baseline. Production Go and native verifier assertions are
  unchanged, so new native runtime suites were not run for this website-only fix.

The earlier four-module/native results in the 9 October report are **inherited**,
including Chromium's expired-binding failure. They are not rerun claims here.
Final-source hosted workflow results are linked separately in the PR description.

## Remaining maintainer actions

Review/merge PR separately. Configure and verify the Direct Upload project,
environment, credentials and domains using `website-deployment.md` before enabling
any upload. First/live production and custom-domain verification remain unexecuted.
Keep production history for freshness, serialize any manual writes, and use a new
main revert commit after rollback rather than an old workflow. Unknown source or
unfinished write requires investigation; no automatic overwrite/retry is offered.
Public visibility, package publication and security-reporting setup are separate
release decisions. This change performs none of them.
