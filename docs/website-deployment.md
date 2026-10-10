# Website: Cloudflare Pages setup and rollback

The static Astro/Starlight site lives in `website/`. GitHub Actions builds it
without Cloudflare credentials. Production deployment is **disabled by default**
and has not been verified on qwacamole.com. This guide prepares setup; it does
not authorize DNS, nameserver, account or repository visibility changes.

## One deployment mechanism

Use Cloudflare Pages **Direct Upload** through the dedicated GitHub workflow.
Create a new Direct Upload project (do not connect GitHub), using Wrangler's
project creation command from an authenticated maintainer terminal:

```sh
cd website
npm ci
npx --no-install wrangler pages project create qwacamole --production-branch main
```

Choose your actual project name. This one-time command needs your separately
authorized Cloudflare login/account. Do not run it as part of a PR build.
[Direct Upload](https://developers.cloudflare.com/pages/get-started/direct-upload/)
projects cannot later switch to Git integration. If an existing project already
uses Git integration, disable its automatic production and preview builds before
using Direct Upload; avoid duplicate deployment mechanisms.
[Git integration settings](https://developers.cloudflare.com/pages/configuration/git-integration/).

## Configure GitHub, then enable

Create a token with **Account → Cloudflare Pages → Edit**, scoped to **Include →
the specific owning account**. It needs no DNS/zone edit permission. Keep account
setup and domain changes in the dashboard under separately authorized access.
See [Cloudflare's CI token instructions](https://developers.cloudflare.com/pages/how-to/use-direct-upload-with-continuous-integration/).

The deployment job uses the `website-production` GitHub environment. Configure
it with main-only deployment branches; optionally add maintainer reviewers.
Set these environment variables and secret (repository-level equivalents also
work; environment-scoped credentials are preferred):

| Kind | Exact name | Value |
| --- | --- | --- |
| Variable | `CLOUDFLARE_ACCOUNT_ID` | Owning account's ID |
| Variable | `CLOUDFLARE_PAGES_PROJECT` | Existing Direct Upload project name |
| Secret | `CLOUDFLARE_API_TOKEN` | Account-scoped Pages Edit token |
| Variable | `WEBSITE_DEPLOY_ENABLED` | `true` after completing setup; absent/`false` disables uploads |

Leave the enable variable absent/false until the project, production branch,
credentials and permission scope have been verified. Disabled runs still check
and build; their summary explains the missing setup and performs no upload.
Once enabled, missing values, invalid credentials, upload or identity-verification
failures **fail the deployment job**. Do not use continue-on-error.
Before upload, the workflow reads the Pages project and rejects a production
branch other than main or still-enabled Git production/preview builds.
[Project API](https://developers.cloudflare.com/api/resources/pages/subresources/projects/methods/get/).

The workflow runs website checks, link/anchor/asset validation and browser smoke
on relevant PRs and pushes to main. It uploads exactly the checked static artifact.
Only main push/manual dispatch can enter deployment. Deployment concurrency
serializes production uploads. PR jobs receive no Cloudflare credentials and
create no previews. GitHub needs only contents: read; Wrangler directly uploads,
without writing GitHub deployment records. Freshness uses contents-read GitHub
source comparisons inside that production slot; see the rules below.

## First deployment and verification

Dispatch `Documentation website` **from main** after setup, or merge a relevant
documentation change. Select a fresh successfully checked source. The workflow
summary records full Git SHA and the immutable Pages deployment URL. It verifies
that URL's `/build-info.json` reports the same SHA after upload. A docs deployment
does not publish a Go release or make the source repository public.

Before connecting a custom domain, visit the Pages URL and verify HTTPS, landing,
both quickstarts, search, 404 and `/build-info.json`. Retain the run URL, deployment
URL/ID and commit together. Source and release links currently require repository
authorization; public docs hosting does not change that.

## Connect qwacamole.com deliberately

Canonical URL in Astro is **https://qwacamole.com**. We have not checked the
domain's current DNS provider and do not assume Cloudflare nameservers.

For an **apex** Pages domain, Cloudflare requires the domain as a zone in the
owning account and its assigned authoritative nameservers at the registrar.
Inventory existing DNS records and DNSSEC before a separately approved migration;
preserve mail and other services. This is more than adding an apex CNAME.

In Pages → project → Custom domains, add `qwacamole.com` and complete its
activation. Merely adding a DNS record without associating the hostname with
Pages is insufficient. For an external-DNS **subdomain**, Pages supports CNAME
to the project's pages.dev hostname after adding it in Pages; this does not
remove the apex nameserver requirement.
[Custom-domain requirements](https://developers.cloudflare.com/pages/configuration/custom-domains/).

If `www.qwacamole.com` is configured, associate it with the project and use a
Cloudflare redirect rule/Bulk Redirect to the apex, preserving path and query.
Do not add a hostname redirect until its DNS and HTTPS are ready.
[www to apex](https://developers.cloudflare.com/pages/how-to/www-redirect/).
If choosing a different canonical hostname, change Astro's site URL and landing
canonical together and rebuild. This PR performs no domain or DNS changes.

After activation, check normal HTTPS (no insecure override), certificate hostname,
all three main routes, search and `https://qwacamole.com/build-info.json` against
the workflow SHA. The static site does not use qwacamole's Go TLS integrations.

## Rollback

Temporarily set `WEBSITE_DEPLOY_ENABLED=false` to prevent the next automatic
upload while investigating. In Pages → Deployments, select a **previous successful
production** deployment and choose **Rollback to this deployment**. Preview
deployments are not valid rollback targets.
[Cloudflare rollback procedure](https://developers.cloudflare.com/pages/configuration/rollbacks/).

Verify its Pages/custom-domain HTTPS and build-info SHA against the retained
record. Revert the bad website/docs change through a new PR before re-enabling,
or a subsequent successful main workflow will replace the rollback. For a
rebuild/redeployment use manual dispatch from main after required checks.
Rollback changes served files, not DNS, token scope or repository history.

## Free-plan fit and limits

This is a small static site with no Functions, backend, paid search, database or
runtime Go services. Static Pagefind search is generated at build time. Keep
within Pages Free's **20,000 files**, **25 MiB per asset** and **100 custom domains**
per project. Cloudflare-hosted builds have **500 builds/month** and one concurrent
build on Free; this workflow builds on GitHub, whose own Actions quotas apply.
These are current plan limits, not a promise of unlimited free service.
[Pages limits](https://developers.cloudflare.com/pages/platform/limits/)
(checked 9 October 2026). Recheck plan/usage before changing scope.

## Freshness inside the production slot

Uploads remain serialized with `cancel-in-progress: false`. After setup and
checked-artifact identity validation, the deployment script reads the Pages
project and **all retained production deployment pages** while holding that slot.
It compares full source commits with GitHub's contents-read comparison API.
No commit string, workflow timestamp or build completion order is used as age.
[Pages deployment history API](https://developers.cloudflare.com/api/resources/pages/subresources/projects/subresources/deployments/methods/list/)
and [GitHub commit comparison](https://docs.github.com/en/rest/commits/commits#compare-two-commits)
provide the inputs; `GITHUB_TOKEN` is used only for repository reads.

- A checked source must remain an ancestor of main. A later Go-only commit is
  allowed: the guard does **not** require exact HEAD equality. Existing path
  filters still decide which changes start a website build.
- A source older than any retained successful production source is **skipped**,
  with both commits and the reason in the workflow summary. A rerun of an already
  deployed source also skips. No Wrangler upload is attempted. The same rule
  covers delayed builds, reruns and manual dispatch of superseded workflows.
- First deployment requires an explicit null canonical deployment and complete
  history showing no successful production source. API denial, missing metadata,
  unknown/diverged ancestry, incomplete pagination or an unfinished production
  write fails clearly before upload. History traversal is capped at 100 pages;
  exceeding that bound fails rather than assuming freshness.
- Successful deployment history is the high-water mark even after a dashboard
  rollback. Keep those history records; do not delete them to bypass this guard.
  Cloudflare metadata must contain a clean full commit and main branch. An older
  deployment created outside this mechanism with unknown source blocks uploads
  until its identity/history is investigated under a separate maintainer action.

On rollback, disable uploads first and stop pending deployment jobs **before**
external writes begin. Never cancel an active upload to order jobs. Roll back
through Pages, verify its immutable identity, then create a new main commit that
reverts the faulty content. Dispatch/check that new descendant after re-enabling;
an old workflow or an identical old source cannot be used to undo the high-water
mark. Serialize separately approved manual Pages writes with this workflow;
GitHub concurrency cannot lock a dashboard or another external writer.

The upload still uses the checked artifact, sets its full commit metadata and
verifies immutable deployment URL/build-info identity. Failed or uncertain writes
are never retried automatically. Inspect Pages before any subsequent action.
