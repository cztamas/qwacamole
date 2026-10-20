# Website content review — 20 October 2026

Reviewed all 14 published documents in `website/docs-map.mjs`, the homepage,
404 page, documentation navigation/accessibility labels and shared example
configurations. The starting main commit was
`50a370c93257517108faaf9200bf9369d9af86c7`.

Compared release and platform claims with the public `v0.1.0-alpha.1` release
(published 20 October 2026, prerelease, six assets) and the final verification
in [the release report](evidence-alpha-release-20261020.md). Checked command names,
setup paths, serving defaults, migration formats and response gates against the
current agent/Caddy/core source. Native integration evidence is reused from the
release verification; this documentation change does not rerun those suites.

## Corrections

- State that the experimental release and its packages/module tags are published,
  removing publication-waiting language across installation, overview, support,
  integration and licensing pages.
- Remove the homepage's unpublished label and documentation header's private
  repository label. Present packaged Caddy as the normal starting path.
- Remove authenticated-clone prerequisites from the public demo, and distinguish
  its development tools from the production packages.
- Align Chrome PKIMetadata with final verification (1810) and date the tested
  configuration table. Retain Chromium's failed expired-binding assertion,
  Firefox compatibility requirements and cross-compilation/platform limits.
- Clarify the agent CLI prerequisite for Caddy credential preparation, and the
  published Caddy behavior for rejected delayed streaming responses.
- State the published experimental version in the security policy, retaining the
  primary email, private reporting route and best-effort support terms.

## Validation

Using Node 24.21.0 from `website/`:

- `npm run check`: no Astro diagnostics; all 19 script tests pass.
- `npm run build`: 16 HTML pages, 702 internal links/anchors/assets, 14 indexed
  product documents; maintainer material remains outside product routes/search.
- `npm run test:browser`: all 16 tests pass, including desktop/mobile layout,
  accessibility, navigation and search. Inspected the mobile homepage screenshot.
- A one-off Python HTMLParser audit checks all 16 built pages' text and accessible
  labels for obsolete release/access wording, plus current version, support,
  verification and compatibility statements. All checks pass.
- `git diff --check`: passes.

Raw local logs and the rendered-content assertion results are retained under
ignored `.local/website-content-20261020/`; browser screenshots are under
`website/test-results/`. The first sandboxed browser attempt could not bind the
loopback preview port; the permitted rerun passed. No runtime code, published
release assets or immutable tags change.
