# Documentation website evidence — 9 October 2026

## Source and scope

Release-preparation PR [#7](https://github.com/cztamas/qwacamole/pull/7) merged
as `e24ed287023f0ec7800c93858ced76a92009f681`. This website branch starts there.
The repository remains private, original code is MIT licensed and no alpha
release/tag/assets have been published. No visibility, release, Cloudflare
account or domain settings were changed.

The final production Go sources, four module manifests/checksums, runtime
configuration examples, Makefile and production verification scripts are
identical to that baseline. All fresh native runs below used binaries rebuilt
from those sources, with only documentation/website changes in the checkout.
Their build metadata can therefore name the baseline and a dirty checkout;
these results are not a claim about a newer production binary revision.

The earlier release, lifecycle, stream-rejection, Firefox and Nginx evidence
reports remain intact. No earlier native result is substituted for the fresh
runs recorded here. Earlier Linux serving/migration and package target evidence
remains dated in the release-preparation report; macOS arm64 is this local
native-test environment.

## Website implementation and fresh local checks

The isolated `website/` package uses Node **24.21.0** LTS (official arm64
distribution checksum verified), bundled npm **11.19.0**, Astro **7.3.8**,
Starlight **0.42.6**, Pagefind **1.5.2**, Wrangler **4.149.0** and Playwright **1.64.0**. Dependencies
are locked. Only pinned esbuild/workerd binary setup scripts are approved by
npm's installation policy; the optional fsevents hook is denied.

Canonical repository Markdown supplies **17** rendered documents; the explicit
allowlist copies **10** public assets. Historical reports stay GitHub links.
No state, keys, profiles, raw logs or private account configuration is published.
The custom landing, two complete quickstarts, static Pagefind search and useful
404 build without Go, browsers, a test CA or Cloudflare credentials.

| Fresh command/check | Result |
| --- | --- |
| `cd website && npm ci` | Clean reproducible install under the pinned Node/npm |
| `npm run check` | Astro: 0 errors/warnings/hints; **8** script tests pass |
| `npm run build` | **19** HTML pages; **920** internal references; **17** search pages and **33** complete search assets pass |
| `CI=true npm run test:browser` | **11** checks pass, zero retries; its own production preview server |
| Shared-source dev watch | A temporary edit appeared; invalid-then-valid Markdown recovered without restarting; source restored |
| `git diff --check` | Pass |

The script tests exercise a full mocked successful deployment, artifact mismatch,
production metadata, post-upload identity failure, incorrect Pages production
branch, duplicate Git builds, API denial, upload failure, disabled/missing setup,
main-only events, both workflow path filters and shared-link rewriting.
They perform no real Cloudflare upload.

Final local revalidation exposed incomplete native Pagefind output (an empty
entry JSON, then an empty worker). The build now finalizes the same built-in
Pagefind index through its supported in-memory file API with awaited Node
writes, nonempty/size checks and parsed index metadata. The existing real search
assertion remains unchanged; no retry or timeout increase was added.
After the fix, all eleven browser checks passed, followed by the same real search
check in three fresh browser contexts, all passing.

Real Playwright Chromium **156.0.8078.4** rendered all three main routes at
**1440 × 1000** and **390 × 844**. Automated checks cover navigation, static
search/result navigation, keyboard skip link and search focus return, mobile
menu, keyboard scrolling/focus for code blocks, document overflow and 404 status.
Axe WCAG 2 A/AA and 2.1 AA checks pass on all six layouts and both quickstarts
in dark theme. Screenshots were visually inspected for typography, spacing,
readable code and responsive layout. This is website proof, separate from native
2-QWAC verifier acceptance.

| Page | Desktop screenshot | Mobile screenshot |
| --- | --- | --- |
| Landing | [1440 px](evidence/website-20261009/landing-1440.png) | [390 px](evidence/website-20261009/landing-390.png) |
| Caddy quickstart | [1440 px](evidence/website-20261009/caddy-quickstart-1440.png) | [390 px](evidence/website-20261009/caddy-quickstart-390.png) |
| Certbot/Nginx quickstart | [1440 px](evidence/website-20261009/nginx-quickstart-1440.png) | [390 px](evidence/website-20261009/nginx-quickstart-390.png) |

The checked static output has **79 files**, approximately **2.5 MiB** total;
its largest individual asset is **235,794 bytes**. It contains no Functions,
worker, server adapter, private PEM/key or raw log. Generated output/caches,
dependencies and browser results are ignored; only the six deliberate screenshots
and sanitized scalar native summary are retained here.

## Fresh repository, installation and native checks

| Command/check | Result |
| --- | --- |
| `make test` | All four module race suites and dependency boundaries pass; unchanged packages use Go's valid cached results |
| `scripts/check-independent-modules.sh` | Independently pinned module races pass without workspace/replacements |
| `make build agent-build plugin-build` | Harness and unchanged production components rebuilt successfully |
| `scripts/check-remote-agent.sh bb911a8f69a8743faab70967232ff166c828c4b7` | Fresh-cache authenticated immutable agent install passes |
| `scripts/check-remote-install.sh 9322bd566e825a2c2c645e464a630e09d7aa68d6` | Fresh-cache authenticated xcaddy plugin build/module inventory passes |
| `qwacamole-agent render-nginx -config config/agent.json.example` | Current CLI renders the documented complete configuration |
| `python3 -m unittest discover -s scripts -p 'test_release*.py' -v` | **25** existing release tests pass |

Installation examples use current real immutable sources, not proposed alpha
tags/assets or assumed anonymous access. CLI/configuration spelling was checked
against the implementation and existing verification scripts. A real production
Linux hostname, issuer credential and public ACME deployment were not supplied;
the quickstarts were not deployed to a production host. Systemd units retain
their existing untested-example status.

The four existing native demonstrations ran sequentially in separate fresh
state on macOS arm64, using stock Chrome **154.0.8037.99**, installed PKI Metadata
**1798**, Firefox **157.0.1**, geckodriver **0.37.1**, and explicit isolated test
trust. No normal browser profile or system trust was changed.

| Native demonstration | UTC interval | Exit and result |
| --- | --- | --- |
| Caddy Chromium `integration-test` | 18:04:30–18:17:00 | **1**, intentionally preserved expired-binding failure; **10/11** fixtures pass |
| Caddy Firefox `firefox-integration` | 18:17:00–18:22:57 | **0**; **11/11** fixtures and lifecycle checks pass |
| Nginx Chromium `nginx-browser-integration -native-browser chrome` | 18:22:57–18:24:42 | **0**; all **24** lifecycle/serving checks pass |
| Nginx Firefox `nginx-browser-integration -native-browser firefox` | 18:24:42–18:25:48 | **0**; all **24** lifecycle/serving checks pass |

Caddy Chromium passes initial native identity, independent refresh, renewal,
replacement, restart, retained cached discovery through expiry and continuous
HTTP/2 through TLS expiry. Its unchanged negative suite reports exactly
`BROWSER INCOMPATIBILITY: native verifier accepted expired exp header`.
This remains a browser limitation, not a green complete Chromium suite.
Firefox retains explicit compatibility mode and rejects expired bindings.
Nginx checks include real Certbot issuance/repeated renewal, verified activation,
WebSocket traffic, slow uploads, cached discovery, credential replacement,
collection, fail-closed unavailability and restart recovery.

[The sanitized summary](evidence/website-20261009/native-summary.json) records
actual commands with local browser/account paths replaced by placeholders,
timestamps, exit codes, fixture booleans and Nginx check names. Raw profiles,
keys and logs remain ignored. Website Chromium and stock native Chrome are
different browser versions serving different verification purposes.

## Deployment status and remaining setup

No Cloudflare credentials or Pages project configuration are available to this
task; GitHub deployment environments/settings are unset and domain binding has
not been verified. **No live upload, Pages URL or qwacamole.com
HTTPS acceptance is claimed.** Deployment defaults disabled with an explicit
workflow summary. Enabled setup/API/upload/identity failures fail the job;
uncertain writes are not retried automatically.

The dedicated workflow checks relevant PR/main changes, consumes the checked
artifact on main, serializes production uploads and supports manual dispatch.
PRs receive no Cloudflare secrets. A mocked full deployment verifies branch,
project and artifact/deployed commit identity; live account verification remains
unavailable. Hosted CI results for this PR will be recorded in its description
with exact tested SHA/run URLs, separately from these local checks.

Before enabling, create the **Direct Upload** Pages project with production
branch **main** and no automatic Git production/preview builds; create the
`website-production` environment restricted to main; set
`CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_PAGES_PROJECT`, account-scoped Pages Edit
`CLOUDFLARE_API_TOKEN`, then `WEBSITE_DEPLOY_ENABLED=true`. Dispatch from main
and verify the immutable URL and full commit in `/build-info.json`.

Connecting the apex requires separately approved Cloudflare-zone/nameserver
setup, preservation of existing DNS services, Pages custom-domain activation,
normal HTTPS and commit verification. Configure optional www redirect only
after its DNS/HTTPS is ready. Follow [the exact setup/rollback guide](website-deployment.md),
including rollback to a previous successful production deployment and free-plan
limits. This PR does not merge or perform any of those external writes.
