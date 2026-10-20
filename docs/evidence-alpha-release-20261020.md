# 0.1.0-alpha.1 release verification — 20 October 2026

This report records the release-source preparation and acceptance checks. The
[release page](https://github.com/cztamas/qwacamole/releases/tag/v0.1.0-alpha.1)
is authoritative for publication status and the final verified C/E source,
candidate-run and manifest identities. The final commit cannot embed its own SHA
or subsequent workflow IDs; `publish-alpha.py` appends those verified values to
the release body. Earlier dated reports retain their historical results.

## Source sequence and scope

- PR #14 merged at **`dc0a66e4e8b19de0ae44341af389994c154c1957`**, selected as
  core **C**. Its approved standard MIT/NOTICE hashes match every module copy.
  Core production code is unchanged from the previously validated `9b966…`
  source. Existing prerequisite sources remain ancestors of main.
- [Manual main integration 39469414439](https://github.com/cztamas/qwacamole/actions/runs/39469414439)
  passed at C: four-module races, independent modules, boundaries/security,
  Ubuntu restrictive-umask setup, real Nginx serving/upgrade and Caddy lifecycle.
  Fresh local core races and all 35 release tests also passed.
- Immutable root tag **`v0.1.0-alpha.1`** points to C. Public proxy/checksum
  retrieval passed and returned exact origin C, module checksum
  `h1:Im9iQ37FfDEBcgRV0YMOBLPM+Ci5MyA5W9lvHIbCCxo=` and go.mod checksum
  `h1:0dIQ45E9fea0OdtdVp8Y11lQ8cWTgkaL7lL42UN1F/k=`. The 17 October
  proxy/checksum failure remains historical; no checksum bypass was used.
- Production agent/Caddy commit **D** is
  **`c2c664135627f54c393941b64f2755a6872843a1`**. Only their core requirements
  and checksum files changed. Both now require `v0.1.0-alpha.1`; independent
  race suites passed before D was pushed.
- Devtools resolves both production modules at their actual D pseudo-version
  **`v0.0.0-20261020180210-c2c664135627`**, with root `v0.1.0-alpha.1`.
  Public metadata returned exact origin D; independent tidy verified downloads.
  The selected final nested **E** must preserve D's complete agent/Caddy trees.

No production implementation, credential validation, binding/store transaction,
activation gate, serving default or browser assertion changes in this sequence.
Operator documentation switches to release-version installation and directs
users to check actual release availability. The package guide obtains the reviewed
manifest digest from the independently rendered release identity before checking
assets; an Actions candidate cannot substitute for the release.

## Publication workflow

The existing protected main workflow now has a boolean `publish` input, default
**false**. A false run omits `--publish`, validates all source/API/artifact gates
and renders notes. An explicit true run executes the existing single-publication
path. Both modes use the same environment and job credential/permissions.
`actionlint` passes; no environment protection or refusal is removed.

Final publication still requires immutable core/agent/Caddy tags, matching nested
core requirements, successful manual main integration/candidate runs at E,
reviewed manifest, approved legal files, reporting, reproducible rebuild and no
existing release. Run the workflow first with `publish=false`, review its actual
output, then use `publish=true`. Never retry an uncertain publication without
inspecting the resulting release.

## Newly executed acceptance

Raw generated keys, browser profiles, caches and logs stay under ignored
`.local/alpha-release-20261020/`. No generated credential or profile is committed.

| Check | Result at this preparation checkpoint |
| --- | --- |
| Core C local race tests / release Python tests | Pass / 35 pass |
| Agent/Caddy independent D race tests | Both pass |
| Public core tag and D module identities | Exact origins and checksums verified |
| Hosted review source `47e2952…` | Candidate `39471865603`, portable integration `39471865470` and website `39471865461` all pass; website deployment skipped for PR |
| Full release-graph workspace/independent tests | Both pass, including all four modules and dependency boundaries; no independent replacement |
| Native Caddy / Chrome | Lifecycle passes; ten fixtures pass, unchanged expired-`exp` negative fails as the documented incompatibility |
| Native Caddy / Firefox 158.0 | All eleven fixtures and lifecycle/cache/replacement/restart/failure-recovery checks pass |
| Installed Nginx package / Chrome and Firefox | Both pass all 24 lifecycle checks, including actual Certbot renewals, cached identity after expiry/replacement, uploads, WebSocket, collection and restart |
| Website check/build/browser tests | Astro diagnostics, 19 script tests, 16 pages, 702 internal links/assets and 16 Chromium browser tests pass |
| History audit | Refreshed Gitleaks full reachable history, inline ignores disabled: only the same five exact reviewed public-fixture findings |
| Tracked tree and review-package audit | Same five exact public-fixture source findings; no full private-key PEM or recognized GitHub/OpenAI token in all 30 decompressed archive members |
| Archive authentication regression | Actual archives match reviewed-manifest digests; replacing an archive and regenerating SHA256SUMS passes the old check but fails the new direct manifest comparison |

Chrome is **155.0.8059.85**, with genuine installed PKIMetadata **1810**.
Its lifecycle summary preserves positive initial/renewed identity, cached
discovery after expiry/renewal/replacement, held HTTP/2 through TLS expiry plus a
complete binding lifetime, replacement, restart, independent refresh and failure
recovery. `negative_suite_passed` remains **false**, and the only failed fixture
is `expired-binding`; the failing assertion was not weakened.

The verified review package at source
`47e2952f89e39f99a38a38b196d4b53d6516d281` was extracted and installed into a
new local macOS arm64 prefix. Its agent reports version `0.1.0-alpha.1`, core
`v0.1.0-alpha.1`, Go 1.27.2 and `modified: false`. Its production trees equal D. Final selected E
packages must still pass their own exact identity, rebuild and runtime checks.

Fresh runs were sequential, with one new state/profile per run. Firefox used
the already staged, stock signed **158.0** executable after strict signature
verification; no application update or user preferences were installed. The
browser harness contains development adapters, while the Nginx checks use the
actual extracted/installed production package. Production trees match D exactly.
Sanitized [Chrome suite](evidence/alpha-release-20261020/native-caddy-chrome-browser-suite.json),
[Chrome lifecycle](evidence/alpha-release-20261020/native-caddy-chrome-integration.json),
[Firefox suite](evidence/alpha-release-20261020/native-caddy-firefox-firefox-suite.json),
[Firefox lifecycle](evidence/alpha-release-20261020/native-caddy-firefox-firefox-integration.json),
[Nginx Chrome](evidence/alpha-release-20261020/native-nginx-chrome.json) and
[Nginx Firefox](evidence/alpha-release-20261020/native-nginx-firefox.json) retain
the results without keys, full profiles or raw browser logs.

The five history findings are four RFC 6455 public example nonces and the exact
Mozilla fixture key, whose SHA-256 remains
`0560d06fc2d28c8c66b873f823542dea3abd99b212b2ced76e9d5f331a1afccd`.
Every historical blob was compared with its previous disposition. Hidden remote
refs, unreachable objects and unknown secret formats are outside this bounded
audit. Final tree and complete decompressed package audits remain required.

Native acceptance preserves the Chromium expired-binding negative assertion.
The known ignored-`exp` result is an explicit browser limitation, while all other
fixture and lifecycle checks must pass. Firefox compatibility remains explicit;
synthetic roots stay confined to fresh disposable profiles. No production or
user browser trust is expanded. Final anonymous tagged installations and package
runtime checks remain separate from development-harness acceptance.

## Final publication and public verification

The [0.1.0-alpha.1 prerelease](https://github.com/cztamas/qwacamole/releases/tag/v0.1.0-alpha.1)
was published on **20 October 2026 at 19:07:39 UTC**. Final nested source E is
**`be065af605ce3f7be1eaf8981df343c82082aebf`**, the merge of PR #15, preserving D.
Both nested module tags point to E; the core tag remains C. Their complete
production trees equal the independently tested D trees.

- Exact-main [integration 39476496412](https://github.com/cztamas/qwacamole/actions/runs/39476496412)
  and [candidate 39476501314](https://github.com/cztamas/qwacamole/actions/runs/39476501314)
  passed at E. All ten enforcing source/command/binary security scans and the
  packaged Caddy routing regression passed.
- Downloaded final candidate and independent approved rebuild have identical
  complete SHA256SUMS and manifest bytes. Reviewed manifest SHA-256:
  **`e8c28682e15ff637dbb01f293ebc4e306a83ba9e2512818bfde88372410edad8`**.
  Final archives' legal files match policy/source; all 30 decompressed members
  passed the bounded key/token scan without an artifact exception.
- Fresh credential-free tagged core/agent/Caddy installs resolved exact C/E
  origins, with public checksums, no workspace/replacements or private-module
  overrides. Actual anonymously installed Caddy and Nginx-agent smokes passed.
- Protected [preflight 39479178016](https://github.com/cztamas/qwacamole/actions/runs/39479178016)
  passed with `publish=false`, using the actual publication job credential and
  unchanged environment protection. Its rendered identities were reviewed.
  [Publication 39480194073](https://github.com/cztamas/qwacamole/actions/runs/39480194073)
  then passed with identical inputs and `publish=true`. The configured eligible
  owner reviewer approved each environment gate; no protection was disabled.
- All six public assets downloaded over verified HTTPS in a credential-free
  curl environment and match the reviewed candidate. Public-package verification,
  direct manifest/archive checksum comparison and native smoke pass. The actual
  public macOS package installs into a fresh prefix and reports E, tagged core,
  Go 1.27.2 and `modified: false`.
- [Website deployment 39475969519](https://github.com/cztamas/qwacamole/actions/runs/39475969519)
  passed. Ordinary public HTTPS on qwacamole.com returned E's build-info with no
  DNS override; later documentation-only commits may advance the website.

The public release-page check found that GitHub rendered template-relative
`guides/…` links from the core tag, omitting `docs/` and selecting the wrong
component source. Only those six release-body documentation links were corrected
to immutable E paths; the verified identity section was preserved exactly.
The future renderer now performs that conversion, with a regression covering
guide anchors, source selection and unchanged external/mail/local-anchor links.
**36 release tests pass.** Published tag/source and artifact identities remain
the values above.

[Sanitized final verification](evidence/alpha-release-20261020/publication-verification.json)
records public asset sizes/hashes, workflow identities, public native version,
verification scope and the unchanged browser/platform limitations. Full raw
evidence remains in the ignored directory named above.
