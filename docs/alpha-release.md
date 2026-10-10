# Proposed 0.1.0-alpha.1 release contract and maintainer checklist

This PR prepares an alpha; it publishes nothing. Owner approval is still needed
for the version, visibility, reporting channel and
publication timing. No proposed tag exists merely because it is described here.

## Components and support

| Component | Go consumer version | Git tag | Distribution |
| --- | --- | --- | --- |
| Core | `github.com/cztamas/qwacamole@v0.1.0-alpha.1` | `v0.1.0-alpha.1` | stdlib-only Go module |
| Caddy | `github.com/cztamas/qwacamole/caddy@v0.1.0-alpha.1` | `caddy/v0.1.0-alpha.1` | xcaddy module, no prebuilt custom Caddy |
| Agent | `github.com/cztamas/qwacamole/agent@v0.1.0-alpha.1` | `agent/v0.1.0-alpha.1` | Go module and executable archives |
| Devtools | no supported product version | no alpha tag | source-only synthetic demonstrations |

The directory prefix is in the Git tag, never in the Go requested version.
All three proposed versioned commands become valid only after those tags exist.
Current executable immutable-ref installs are in the README and integration guides.

This experimental alpha covers supplied RSA/RS256 credentials, explicit trust,
one literal hostname per state, same-authority replacement, exclusive local
storage and browser-facing TLS coverage. Caddy can use independent states/sites;
the agent owns one dedicated Nginx master and complete generated configuration.
No stable production SLA, legal qualification, automatic enrollment, revocation
service, HSM, distributed state, authority migration, shared multi-site Nginx or
arbitrary CDN termination is claimed. Strict bindings remain default; Firefox
compatibility is explicit. Preserve Chromium's expired-exp failing assertion.

Tested tool pair: Go 1.27.1 with `GOEXPERIMENT=nojsonv2`, xcaddy 0.4.5, Caddy
2.10.2, CertMagic 0.24.0, Pebble 2.8.0; Nginx 1.30.5/Certbot 5.8.0 on macOS
arm64. Fresh native runs use Chrome 154.0.8037.99 / genuine PKIMetadata 1798,
Firefox 157.0.1 / geckodriver 0.37.1. Fresh Ubuntu 24.04 amd64 integration CI
uses Nginx 1.24.0-2ubuntu7.18 and Certbot 5.8.0. See
[the dated release report](evidence-release-preparation-20261009.md)
for completed checks rather than assuming a newer toolchain/browser is compatible.

Archives target Linux amd64, Linux arm64 and macOS arm64. macOS arm64 is runtime
tested locally, including extracted packages and both integrations. Candidate CI
runs the extracted Linux amd64 and macOS arm64 `version` command; the portable
integration workflow tests Linux amd64. Linux arm64 is **cross-compiled only**.
No Windows support. Unsigned macOS binaries have no notarization claim. Nginx,
Certbot and Caddy are externally installed dependencies.

Alpha releases may make breaking changes, but each must document config/state
migration and rollback, retain synthetic failure/negative tests and carry a new
version. Never silently reuse a version or reinterpret durable state. Back up
complete state and inputs before upgrade; incompatible migrations require the
existing full-stop procedure. Dependency upgrades are separate reviewed changes:
retest all direct/indirect consumers and native demonstrations before changing
this tested matrix. Browser acceptance does not follow merely from Go tests.

## Candidate build and review (available now)

Use Python 3.12+ and the tested Go toolchain. Source must be a full existing
commit containing the packager; dirty/untracked content is excluded by git archive.

```sh
python3 -m unittest discover -s scripts -p 'test_release*.py' -v
python3 scripts/release-agent.py build --source FULL_SOURCE_SHA \
  --version 0.1.0-alpha.1 --output .local/new-candidate --go .tools/go/bin/go
python3 scripts/release-agent.py verify .local/new-candidate --smoke --require-native
```

The output contains three deterministic archives, `manifest.json` and
`SHA256SUMS`. Each archive contains only the agent, install instructions, project
legal documents and Go runtime/standard-library notices. Candidates carry the
selected MIT LICENSE and project/Go notices; this does not approve release
publication. Build metadata
records actual source, core module version, target, exact Go runtime version,
toolchain and experiment, with no build timestamp. Files are 0755 for the binary,
0644 for documents; tar ownership/time and gzip time are normalized. Build twice
at the same source/toolchain and compare SHA256SUMS. The verifier checks the exact
member allowlist, modes, hashes and extracted native binary identity. It refuses
unsafe links, extra content or skipped native runs when `--require-native` is used.

The read-only `Agent alpha candidate packages` workflow runs on PRs or manual
dispatch, retains `agent-candidate-SOURCE_SHA` for 30 days, and verifies extracted
Linux amd64/macOS arm64 binaries. It creates no release or tags. Review artifact
contents and record the manifest SHA256 independently. Candidate checks complement
all-module/real-integration checks; they do not replace them.

## Engineering gates before PR completion

Use [the preparation ledger](release-preparation-plan-20261009.md) and
[the dated report](evidence-release-preparation-20261009.md), distinguishing new
commands from inherited reports:

- Four-module races, dependency boundaries, GOWORK=off independent resolution
  with real pins, production agent and plugin-only builds.
- Fresh outside-checkout authenticated immutable agent and Caddy installations,
  no replacements, fresh module caches, build/module inventory, real smoke using
  those installed binaries. Private installation is not anonymous evidence.
- Sequential fresh Caddy Chrome/Firefox and Nginx Chrome/Firefox native runs,
  trust/identity/cache/lifecycle/held-connection assertions preserved. Chrome's
  only known expired-exp fixture failure stays nonzero, separately recorded.
- Deterministic archives, extracted native binary checks, archive modes/content/
  checksums/version identity and meaningful negative publication/package tests.
- Tracked content/history review, updated operator guides and final diff review.

## Owner prerequisites before public launch

- [ ] Approve version/support scope and publication timing.
- [x] Select MIT and install matching LICENSE/NOTICE files at root and in nested
  modules. See [licensing inventory](guides/licensing.md).
- [ ] Review ownership/third-party notices for the final release and verify the
  legal-file copies and recorded hashes still match.
- [ ] Review [tracked-content/history audit](evidence-release-preparation-20261009.md).
  Resolve concrete findings before changing visibility; no history rewrite is
  performed by this task.
- [ ] Enable and verify GitHub private vulnerability reporting, or publish a real
  monitored confidential channel in SECURITY.md. Approve a realistic policy;
  no address/SLA is invented.
- [ ] Record the owner decision URL, approved version and exact LICENSE/NOTICE
  SHA256s in `release/publication.json`; `approved` stays false until that review.
  For an alternative channel, set its real HTTPS URL and include it in SECURITY.md.
- [ ] Configure `public-alpha-release` as a protected GitHub environment with
  maintainer reviewers and main-only deployment branches; verify those settings.
  Merely naming the environment in YAML does not establish protection.
- [ ] Decide and perform repository visibility change, then verify public source
  access. This preparation PR does not change visibility.

## Exact future source/tag sequence (owner executes after approval and merge)

The root must become a real retrievable release before nested requirements name
it. Record all full SHAs in the final release notes and manifest review record.
The two distributed production source identities are **C** (core) and **E**
(agent/Caddy); **D** is an intermediate pushed commit needed for real test pins.

1. On main, finish the approved legal/security files and support contract; commit
   and push **C**. Run its required checks. Review root contents and then create
   and push root tag `v0.1.0-alpha.1` at C. Confirm remotely:
   `GOWORK=off go list -m -json github.com/cztamas/qwacamole@v0.1.0-alpha.1`.
   Retain its full origin SHA. Do not tag if a version/ref already exists.
2. Only after that retrieval succeeds, update `agent/go.mod` and `caddy/go.mod`
   to the actual root `v0.1.0-alpha.1`, tidy GOWORK=off, run workspace and
   independent checks, commit/push **D**. Resolve the agent/Caddy pseudo-versions
   at D remotely; do not guess timestamps or SHA suffixes.
3. Pin devtools to those real D versions, tidy, finalize the generic approved
   release notes (remove draft/private-candidate wording) and update the source ledger;
   then commit/push **E**. Verify `git diff D E -- agent caddy` is empty, so independent
   tests consume identical production modules. Run all required checks/native
   acceptance at the selected sources; prepare candidate artifacts at E. Core
   remains C; the package binary stamps E, with core version `v0.1.0-alpha.1`.
4. Tag agent and Caddy **at E** using `agent/v0.1.0-alpha.1` and
   `caddy/v0.1.0-alpha.1`. Confirm both Go consumers request the plain
   `v0.1.0-alpha.1` and their resolved origin SHA is E. Tags are immutable; never
   delete/move them to repair a mistake. Select a new alpha version instead.
5. At E on main, manually dispatch both portable Go integration and agent
   candidate workflows. Require both successful at exact source E. Candidate CI
   automatically uses approved LICENSE/NOTICE only when policy approves this
   version and their hashes match. Review retained archives and manifest digest.
6. Run the anonymous post-publication check from a clean environment:

   ```sh
   python3 scripts/check-public-install.py v0.1.0-alpha.1 \
     --go /absolute/path/to/go --output /new/anonymous-install \
     --harness /absolute/path/to/bin/qwacamole
   ```

   It replaces the process environment, disables Git config/interactive auth,
   drops tokens/SSH agent/private-module overrides, uses fresh HOME/caches and
   only the public proxy/checksum service, GOWORK=off and no replacements. It
   verifies core/agent/Caddy versions and runs smoke against those binaries when
   a harness is supplied. An existing devtools harness can use its documented
   external runtime tools, but is not part of the anonymous installers. Private
   repository authentication never satisfies this public gate. Repeat all native
   acceptance against final production sources as recorded in the release ledger.
7. Run the live publication preflight **without `--publish`**, using the intended
   release credential, and require every API/source/artifact check to pass before
   reviewing its rendered notes. Record the source/run/digest and result.
   The approved generic notes must already be committed at E; the script appends
   actual C/E SHAs, reviewed manifest digest and run ID from verified inputs.
   GitHub's automatic source archives follow root tag C; nested module installs
   and agent packages use E. The rendered source ledger makes this explicit.
   Do not edit/retag E to put its own SHA or later CI run ID into its source.
   Confirm tested runtime targets and unresolved limits. Manually dispatch
   `Publish owner-approved alpha` **from main**, supplying version, E, successful
   candidate run ID and owner-reviewed manifest SHA256. The script rejects missing
   approval/security/public availability, wrong tags/pins/source, dirty checkout,
   missing exact-source CI, an unapproved package or existing release. It rebuilds
   E independently and compares checksums before the single prerelease creation.
   It never creates/moves tags, overwrites assets, or retries an uncertain write.
   If a write outcome is uncertain, inspect the release before any further action.
8. Verify the published assets, SHA256SUMS, manifest/source, package versions,
   public install/runtime evidence and reporting route. Record the release URL.

For a local maintainer preflight, use `scripts/publish-alpha.py` with the same
arguments and **without `--publish`**. It still verifies all prerequisites and
rebuilds the approved source. Public publication is available only through the
explicit manual action; ordinary pushes and PRs cannot publish.

Caddy package registration and hosted-builder testing are separate future owner
work. The supported installation path here is xcaddy. Website deployment uses the
separate [Cloudflare Pages setup](website-deployment.md); it does not publish
software. Devtools has no independently supported alpha tag.
