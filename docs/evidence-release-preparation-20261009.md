# Alpha release preparation — 9 October 2026

Baseline is merged main `08872250a0813f232faaca9794a9e5b481b4d19b`.
Release branch: `codex/public-alpha-release`; review PR:
https://github.com/cztamas/qwacamole/pull/7. This report records newly executed
preparation checks. Earlier lifecycle/streaming reports remain historical evidence;
none has been rewritten. Engineering preparation is complete; public launch is
still subject to the explicit owner prerequisites below.

## Source and package identity

Agent CLI/version implementation: `bb911a8f69a8743faab70967232ff166c828c4b7`,
real module version `v0.0.0-20261009145422-bb911a8f69a8`. Corrected packager/manual
publication tooling and devtools real agent pin: `cb7d66730030600edbfc0cde5fafee106e3f8df9`.
Core remains `8e62f4b8c340e572584f6d71355a8f89a7d0be42`, version
`v0.0.0-20261008203612-8e62f4b8c340`; Caddy remains
`9322bd566e825a2c2c645e464a630e09d7aa68d6`, version
`v0.0.0-20261009072701-9322bd566e82`. No future module tag is added to Go manifests.

The candidate tested through extracted-binary Nginx acceptance stamps full source
`bbf5117f309cb583ea5aaad8ffdd481028b77f16`. Remote Go installation identifies the
actual agent bb911a8 module. Agent/core/Caddy runtime Go code is unchanged between
these sources and subsequent release-tool/documentation corrections. These
identities are deliberately distinguished; final publication must repeat the
checklist against its selected approved sources.

The [candidate manifest](evidence/release-preparation-20261009/candidate-manifest.json)
and [checksums](evidence/release-preparation-20261009/candidate-SHA256SUMS.txt)
record all three archive/member hashes, modes, source/core and toolchain. Manifest
SHA256: `db6f7a8bf245308549329a67dfd3e33bb8c3ef815ebe2e90cefc0d2d9fb493f1`.
These are **private review candidates**, not released assets or a licence grant.
Future publication records core C separately from nested agent/Caddy E; GitHub's
automatic source archive follows C, agent packages and nested module tags follow E.
The approved source cannot contain its own future SHA or later CI run ID: the
publisher renders those verified identities into the release body without changing E.

## Tested environments

Fresh local platform: **macOS 26.5 arm64**. Go **1.27.1** with
`GOEXPERIMENT=nojsonv2` (binary runtime `go1.27.1-X:nojsonv2`), xcaddy **0.4.5**,
Caddy **2.10.2**, CertMagic **0.24.0**, Pebble **2.8.0**; Nginx **1.30.5**,
Certbot **5.8.0**; Chrome **154.0.8037.99** with genuine PKIMetadata **1798**;
Firefox **157.0.1**, geckodriver **0.37.1**, Homebrew certutil.

Fresh hosted portable checks use Ubuntu **24.04 amd64**, runner image
`20261004.327`, Nginx **1.24.0-2ubuntu7.18**, Certbot **5.8.0**, and the pinned
Go/Caddy tools above. Candidate CI runs extracted binaries on Linux amd64 and
macOS arm64 (`macos-15`). **Linux arm64 is cross-compiled only**. Service-manager
examples, Windows, macOS notarization and Caddy's hosted builder are untested.

## Newly completed engineering checks

- Config-free agent `version` tests pass under race detection. Sixteen Python
  release-tool regressions pass, covering corrupt/extra artifacts, unsafe paths,
  symlinks, skipped native runs, invalid source/version/tag maps, missing approval,
  actual publication-environment protections, rendered source identity and
  credential-free installer isolation. PR checks against GitHub merge commits
  cannot authorize publication: both required workflows must succeed as manual
  main runs at the selected source. Wrong-source and unapproved-policy tests
  reject before API access or publication. Raw log:
  `.local/evidence/release-preparation-20261009/release-tool-tests.log`.
- `make build agent-build plugin-build`, `make test`, and
  `scripts/check-independent-modules.sh` pass. All four module races and dependency
  boundaries pass; some unchanged Go packages use valid cached results.
  Independent devtools/coretest races run for 141 seconds against real new agent
  pins. Raw logs: `.local/evidence/release-preparation-20261009/{builds,all-module-races,independent-module-races}.log`.
- The already-merged streaming fix was freshly rechecked with uncached race
  tests: delayed headers across expiry plus endless/stalled bodies over HTTP/1.1
  and HTTP/2 all pass, including prompt clean 503, no protected output, upstream
  cancellation and serving completion. Unrelated/valid-response panics still
  propagate. Raw log: `.local/evidence/release-preparation-20261009/streaming-review-races.log`.
- Fresh outside-checkout authenticated immutable agent/Caddy installs pass with
  fresh module caches, `GOWORK=off` and no replacements. The production agent has
  no Caddy/browser/devtools dependency; Caddy has no devtools. Exact full origin
  SHA/version/build metadata: [agent](evidence/release-preparation-20261009/remote-agent-install.json),
  [Caddy](evidence/release-preparation-20261009/remote-caddy-install.json).
- Real smoke using those remotely installed binaries: Caddy **26/26**, agent
  **34/34**. Evidence: [Caddy runtime](evidence/release-preparation-20261009/caddy-runtime.json),
  [agent runtime](evidence/release-preparation-20261009/agent-runtime.json).
- Repeated builds at cb7d667 produce byte-identical SHA256SUMS. Independent
  macOS builds at 30a79ef and bbf5117 also match **every checksum of their Linux CI
  builds**, including the manifest. Extracted macOS binaries match all JSON
  identity fields. Archives contain exactly five allowlisted files with validated
  permissions, normalized ownership/timestamps and hashes. Initial bb911a8
  verification correctly failed on runtime/toolchain version mismatch; corrected
  manifests distinguish those fields. [Check summary](evidence/release-preparation-20261009/checks.json).
- The extracted bbf5117 macOS package passes config-free version, key generation,
  CSR generation and independent OpenSSL CSR-signature verification. Key/CSR
  modes are **0600**, and stdout contains no key material.
  [Exact CLI commands/results](evidence/release-preparation-20261009/package-cli.json).
  That extracted binary also passes **34/34 real Nginx/Certbot checks**, including
  WebSocket, automatic renewal, held streams, rejection/recovery and capacity
  refusal: [packaged runtime](evidence/release-preparation-20261009/packaged-agent-runtime.json).
- Hosted candidate run [37951376820](https://github.com/cztamas/qwacamole/actions/runs/37951376820)
  succeeds at bbf5117; retained artifacts were downloaded and verified locally.
  Linux amd64 and macOS arm64 extracted native checks pass.
  [Candidate metadata](evidence/release-preparation-20261009/hosted-candidate.json).
- Hosted portable run [37951376881](https://github.com/cztamas/qwacamole/actions/runs/37951376881)
  succeeds at bbf5117, including all four module races, independent resolution,
  boundaries, production builds, real default/custom Nginx serving, immutable
  baseline upgrades and Caddy/Pebble integration. Every job step succeeded.
  [Full step metadata](evidence/release-preparation-20261009/hosted-go.json).
  Raw ignored log: `.local/evidence/release-preparation-20261009/hosted-go.log`.

Package build commands use:

```sh
python3 scripts/release-agent.py build --source bbf5117f309cb583ea5aaad8ffdd481028b77f16 \
  --version 0.1.0-alpha.1 --output .local/release-local-bbf5117 --go .tools/go/bin/go
python3 scripts/release-agent.py verify .local/release-ci-bbf5117 --smoke --require-native
diff -u .local/release-ci-bbf5117/SHA256SUMS .local/release-local-bbf5117/SHA256SUMS
```

The same source/toolchain comparison was executed at 30a79ef, and twice locally
at cb7d667. Final release-tool edits after bbf5117 were tested by the 16 Python
regressions; they do not change production Go code. PR checks identify the final
review head independently of the retained bbf5117 runtime package.

## Fresh native acceptance

The [command ledger](evidence/release-preparation-20261009/commands.json) records
exact arguments, UTC start/end times and exits for builds, immutable installations,
real smoke and all four native acceptance runs. Native checks ran sequentially
in fresh distinct ignored state. No normal browser profile, system trust or
network interface was changed. Existing native trust/identity/cache/lifecycle
assertions remain intact.

| Integration/browser | Fresh result | Native and lifecycle evidence |
| --- | --- | --- |
| Caddy Chrome | **10/11 fixtures, exit 1**; only `expired-binding: native verifier accepted expired exp header` | [Chrome](evidence/release-preparation-20261009/caddy-chrome.json): native QWAC verdicts, unchanged cached trust, TLS renewal with same QWAC, replacement identity, restart/failure recovery; original TLS fingerprint held at least 45 seconds beyond expiry, all held requests on one original HTTP/2 session |
| Caddy Firefox | **11/11 fixtures, exit 0** | [Firefox](evidence/release-preparation-20261009/caddy-firefox.json): six native `secure-etsi` phases, no certificate override/insecure acceptance, unchanged trust, cached renewal/replacement, independent refresh, restart/failure recovery |
| Nginx Chrome | **24/24 integration checks, exit 0** | [Chrome](evidence/release-preparation-20261009/nginx-chrome.json): three native QWAC verdict phases, unchanged trust; TLS changes on renewal, QWAC changes on replacement; two actual Certbot renewals retain original held connection and complete coverage |
| Nginx Firefox | **24/24 integration checks, exit 0** | [Firefox](evidence/release-preparation-20261009/nginx-firefox.json): three native `secure-etsi` phases, no override/insecure acceptance, unchanged trust and identities; two renewals with held connection/coverage |

Nginx browser runs are integration checks, not the eleven-fixture Caddy suite;
the preserved Chrome fixture failure remains separately reported. No assertion,
trust override or timeout was weakened to obtain a passing result.

## Content and launch prerequisite review

The [history/content scan](evidence/release-preparation-20261009/content-audit.json)
records its exact baseline and staged-file scope, findings and the sole permitted
private-key fixture. No generated private-key PEM, recognized token-format match,
raw log/NetLog/HAR or browser-profile database filename candidate was found.
The already-public Mozilla fixture has full-file SHA256
`0560d06fc2d28c8c66b873f823542dea3abd99b212b2ced76e9d5f331a1afccd`, with pinned
source/attribution and public-key fingerprints retained. Full MPL text is added.
All ten tracked PNGs were visually inspected: synthetic test identities, no
unrelated browsing or credentials. Raw logs/generated keys/profiles stay ignored.
The scan does not inspect unreachable objects/remote-only refs or prove absence
of arbitrary unknown secret formats. Owner history review before changing
visibility remains required. No history is rewritten.

GitHub API inspection confirms private=true, default main, licence=null. The
private-vulnerability-reporting endpoint returns 404, so its availability is
**unverified**, not assumed enabled or disabled. No contact/SLA is invented.
Apache-2.0 is an unapproved recommendation, with complete draft text and Go notices
in packages. `release/publication.json` remains approved=false. The security policy
and maintainer checklist require actual owner decisions and live verification.

## Final requirement audit

| Objective requirement | Reviewable implementation/evidence | Status |
| --- | --- | --- |
| 1. Components/support | README and alpha checklist cover all four modules, exact tested matrix, RSA/RS256/explicit trust/same-authority limits, strict/Firefox modes, Chrome exp, alpha migration/rollback/dependency policy | Engineering complete; version/support approval pending owner |
| 2. Module versioning | Three correct tag namespaces/plain consumer versions; root C → real nested dependency D → tested/tagged E; immutable current pins; separate C/E binary/source ledger and rendered release identity | Complete plan and validation; tags intentionally absent |
| 3. Production packages | Deterministic three-target agent archives, exact allowlist/legal/install docs, SHA256/manifest, config-free CLI, integrity/mode/native/reproducibility tests and extracted real runtime | Complete; Linux arm64 compile-only; legal proposals remain unapproved |
| 4. Release automation | Read-only retained PR candidates; manual main-only protected publication, actual API reviewer/branch checks, exact tags/pins/source/CI/digest/rebuild/security/legal gates, refusal to overwrite, no ordinary push publishing | Complete tooling/tests; live approved publication intentionally unavailable |
| 5. Installation/operations | Current README plus reused Caddy/agent/credential/demo/operations guides; immutable current vs future installs, package ownership/startup/hooks/status/update/rollback, dedicated Nginx and untested service examples, xcaddy-only distribution | Complete; no website or hosted-builder claim |
| 6. Public prerequisites | CONTRIBUTING owner boundary, concrete Apache proposal, attribution inventory/full MPL/public fixture, SECURITY and owner checklist, bounded history/staged-content and screenshot review | Engineering review complete; licence/ownership/visibility/confidential route/protected environment pending owner |
| 7. Preserve/prove behavior | Four-module races/boundaries/independence/builds; authenticated fresh-cache immutable installs and real binary smoke; all four fresh sequential native runs; packaged/native/cross-host checks and source/version/command ledgers | Available checks complete; anonymous tagged/public checks prepared and unavailable while private |
| Completion. Review/PR | Final diff reviewed, source-identity/expiry/environment/legal-notice defects corrected, PR #7 against main; runtime APIs/lifecycle unchanged | Reviewable preparation PR; no merge/publication |

The remaining owner/public-launch steps are listed in
[the exact maintainer checklist](alpha-release.md): approved licence/copyright,
version/support/timing, confidential reporting or verified private reporting,
protected environment, visibility, real source tags/dependency updates, final
approved-source CI/native/public installation and publication. The anonymous
verifier is prepared and strips credentials/private overrides/workspaces; no
successful anonymous installation is claimed. No merge, release, release tag,
visibility change, Caddy registration or website deployment occurred in this task.
