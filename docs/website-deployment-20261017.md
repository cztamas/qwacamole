# Website deployment setup — 17 October 2026

The owner authorized setting up website deployment after creating the Cloudflare
account and granting Wrangler access. No native package/tag release is included.

## Configured infrastructure

- Cloudflare account: `qwacamole` (`943250b3e756e223d6f58d90147ead13`).
- Pages Direct Upload project: `qwacamole`, production branch `main`, no Git
  integration. Assigned hostname: `qwacamole.pages.dev`; no deployment yet.
- GitHub environment: `website-production`, selected deployment branch `main`.
- Environment variables: account ID above, `CLOUDFLARE_PAGES_PROJECT=qwacamole`,
  and `WEBSITE_DEPLOY_ENABLED=false` while setup is incomplete.
- A separate account-scoped Pages Edit API token must be saved by the owner as
  environment secret `CLOUDFLARE_API_TOKEN`. Wrangler's interactive OAuth login
  was used only locally; its credentials were not copied into GitHub.
- No custom domains, DNS records, nameservers or mail settings were changed.

Wrangler 4.149.0 delegates creation of a nonexistent Pages project to Workers in
an agent session. Its hidden `--force` option explicitly selects Pages creation;
that created the Direct Upload project without a Worker. Subsequent deployments
select the existing Pages project and need no change to the upload command.

## Live preflight and correction

Cloudflare rejected the existing history request
`deployments?env=production&per_page=100&page=1` with HTTP 400, code `8000024`
(`Invalid list options provided`). Requests for 20 and 25 entries succeeded;
50 was also rejected. The production guard now requests 25 entries and still
traverses every page, retaining the 100-page bound and fail-closed validation.
The successful empty response included `page=1`, `per_page=25`, `count=0`,
`total_count=0` and `total_pages=0`.

The corrected `deploymentFreshness()` ran against live Cloudflare and GitHub
read APIs, checking main source `e3c9c1c9322c1bee9cbf3fa1cd5177be39c3aa33`.
It returned: `First production deployment; no successful production source
exists.` This was a read-only preflight, not an upload.

The prior main website run
[39040405648](https://github.com/cztamas/qwacamole/actions/runs/39040405648)
passed its build/browser checks. Its downloaded static artifact reports the
same full source SHA and contains 73 files, largest 235,794 bytes. That artifact
was inspected but not deployed.

## Local validation

Using Node 24.21.0 and the existing lockfile (Wrangler 4.149.0):

- From `website/`, `node --test scripts/deployment-setup.test.mjs scripts/deploy.test.mjs`:
  10 tests passed. The initial root-directory invocation failed because the
  setup test resolves workflow paths relative to `website/`; rerunning from the
  documented directory passed.
- `npm run check`: Astro check and all 14 script tests passed. The approved
  first-upload test now rejects the API-incompatible page-size request.
- `npm run build`: 16 HTML pages, 703 internal links/anchors/assets checked;
  static search has 14 indexed pages and 29 complete assets. Publication
  allowlist check passed.
- `npm run test:browser`: all 16 Chromium checks passed, including desktop/
  mobile routes, navigation, search and 404. The sandbox initially blocked the
  preview server's localhost listener; the authorized rerun outside that sandbox
  passed.
- `git diff --check`: passed.

Local command logs and filtered preflight results are in ignored
`.local/website-deployment-20261017/`. No credentials are recorded there.

## Remaining activation

Land the pagination fix, save the scoped environment secret, then enable the
existing workflow and dispatch it from main. Require a successful upload and
matching immutable `/build-info.json` before claiming hosting is live. Verify
landing, quickstarts, search and 404 on Pages before connecting the custom
domain through the separately documented domain setup.
