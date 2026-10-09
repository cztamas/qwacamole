# Alpha release preparation — 9 October 2026 (in progress)

Baseline is merged main `08872250a0813f232faaca9794a9e5b481b4d19b`.
Release branch: `codex/public-alpha-release`. This report records newly executed
preparation checks, separately from the dated lifecycle/streaming reports. The
remaining acceptance runs below must finish before PR completion.

## Source and package identity

Agent CLI/version implementation: `bb911a8f69a8743faab70967232ff166c828c4b7`,
real module version `v0.0.0-20261009145422-bb911a8f69a8`. Corrected packager/manual
publication tooling and devtools real agent pin: `cb7d66730030600edbfc0cde5fafee106e3f8df9`.
Core remains `8e62f4b8c340e572584f6d71355a8f89a7d0be42`, version
`v0.0.0-20261008203612-8e62f4b8c340`; Caddy remains
`9322bd566e825a2c2c645e464a630e09d7aa68d6`, version
`v0.0.0-20261009072701-9322bd566e82`. No future module tag is added to Go manifests.

Candidate archives stamp **cb7d667**, which includes the unchanged agent CLI from
bb911a8 and the corrected release tooling. Remote Go installation identifies the
actual agent bb911a8 module. These identities are deliberately distinguished.
The [candidate manifest](evidence/release-preparation-20261009/candidate-manifest.json)
and [checksums](evidence/release-preparation-20261009/candidate-SHA256SUMS.txt)
record all three target archive/member hashes, modes, source/core and toolchain.
These are **private review candidates**, not released assets or a licence grant.

Fresh versions: Go 1.27.1 (`GOEXPERIMENT=nojsonv2`, binary runtime reports
`go1.27.1-X:nojsonv2`), xcaddy 0.4.5, Caddy 2.10.2, CertMagic 0.24.0;
Nginx 1.30.5, Certbot 5.8.0; Chrome 154.0.8037.99 with genuine PKIMetadata 1798;
Firefox 157.0.1/geckodriver 0.37.1. Local platform is macOS arm64.

## Newly completed engineering checks

- Agent config-free `version` tests pass under race detection. Meaningful package
  tests reject corruption, extra/secret filenames, unsafe archive paths, symlinks,
  invalid versions/sources, unapproved licensing and skipped native runs. Tag-map
  tests reject wrong namespace/source; anonymous environment tests drop inherited
  tokens, Git config, SSH agents, private overrides and workspaces. Thirteen Python
  tests currently pass.
- `make build agent-build plugin-build`, `make test`, and
  `scripts/check-independent-modules.sh` pass. All four module races and dependency
  boundaries pass; some unchanged Go packages use valid cached test results.
  Independent devtools/coretest races run for 141 seconds against real new agent
  pins. Raw logs: `.local/evidence/release-preparation-20261009/{builds,all-module-races,independent-module-races}.log`.
- Fresh outside-checkout authenticated immutable agent/Caddy installs pass with
  fresh module caches, GOWORK=off and no replacements. The production agent has
  no Caddy/browser/devtools dependency; Caddy has no devtools. Exact resolved
  origin/full SHA/build metadata: [agent](evidence/release-preparation-20261009/remote-agent-install.json),
  [Caddy](evidence/release-preparation-20261009/remote-caddy-install.json).
- Real smoke uses those remotely installed binaries: Caddy **26/26**, agent
  **34/34**. Evidence: [Caddy runtime](evidence/release-preparation-20261009/caddy-runtime.json),
  [agent runtime](evidence/release-preparation-20261009/agent-runtime.json).
- Two package builds at exact cb7d667/toolchain produce byte-identical SHA256SUMS.
  Both run the extracted macOS arm64 binary and compare all JSON identity fields
  to the manifest. Archive contents/modes/hashes pass. Linux amd64/arm64 are
  cross-compiled locally, with no local Linux runtime claim. Commands:
  `python3 scripts/release-agent.py build --source cb7d66730030600edbfc0cde5fafee106e3f8df9 --version 0.1.0-alpha.1 --output .local/release-candidate-cb7d667 --go .tools/go/bin/go`
  and the same with `--output .local/release-candidate-cb7d667-repeat`; compare
  `diff -u .../SHA256SUMS ...-repeat/SHA256SUMS`. Raw package logs are alongside
  other new evidence. Initial bb911a8 package verification correctly failed on
  runtime/toolchain version mismatch; cb7d667 records both fields explicitly.
- Fresh Caddy Chrome native lifecycle, unchanged-trust cached identity, refresh,
  replacement, restart and failure recovery all pass. Held TLS fingerprint stays
  identical for at least 45 seconds beyond expiry, with all held requests on one
  original HTTP/2 session. Fixtures **10/11, exit 1**, only preserved
  `expired-binding: native verifier accepted expired exp header`. The
  [sanitized Chrome evidence](evidence/release-preparation-20261009/caddy-chrome.json)
  includes actual trust/identity/session proof; no assertion was weakened.

The [command ledger](evidence/release-preparation-20261009/commands.json) records
sequential runtime/native commands, exact arguments, times and exits so far.
It will be finalized when remaining native runs complete. Go/package commands
are recorded above separately. Raw logs/keys/profiles stay ignored under `.local`.

## Content and launch prerequisite review

The [history/content scan](evidence/release-preparation-20261009/content-audit.json)
examined **505** reachable file blobs across local `--all` refs at cb7d667.
It found no generated private-key PEM, recognized token-format matches, raw log/
NetLog/HAR or browser-profile database filename candidates. Its sole private-key
exception is the already-public Mozilla fixture, full-file SHA256
`0560d06fc2d28c8c66b873f823542dea3abd99b212b2ced76e9d5f331a1afccd`, with pinned
source/attribution and public-key fingerprints retained. Generated keys/profiles
remain ignored. Historical public JSON is sanitized synthetic evidence and the
tracked screenshots illustrate synthetic test identities; raw unrelated browsing
history is not committed. The scan is bounded: it does not inspect unreachable
objects/remote-only refs or prove absence of arbitrary unknown secret formats.
Owner review before changing visibility remains required; no history is rewritten.

At 15:13 UTC the GitHub API confirms private=true, default main, licence=null.
The private-vulnerability-reporting endpoint returns 404, so its availability is
**unverified**, not assumed enabled/disabled. No contact/SLA is invented. Apache-2.0
is an unapproved recommendation, with complete draft text and Go notices in
packages; full Mozilla MPL text is added beside its existing attribution.
`release/publication.json` remains approved=false. The security policy and
maintainer checklist specify real owner decisions and live verification.

## Still required before completion

- Finish fresh Caddy Firefox, Nginx Chrome and Nginx Firefox runs sequentially;
  inspect native trust/UI/identity/cache/lifecycle results and final exits.
- Run/inspect candidate CI's retained archives and Linux/macOS extracted-binary
  checks, alongside hosted portable integration checks at the final code source.
- Complete publication-gate negative checks, documentation/link review, final
  source-bound candidate verification and final requirement audit; create PR.

Anonymous public installation, approved release-source tags, legal/visibility/
reporting/environment approvals and actual publication remain owner/public-launch
steps. The anonymous verifier is prepared; no successful anonymous installation
is claimed. No merge, release, release tag, visibility change, Caddy registration
or website deployment has occurred.
