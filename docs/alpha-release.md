# Proposed 0.1.0-alpha.1 release contract and maintainer checklist

This guide prepares an alpha; it publishes nothing. The repository and website
are public, and confidential reporting is available. The owner approved
publication of `0.1.0-alpha.1` under MIT; the decision is recorded in
[PR #14](https://github.com/cztamas/qwacamole/pull/14) and `release/publication.json`.
Final source identities, tags, artifacts and live checks remain prerequisites.
No proposed tag exists merely because it is described here.

## Components and support

| Component | Go consumer version | Git tag | Distribution |
| --- | --- | --- | --- |
| Core | `github.com/cztamas/qwacamole@v0.1.0-alpha.1` | `v0.1.0-alpha.1` | stdlib-only Go module |
| Caddy | `github.com/cztamas/qwacamole/caddy@v0.1.0-alpha.1` | `caddy/v0.1.0-alpha.1` | xcaddy module plus Linux amd64 Caddy distribution |
| Agent | `github.com/cztamas/qwacamole/agent@v0.1.0-alpha.1` | `agent/v0.1.0-alpha.1` | Go module and executable archives |
| Devtools | no supported product version | no alpha tag | source-only synthetic demonstrations |

The directory prefix is in the Git tag, never in the Go requested version.
All three proposed versioned module commands become valid only after those tags exist.
Current authenticated candidates are in the canonical packaged-installation guide;
custom immutable-ref builds are in the advanced source guide.

This experimental alpha covers supplied RSA/RS256 credentials, explicit trust,
one literal hostname per state, same-authority replacement, exclusive local
storage and browser-facing TLS coverage. Caddy can use independent states/sites;
the agent owns one dedicated Nginx master and complete generated configuration.
No stable production SLA, legal qualification, automatic enrollment, revocation
service, HSM, distributed state, authority migration, shared multi-site Nginx or
arbitrary CDN termination is claimed. Strict bindings remain default; Firefox
compatibility is explicit. Preserve Chromium's expired-exp failing assertion.

The reviewed candidate uses Go 1.27.2 with `GOEXPERIMENT=jsonv2`, xcaddy 0.4.5,
Caddy 2.11.7, CertMagic 0.25.6 and Pebble 2.8.0. New native runs use Chrome
155.0.8059.82 / genuine PKIMetadata 1803 and Firefox 158.0 / geckodriver 0.37.1.
The clean Ubuntu 24.04.5 amd64 guest uses distro Nginx 1.24.0 and Certbot 2.9.0;
macOS arm64 uses Nginx 1.30.5 and Certbot 5.8.0. See the
[current readiness report](evidence-alpha-readiness-20261016.md) for exact-source
results and limitations. Earlier reports retain their own historical tool pairs.

Archives target Linux amd64, Linux arm64 and macOS arm64. macOS arm64 is runtime
tested locally, including extracted packages and both integrations. Candidate CI
runs the extracted Linux amd64 and macOS arm64 `version` command; the portable
integration workflow tests Linux amd64. Linux arm64 is **cross-compiled only**.
No Windows support. Unsigned macOS binaries have no notarization claim. Nginx and Certbot remain external dependencies; Caddy is included in its separate distribution.

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

The output contains four deterministic archives, `manifest.json` and
`SHA256SUMS`. Agent archives contain the executable, installation script/information, JSON template, project legal documents and Go notices. The Caddy archive adds the plugin-included executable, minimal Caddyfile, build inventory and notices for its actual dependency graph. Candidates carry the
selected MIT LICENSE and project/Go notices; this does not approve release
publication. Build metadata
records actual source, core module version/source, Caddy version/plugin source/dependencies, target, exact Go runtime version,
toolchain and experiment, with no build timestamp. Files are 0755 for executables/installers,
0644 for documents; tar ownership/time and gzip time are normalized. Build twice
at the same source/toolchain and compare SHA256SUMS. The verifier checks the exact
member allowlist, modes, hashes and extracted native binary identity. It refuses
unsafe links, extra content or skipped native runs when `--require-native` is used.

The read-only `Agent alpha candidate packages` workflow runs on PRs or manual
dispatch, retains `agent-candidate-SOURCE_SHA` for 30 days, and verifies extracted
Linux amd64/macOS arm64 binaries. It creates no release or tags. Review artifact
contents and record the manifest SHA256 independently. Candidate checks complement
all-module/real-integration checks; they do not replace them.

## Operator setup and publication prerequisites

Candidate manifest schema 2 requires all three agent targets and Linux amd64
Caddy. Publication still checks the exact approved source, environment, tags,
reviewed manifest, successful manual main workflows and reproducible rebuild.
Schema-1 format handling remains for tests; verify historical candidates with
their corresponding committed tooling/toolchain. Only schema 2 can pass the
current publication gate. Candidate binaries/templates are installable
without Go or a clone; the authenticated operator procedure is in
[the installation guide](guides/binary-installation.md).

Before publishing, review both component inventories/notices and manifest/core
source identities, run the documented live preflight without `--publish`, and
perform anonymous downloads/checksum/extracted installation as well as the
existing anonymous module/xcaddy checks after separately authorized publication.
No private artifact result establishes public availability. Retained GitHub
candidate artifacts expire; refresh a reviewed run rather than inventing URLs.

Ubuntu 24.04 amd64 setup is a fresh-deployment command, not an update engine.
Both generated service identities are root, with private owned state/inputs.
The Nginx deployment keeps its dedicated master, existing validators and
binding-first Certbot hook. Repeat setup never overwrites operator edits or
credentials. Manual service examples for other arrangements remain untested;
new runtime/platform evidence belongs in the dated operator report.

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

## Remaining owner actions before public launch

This is the canonical launch checklist. The [readiness report](evidence-alpha-readiness-20261016.md)
records native/runtime checks; the [17 October preparation report](evidence-alpha-prepublish-20261017.md)
records current external settings, fresh checks and remaining release gates.
The [20 October release verification](evidence-alpha-release-20261020.md) records
the actual core tag, production pin commit and newly executed release checks.
The owner's 17 October instruction, “Record the publication approval”, is
recorded as `approved: true`, with PR #14 as the decision/review URL. PR #14
was merged at `dc0a66e4e8b19de0ae44341af389994c154c1957` on 20 October.

Completed prerequisites, verified on 17 October 2026:

- The repository is public; standard MIT/NOTICE hashes match publication policy.
- Owner publication approval is recorded for version `0.1.0-alpha.1`, retaining
  those MIT/NOTICE hashes and the documented experimental support limits.
- GitHub private vulnerability reporting is enabled. The primary email in
  SECURITY.md remains `tamas.czank@gmail.com`, with best-effort support only.
- `public-alpha-release` requires the existing maintainer reviewer and allows
  only the `main` branch. The live environment passes the publication validator.
- Preparation PR #10 is merged, with all three prerequisite source identities
  retained as ancestors of main.
- The website is deployed at https://qwacamole.com. Website deployment is
  independent of module tags, package approval and the GitHub prerelease.

The core tag is now `v0.1.0-alpha.1` at
`dc0a66e4e8b19de0ae44341af389994c154c1957`, retrieved with public checksum
verification. Agent/Caddy require that tagged core. Their final tags, artifact
identities and publication status must be checked against the release record.
Never require an invented or unverified version. The remaining sequence is:

| External action | Prerequisite and command or setting | Verify success |
| --- | --- | --- |
| Verify anonymous release dependencies | The retained core dependency downloads and verifies through the public proxy/checksum service on 20 October; the 17 October failure remains historical | Repeat full anonymous tagged installation with checksums enabled; metadata resolution or authenticated CI alone is insufficient |
| Review final source and inventory | Publication approval is recorded; review the eventual final candidate, MIT/NOTICE copies, bundled notices and fresh content/history audit against that decision | Review exact C/E source ledger and manifest digest; legal hashes/copies and reproducible rebuild agree |
| Recheck reporting and publication protection | Keep GitHub private vulnerability reporting enabled, retain the primary email policy, and preserve the environment's reviewer/main-only restrictions | Both API reads and the existing live validators pass immediately before publication |
| Establish immutable module identities | With publication approval, follow the C/D/E sequence below. Never move a tag or remove retained source history | Anonymous Go origin lookups resolve core C and nested E; independent installs and full checks pass |
| Select and review the approved candidate | Run `gh workflow run go.yml --ref main` and `gh workflow run agent-candidate.yml --ref main` at E; review security/routing artifacts, exact-source runs and manifest digest | Both manual main runs succeed at E, extracted packages verify, and the independent rebuild matches |
| Run live preflight and publish once | Use the command below **without `--publish`** first, then the protected manual main publication action | All checks pass and rendered source/run/digest notes are reviewed; one prerelease appears without overwrite/retry |
| Verify public distribution | After publication, use `curl --fail --location` on the actual published asset URLs, run `release-agent.py verify --smoke --require-native`, install into new prefixes and repeat `check-public-install.py` module/xcaddy checks | Asset identities/installations and HTTPS/binding checks pass; record release URL and actual reporting route |

A mailto policy accepts one ordinary literal mailbox, without recipient lists,
encoded characters or query/header fields. Its exact URL must appear in SECURITY.md.
The gate does not verify inbox monitoring; support remains best-effort without a
response guarantee. A contact listing does not itself approve publication.

## Exact future source/tag sequence (owner executes after approval and merge)

**Preserve the preparation history.** PR #10 was merged at
`e3c9c1c9322c1bee9cbf3fa1cd5177be39c3aa33`; its commit pins and source-installation
examples require the actual `9b966…`, `e6198…` and `6b966…` identities. All three
are retained on main. Before the C/D/E sequence, verify again:

```sh
git fetch origin main
for retained_source in \
  9b96642914dc07ebf93e9c4c9e4afb3037caa5a6 \
  e6198e8552b85b7bae72429cb4ee3fc2e6fada07 \
  6b966e265f5535167da8413f2c7b7b2964a5e873
do
  git merge-base --is-ancestor "$retained_source" origin/main || exit 1
done
```

If any retained identity is missing, stop and select a durable repinning sequence
before publishing. Keep branch protections and immutable source history intact.
The source repository is now public; anonymous source installation still has the
availability prerequisite above, and release-version installation requires the
future tags below. Pre-publication preparation does not authorize tag creation.

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
6. After visibility and immutable module tags exist, run the anonymous module/xcaddy
   check from a clean environment (release assets are checked separately after publication):

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
   release credential: dispatch `Publish owner-approved alpha` from main with
   `publish=false`. The protected environment and job permissions are identical
   to publication. Require every API/source/artifact check to pass and review
   its rendered notes. Record the source/run/digest and result.
   The approved generic notes must already be committed at E; the script appends
   actual C/E SHAs, reviewed manifest digest and run ID from verified inputs.
   GitHub's automatic source archives follow root tag C; nested module installs
   and agent packages use E. The rendered source ledger makes this explicit.
   Do not edit/retag E to put its own SHA or later CI run ID into its source.
   Confirm tested runtime targets and unresolved limits. Manually dispatch
   `Publish owner-approved alpha` **from main**, supplying version, E, successful
   candidate run ID, owner-reviewed manifest SHA256 and `publish=true`.
   The input defaults false; an ordinary verification run cannot create a release.
   The script rejects missing
   approval/security/public availability, wrong tags/pins/source, dirty checkout,
   missing exact-source CI, an unapproved package or existing release. It rebuilds
   E independently and compares checksums before the single prerelease creation.
   It never creates/moves tags, overwrites assets, or retries an uncertain write.
   If a write outcome is uncertain, inspect the release before any further action.
8. Verify the published assets, SHA256SUMS, manifest/source, package versions,
   public install/runtime evidence and reporting route. Record the release URL.

For a local maintainer preflight, select a clean checkout of E and run:

```sh
python3 scripts/publish-alpha.py --source FULL_E_SHA --version 0.1.0-alpha.1 \
  --candidate-run SUCCESSFUL_MANUAL_MAIN_RUN --manifest-sha256 REVIEWED_MANIFEST_SHA256 \
  --directory /path/to/approved-candidate --go /absolute/path/to/go
```

This command deliberately omits `--publish`. It still verifies all prerequisites and
rebuilds the approved source. Public publication is available only through the
explicit manual action; ordinary pushes and PRs cannot publish.

Caddy package registration and hosted-builder testing are separate future owner
work. Packaged installation is the primary path; xcaddy remains the custom-build alternative. Website deployment uses the
separate [Cloudflare Pages setup](website-deployment.md); it does not publish
software. Devtools has no independently supported alpha tag.
