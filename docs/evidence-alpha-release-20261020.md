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
| Full release-graph workspace/independent tests | In progress; no success claimed yet |
| Sequential native Chrome/Firefox acceptance | Pending release-graph builds |
| Website check/build/browser tests | Astro diagnostics, 19 script tests, 16 pages, 702 internal links/assets and 16 Chromium browser tests pass |
| History audit | Refreshed Gitleaks full reachable history, inline ignores disabled: only the same five exact reviewed public-fixture findings |

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
