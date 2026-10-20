# Alpha pre-publication preparation — 17 October 2026

**Preparation and owner publication approval are recorded for review.**
The repository and website are public. The release environment now accepts only
main and retains its maintainer reviewer. Fresh main packages, their legal and
security inventories, and an independent reproducible rebuild pass. Anonymous
Go installation is still an unresolved prerequisite, described below. The owner
subsequently requested the approval record described in the addendum. No tags or
GitHub release were created.

## Source and external settings

Baseline main: **`3473b64a74fad6671347f07ac3e75390a660b26f`**.
This preparation changes documentation, website availability copy and the owner
approval policy, with no production Go, dependency-pin or release-tool changes. Earlier reports remain
historical, especially the [16 October runtime/browser report](evidence-alpha-readiness-20261016.md).

- The repository API reports `private: false` and default branch `main`.
- PR #10 is merged at `e3c9c1c9322c1bee9cbf3fa1cd5177be39c3aa33`.
  The retained `9b96642914dc07ebf93e9c4c9e4afb3037caa5a6`,
  `e6198e8552b85b7bae72429cb4ee3fc2e6fada07` and
  `6b966e265f5535167da8413f2c7b7b2964a5e873` sources are ancestors of main.
- GitHub private vulnerability reporting is enabled. SECURITY.md retains
  `tamas.czank@gmail.com` as the primary contact, with best-effort support and
  no response guarantee. Inbox monitoring was not independently tested.
- `public-alpha-release` retains required reviewer `cztamas`. Its deployment
  branch policy was changed to custom policies with the sole rule `main`, type
  `branch`; protected-branches mode is false. The actual environment and branch
  API responses pass `publish-alpha.py`'s `validate_environment` function.
  Existing administrator bypass and self-review settings were preserved.
- Initial checks used `approved: false` and no owner-decision URL, at proposed
  version `0.1.0-alpha.1`. The later owner approval is recorded below.
  No release or module tag exists.

The live website is independent of software publication. The successful
[production workflow 39055585134](https://github.com/cztamas/qwacamole/actions/runs/39055585134)
deployed this baseline to Cloudflare Pages deployment
`096c6668-440f-4ed0-b170-713518d58c03`. Apex `qwacamole.com` is active with valid
HTTPS and matching build-info. The owner's DNS record is a proxied apex CNAME
to `qwacamole.pages.dev`. Public/authoritative DNS checks succeeded; the owner's
macOS resolver retained a negative result during activation. The 16 live browser
checks used an explicit resolver mapping to the public address, with normal TLS
verification. They establish site behavior, not recovery of that local resolver.
`www` activation/redirect is not claimed. See the [deployment guide](website-deployment.md).

## Fresh baseline candidate

[Manual main candidate run 39059507003](https://github.com/cztamas/qwacamole/actions/runs/39059507003)
passed at the exact baseline source, including Linux checks and native macOS
verification. It produces **unapproved candidates**, not release assets.

| Identity | Reviewed value |
| --- | --- |
| Version / manifest schema | `0.1.0-alpha.1` / `2`, `candidate: true` |
| Agent/Caddy source | `3473b64a74fad6671347f07ac3e75390a660b26f` |
| Core version | `v0.0.0-20261016203648-9b96642914dc` |
| Core source | `9b96642914dc07ebf93e9c4c9e4afb3037caa5a6` |
| Toolchain | Go `1.27.2`, `GOEXPERIMENT=jsonv2`, CGO disabled |
| Caddy / CertMagic | `2.11.7` / `0.25.6` |
| Manifest SHA-256 | `5a97e0b904789138ded9f9ae5261909bcf4ccf0d78e74b9ba9743da35b0f8ab7` |
| Candidate artifact retention | `2026-11-16T18:48:32Z` |

The downloaded archives passed `release-agent.py verify --smoke --require-native`.
The sanitized [candidate identity](evidence/alpha-prepublish-20261017/candidate-identity.json)
and [archive checksums](evidence/alpha-prepublish-20261017/SHA256SUMS) retain the
reviewed values without committing executables.
The actual extracted macOS arm64 agent reports the expected source, core,
compiler and platform identities, with `modified: false`. A separate local
`release-agent.py build` from `git archive` of the baseline produced byte-identical
**SHA256SUMS and manifest.json** to the hosted artifacts. This rebuild used the
normal local module environment/cache; it is not anonymous installation proof.

Root, agent and Caddy LICENSE/NOTICE bytes match the policy hashes and every
archive's legal files. Strict package verification checks the allowed members,
file modes, checksums, dependency boundary and module inventory. The fresh
candidate [security summary](evidence/alpha-prepublish-20261017/security-summary.json)
contains ten passing enforcing scans: four modules,
two production commands and four extracted binaries. Database metadata was
retrieved at `2026-10-17T18:48:05.043370+00:00`, with database modification time
`2026-10-08T22:31:09Z`. The existing exact-version disposition for unreachable
OpenPGP advisory `GO-2026-5932` remains; no affected package/reachable symbol is
reported. Both escaped-path routing regressions return 403 with `DENY` from the
actual packaged Caddy, with no binding gate configured
([routing evidence](evidence/alpha-prepublish-20261017/candidate-routing.json)).

The [binary installation guide](guides/binary-installation.md) now pins this
fresh candidate's source, run, manifest and actual retention deadline. Linux
arm64 remains cross-compiled only. This does not replace final approved E
packages or the required exact-source main checks after release repinning.

## Checks and limits

| Check | Result |
| --- | --- |
| Release Python tests | 35 pass, including the full mocked approved gate, artifact mismatch, existing release and API failures |
| [Manual main portable integration 39059504421](https://github.com/cztamas/qwacamole/actions/runs/39059504421) | Success at the baseline source: four-module races, independent modules, boundaries/security, restrictive-umask setup, Nginx serving/upgrade and Caddy ACME/lifecycle checks |
| Live external validators | Environment reviewer/main-only policy and enabled private reporting pass |
| Non-publishing preflight in a clean baseline checkout | Expected refusal: `owner publication approval missing/version mismatch`; later gate stages were not reached |
| Website check | Astro diagnostics and 19 script tests pass |
| Website production build | 16 HTML pages, 704 internal links/anchors/assets; 14 indexed product pages, 29 search assets; publication boundary passes |
| Website browser tests | 16 Chromium checks pass, including desktop/mobile, search, accessibility and product boundary |

The preflight used the exact baseline source, candidate run and reviewed manifest
above, **without `--publish`**. It did not pass the complete approved gate and
does not verify the GitHub publication job's credential. That full live preflight
still requires the recorded approval on main, real tags/pins and final approved artifacts.

Fresh Gitleaks scans covered all locally reachable refs with `--all --full-history`
and a tracked working-file snapshot, with inline ignores disabled and redacted
output. Each scan reported the same five reviewed public-fixture findings: four
RFC 6455 example WebSocket nonces and the exact Mozilla fixture key (SHA-256
`0560d06fc2d28c8c66b873f823542dea3abd99b212b2ced76e9d5f331a1afccd`).
Historical blobs and current matching lines were checked against the earlier
dispositions. All 30 complete decompressed candidate members also passed a
bounded scan for full private-key PEM and recognized GitHub/OpenAI tokens, with
no artifact exception. Hidden remote refs, unreachable objects and unknown
secret formats are outside this audit; absence is not universally established.
The [sanitized audit](evidence/alpha-prepublish-20261017/content-audit.json)
records the scope, exact public-fixture dispositions and candidate scan count.

Native credential/lifecycle and Chrome/Firefox acceptance was not rerun for
these documentation changes. The prior Chromium expired-`exp` negative remains
explicit: browser identity display after binding expiry is a known limitation,
while server-side expiry enforcement passes. No assertion or support boundary
was weakened.

## Unresolved anonymous installation

A fresh environment from `check-public-install.py`'s `anonymous_environment`
helper disabled Git config, tokens, interactive authentication, private-module
overrides, workspace use and local replacements. HOME and Go caches were new;
`GOPROXY=https://proxy.golang.org` and `GOSUMDB=sum.golang.org` remained enabled.
Core, agent and Caddy metadata at the documented `6b966…` source resolved to the
correct full origin SHA. The agent installation then failed downloading the
retained core ZIP at `v0.0.0-20261016203648-9b96642914dc`: HTTP 404 with
`not found: fetch timed out`.

A separate fresh, credential-free diagnostic with `GOPROXY=direct` reached the
exact core origin `9b96642914dc07ebf93e9c4c9e4afb3037caa5a6`, but checksum
verification of its go.mod failed at `sum.golang.org/lookup/…`: HTTP 404,
`server response: not found:`. The cause and duration of this public-service
availability failure are unresolved; this does not establish a code defect or
a successful anonymous install. The source guide now states the limitation and
points to the verified authenticated package path. Checksums were not disabled.

## Owner approval addendum

On 17 October 2026, after this preparation PR was opened, the owner instructed:
**“Record the publication approval.”** This records approval for the documented
experimental **`0.1.0-alpha.1`** publication under the selected standard MIT
license, retaining the policy's exact LICENSE/NOTICE hashes and support limits.
`release/publication.json` now has `approved: true` and the real review URL
**https://github.com/cztamas/qwacamole/pull/14**. The PR body records the decision;
the policy change reaches main only after merge. Earlier refusal results and the
baseline candidate's `candidate: true` identity remain historical and unchanged.
All 35 release tests passed again after this policy change. The approved legal-file
validator accepted the decision URL/version and exact LICENSE/NOTICE hashes;
agent/Caddy legal copies still match.

Subsequent candidate builds read this approval and can mark packages approved;
they are still not publishable until the full gate verifies final tagged core
requirements, exact-source main runs, artifact identity and reproducibility.
This request records approval without executing tag creation or release publication.

## Remaining publication actions

Follow the [canonical launch checklist](alpha-release.md): resolve anonymous
dependency availability; merge the recorded approval and review final source/inventory;
perform the immutable C/D/E tag and repinning sequence; run final-source manual
main integration/candidate workflows and review their manifest/rebuild; pass
anonymous tagged module/xcaddy checks and the full live preflight; then execute
the approved publication through its protected main workflow. After publication, verify actual public
assets and installed runtime identities. No merge, tag creation or release
publication was executed as part of recording this approval.
