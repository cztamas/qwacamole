# Experimental public alpha readiness — 2026-10-16

**Verdict: the reviewed candidate is technically ready for an experimental public
alpha within the documented platform and browser limits. Publication remains
separate and is not approved or performed.** The preserved Chromium expired-binding
negative assertion still fails: Chrome displays qualified identity after binding
`exp`. Server-side expiry enforcement passes. This is a known browser limitation,
not a passing expiry guarantee or a regression hidden by a weakened test.

## Scope and exact provenance

Base main is `11ac519f76b1ce063b62d0a46433bb15c68ac67c`, including merged PR #9.
Branch: `codex/alpha-readiness`. This report records newly executed acceptance;
October 9/12/15 reports remain historical. The
[plan and acceptance ledger](alpha-readiness-plan-20261016.md) records the assessment.

| Layer | Actual implemented source / resolved identity |
| --- | --- |
| Core security/toolchain/legal preparation | `9b96642914dc07ebf93e9c4c9e4afb3037caa5a6` / `v0.0.0-20261016203648-9b96642914dc` |
| Agent and Caddy production module pins | `e6198e8552b85b7bae72429cb4ee3fc2e6fada07` / `v0.0.0-20261016204019-e6198e8552b8` |
| Frozen candidate and independently installed source | **`6b966e265f5535167da8413f2c7b7b2964a5e873`** / `v0.0.0-20261016204844-6b966e265f55` |
| Final reporting commit | The later commit containing this report changes operator documentation, sanitized evidence and website layout coverage; it does not change the frozen candidate or production sources |

Core Go source is unchanged from its prior b3fb517 pin. The e6198→6b966 diff
contains no agent/Caddy production-module changes; 6b966 resolves independent
devtools pins and finishes candidate tooling. No tags were moved or created.
Distributed modules use real pushed versions without workspace replacements.

## Reviewed dependency and security baseline

Official release/advisory checks on 16 October selected Caddy **2.11.7**,
CertMagic **0.25.6**, quic-go **0.63.0**, Go **1.27.2**, x/net **0.60.0** and
MVS-selected x/text **0.42.0**. Development tools are xcaddy **0.4.5**, Pebble
**2.8.0** and govulncheck **1.9.0**, rebuilt with Go 1.27.2. Gitleaks' official
Go module is **v8.30.1**, verified through binary build metadata.

- [Caddy 2.11.7](https://github.com/caddyserver/caddy/releases/tag/v2.11.7)
  and [CertMagic 0.25.6](https://github.com/caddyserver/certmagic/releases/tag/v0.25.6)
  were the current stable releases checked. The 2.11 major-release notes were
  read at 2.11.1, the first published stable 2.11 release. The HTTPS-upstream Host
  rewrite is documented for custom configurations; quickstarts use HTTP upstreams.
  2.11.7 fixes 2.11.6 HTTP/2 deadline crashes/idle-stream resets. CertMagic's
  ZeroSSL authentication change does not replace the tested gated ACME path.
- [GHSA-g7pc-pc7g-h8jh](https://github.com/advisories/GHSA-g7pc-pc7g-h8jh)
  affects Caddy below 2.11.1. Against the actual old Caddy 2.10.2 distribution,
  `/admin%2Fpanel` returned 403 and `/ADMIN%2Fpanel` returned 200. Against the
  new packaged binary both return 403 with the configured `DENY` body; an ordinary
  control route remains accessible. **No qwacamole gate is configured in this
  test.** This establishes the upstream routing fix, separately from binding gates.
- [GHSA-g754-hx8w-x2g6](https://github.com/advisories/GHSA-g754-hx8w-x2g6)
  affects quic-go below 0.57.0; [GHSA-vvgj-x9jq-8cj9](https://github.com/advisories/GHSA-vvgj-x9jq-8cj9)
  affects versions through 0.59.0. The selected 0.63.0 is beyond both fixes.
- [Go 1.27.2](https://go.dev/doc/devel/release#go1.27.2) replaces the vulnerable
  1.27.1 toolchain. The official macOS arm64 archive checksum was verified:
  `76812b213b1b2302c978d28fa52fa92d541704b9e7d9d5db8002c50e4018c4c5`.

The new Google API/GAX graph imports `encoding/json/jsontext`, so the old
`nojsonv2` workaround no longer compiles. Caddy now uses `reflect.TypeFor` for
RawMessage, fixing the original alias issue. All build/tool/CI paths explicitly
use **`GOEXPERIMENT=jsonv2`**. Measured runtime identity is exactly **`go1.27.2`**;
binary build metadata also records jsonv2 and `CGO_ENABLED=0`. Exact package
checks were updated, not relaxed. Config/JSON/signature/runtime tests pass.

CertMagic's new `ShouldEmitFunc` could omit the abortable `tls_get_certificate`
event when Caddy sees no subscribers. The issuer retains that event and
`cached_managed_cert`, preserving previous event interests and callbacks. A focused
regression calls CertMagic's actual `GetCertificateWithContext` with a rejecting
startup gate and a prior filter returning false. Real issuance, managed-cache
loading, replacement, restart, streaming, shutdown and retained-connection tests
then exercise the integration. The conservative full-stop TLS retirement policy
remains; no store, journal or deployment lifecycle was rebuilt.

### Enforcing vulnerability scans

[Security evidence](evidence/alpha-readiness-20261016/security-summary.json) covers
all four modules independently, both actual production command packages and all
four extracted production binaries. All ten scans passed the enforcing policy.
Database metadata was retrieved at **2026-10-16T20:56:25.331805+00:00**, SHA-256
`e23e6df49ea2c2390b01327a1ca29e5e766319f401a2ca733ab0d5f4ab221dda`;
each scan reports database last modification **2026-10-08T22:31:09Z**.

No reachable production or development finding remains. One module-level advisory
remains: **GO-2026-5932**, x/crypto **v0.57.0**, unmaintained OpenPGP APIs with no
fixed version. No module/command imports the affected OpenPGP packages; binary
scans contain no affected package/symbol. Other maintained x/crypto APIs are used.
The [exact version-specific disposition](../release/vulnerability-dispositions.json)
cites [the advisory](https://pkg.go.dev/vuln/GO-2026-5932) and
[upstream deprecation](https://go.dev/issue/44226). It cannot suppress reachable
symbols and fails if an excluded package appears.

`scripts/check-vulnerabilities.py` parses JSON findings because govulncheck's JSON
mode can exit zero with findings. Missing scanner/database/SBOM metadata, reachable
symbols and undispositioned module findings fail. `go.yml` enforces independent
source/command scans; `agent-candidate.yml` also scans every packaged binary and
runs the actual packaged-Caddy routing regression. The scanner is pinned through
bootstrap. Regression tests exercise these failure paths.

## Actual candidate and hosted proof

Version **0.1.0-alpha.1**, `candidate: true`, source **6b966e265f5535167da8413f2c7b7b2964a5e873**.
Manifest SHA-256:
**`923eb8ad0b56959284f4ab1fba9debaea5f7e2850d812daa881ad3b4f18d388d`**.

Two separate builds from `git archive` produced identical complete SHA256SUMS.
The downloaded hosted candidate is byte-identical to both local builds.
[Manifest](evidence/alpha-readiness-20261016/candidate-manifest.json),
[archive checksums](evidence/alpha-readiness-20261016/SHA256SUMS) and
[candidate identity](evidence/alpha-readiness-20261016/candidate-identity.json)
are retained without embedding the executables in Git.

| Manual feature-branch workflow at selected source | Result |
| --- | --- |
| [Candidate 38965179833](https://github.com/cztamas/qwacamole/actions/runs/38965179833) | Success: package checks, source/binary security, routing, extracted native versions |
| [Portable integration 38965536609](https://github.com/cztamas/qwacamole/actions/runs/38965536609) | Success: all modules, boundaries, security, Ubuntu permissions, real Caddy/Nginx integrations |

The retained private candidate artifact is **12074583060**, named
`agent-candidate-6b966e265f5535167da8413f2c7b7b2964a5e873`, expiring
**2026-11-15T20:55:33Z**. It is not a public release. The authenticated canonical
installation guide uses these concrete identities and refuses identity/checksum
mismatches. Expired artifacts require a newly reviewed identity, not an invented
replacement. These successful **feature-branch** workflows do not satisfy the
publication requirement for manual exact-source **main** runs.

Strict archive allowlists, executable modes, platform/source/compiler identities,
module inventories, notices and installer refusal checks pass. All three agent
targets were extracted and scanned; Linux arm64 is **cross-compiled only**.
Actual installed binaries were exercised on macOS arm64 and Ubuntu amd64.
Installers reject existing prefixes; Linux agent wrong-platform/corruption
refusals and Caddy preservation of the existing binary were checked.

The [legal/inventory verification](evidence/alpha-readiness-20261016/candidate-legal-inventory.json)
compares LICENSE/NOTICE in all module copies and all archives with the selected
source and policy hashes. Every linked Caddy module has bundled attribution;
the manifest matches actual `go version -m`. No production binary includes
replacements/devtools, and agents do not include Caddy. Go licence/PATENTS notices
are bundled. The standard MIT licence remains selected.

## Newly executed runtime and repository acceptance

Commands ran with the pinned tools and explicit jsonv2 configuration. Raw output
is under ignored `.local/alpha-readiness/`; only sanitized summaries are tracked.

| Check | New result |
| --- | --- |
| `make test` | Four workspace race suites and dependency boundaries pass, including real proxy HTTP/1.1+HTTP/2 rejection/cancellation regressions |
| `scripts/check-independent-modules.sh` | Four independent race suites pass with actual resolved production pins |
| `make agent-build plugin-build build` | Pass with upgraded graph |
| `scripts/check-consumer.sh` | Outside-workspace production-only consumer registration/build passes |
| `python3 -m unittest discover -s scripts -p 'test_release*.py'` | **35 pass**, including full mocked approved publication, mismatch/existing-release/API failure cases, mailto validation and vulnerability enforcement |
| `scripts/check-remote-agent.sh 6b966e265f5535167da8413f2c7b7b2964a5e873` | Fresh immutable remote install passes, no workspace/replacements/devtools |
| `scripts/check-remote-install.sh 6b966e265f5535167da8413f2c7b7b2964a5e873` | Independent Caddy install passes with exact upstream/runtime identities |
| Remote-installed `plugin-smoke` then `nginx-smoke` | Both pass actual ACME, publication/activation, retained TLS, expiry/recovery, replacement and restart |
| Extracted Linux candidate routing/security | Both escaped-path variants deny; all binary scans pass |

### Clean supported Ubuntu, actual installed packages

A fresh official **Ubuntu 24.04.5 LTS amd64** cloud image ran in a task-owned
QEMU VM with real systemd/reboots, distro **Nginx 1.24.0** and **Certbot 2.9.0**.
The image's official checksum was verified:
`6a81c37564db9b1ee84e141922625e1d7c5b389b99bb3c572e0243607d5bb4d2`.
The guest has no Go installation or repository clone. The exact candidate agent
and Caddy were extracted, checksum-verified and installed into new `/opt` prefixes.
A separately cross-built development harness/Pebble supplies test-only ACME and
synthetic credentials; neither ships in the production packages. Only loopback
SSH was exposed on the host. No public DNS/account or production trust changed.

- Both package setup commands passed fresh/repeat setup under **077 and 0777**
  in disposable Ubuntu acceptance containers. Real generated Nginx `www-data`
  workers returned HTTP 200 for an HTTP-01 challenge. Exact file modes and
  unchanged files/state/ownership/mtimes on repeat were verified; edits and
  changed modes were preserved/refused. [Four results](evidence/alpha-readiness-20261016/package-umasks/agent-077.json)
  are retained in the adjacent directory.
- In the full guest, the Nginx quickstart ran under 077 and Caddy under 0777.
  Dry-run, actual setup, real HTTPS, human/JSON signed-binding checks and active
  repeat setup passed. Agent rejection/preservation tests covered invalid chain,
  key/hostname/authority, changed valid inputs, operator edits/modes/ownership,
  missing Certbot/Nginx capabilities, occupied ports and an unrelated real Nginx
  master, which remained alive. Caddy rejected bad inputs, missing validation
  prerequisites and occupied ports without stopping the running integration.
- Real Certbot HTTP-01 issuance used the installed deploy hook. Forced renewal
  through the saved configuration changed the actual TLS fingerprint and produced
  a current covering binding. Certbot's real scheduled 383.754-second random
  delay was allowed to complete. Caddy's gated ACME issuance/renewal also passed.
- Both services passed restart and **full host reboot** with changed boot IDs,
  removed volatile markers, automatic enabled service startup and verified HTTPS/
  current public binding. The agent recreated private `/run/qwacamole` and
  `www-data` body-temp ownership/modes. Caddy preserved private persisted state.
- Both integrations accepted separately validated **same-key and new-key**
  replacements under the **unchanged explicit authority**. Public JWS signer DER
  matched each activated replacement. Cold backups and the documented controlled
  restart/reload sequence were used; importing did not expand trust.
- Full installed-package lifecycle smokes run as an unprivileged guest user cover
  publication/reload rejection, actual expiry/recovery, retained TLS coverage,
  streaming/cache/WebSocket behavior, replacement and restart with the packaged
  executables, real distro Nginx/Certbot and Pebble. **Both passed.**
  [Quickstart/reboot/replacement evidence](evidence/alpha-readiness-20261016/ubuntu-quickstarts.json)
  and [installed identities/lifecycle assertions](evidence/alpha-readiness-20261016/ubuntu-guest-summary.json)
  retain the results.

Two early controller probes were premature (before the agent renewal/reboot and
immediately after Caddy's Type=simple start). Their output is retained; readiness
was subsequently awaited and the same services verified. Pebble regenerates its
ordinary TLS issuing CA on reboot. Caddy correctly renewed into that new test CA;
the first post-boot ordinary trust check therefore failed. Explicitly installing
that new **test** root in the disposable guest restored fully verified HTTPS.
Selected QWAC authority, credentials and TLS assertions were unchanged. This is a
fixture prerequisite, not an automatic production trust expansion.

### Sequential native browser acceptance

All four fresh runs were executed sequentially on macOS arm64 with the upgraded
source. Tools: Chrome **155.0.8059.82**, genuine PKIMetadata **1803**, stock signed
Firefox **158.0**, geckodriver **0.37.1**, Nginx **1.30.5**, Certbot **5.8.0**.

| Run | Result |
| --- | --- |
| `integration-test` (Caddy / Chrome) | Ten fixtures pass; the unchanged expired-exp fixture fails as the known browser incompatibility. All required lifecycle checks pass. |
| `firefox-integration` (Caddy / Firefox) | All eleven fixtures and full native lifecycle pass |
| `nginx-browser-integration -native-browser chrome` | Pass with actual installed macOS package agent |
| `nginx-browser-integration -native-browser firefox` | Pass with the same installed package agent |

Trust/chain/profile/hostname/signature/TLS-hash negatives were preserved. Positive
identity, cached discovery, real renewal, same/new-key replacement, restart,
sign/publication failure and recovery, held HTTP/2 through TLS expiry and full
binding lifetime passed. See [browser versions](evidence/alpha-readiness-20261016/native-versions.json),
[Caddy Chrome](evidence/alpha-readiness-20261016/native-caddy-chrome-summary.json),
[Caddy Firefox](evidence/alpha-readiness-20261016/native-caddy-firefox-summary.json)
and [installed/native checks](evidence/alpha-readiness-20261016/installed-and-native-smokes.json).

The installed Firefox 157 executable still launches a pending updater. The already
staged **stock signed 158.0** executable passed `codesign --verify --deep --strict`
and was used directly without installing an update or changing browser preferences.
No trust or assertions were weakened for startup. Chrome's expired-exp behavior
persists with the current genuine metadata; the public support matrix explicitly
states it cannot provide browser-side binding-expiry enforcement. Server gates
still reject expired protected responses/handshakes as designed.

### Canonical documentation and website

README now leads to packages and the two **install → configure → start → verify**
quickstarts, linking the advanced source guide. There is one source per guide;
the old source blocks and no-prebuilt-Caddy claim are removed. Legal inventory,
NOTICE copies, support matrix, release notes and maintainer checklist agree with
the selected baseline and actual availability. Nginx's direct IPv4 A record,
IPv4-reachable ports and **no AAAA record** remain explicit. Strict defaults,
explicit Firefox compatibility, runtime versus cross-compiled targets, local
trust versus browser/legal qualification, and revocation limits remain distinct.

Using Node **24.21.0**, website check passed Astro diagnostics and 14 Node tests.
Build passed **16 pages, 703 internal links/anchors/assets, 14 indexed product
pages and 29 complete search assets**, plus the exact publication boundary.
**16 Playwright checks passed with zero retries**, including revised binary/source
installation pages at **1440 and 390 pixels**, accessibility, no page overflow,
keyboard/menu/search/code scrolling and dark contrast. All four revised page
screenshots were visually inspected: readable hierarchy, contained horizontal
code scrolling and usable mobile navigation. [Summary](evidence/alpha-readiness-20261016/website-summary.json).
No maintainer reports/decisions are published to product routes/search. No website
deployment occurred.

## Refreshed content/history/artifact audit

The available advertised remote heads and PR heads were fetched into local audit
refs. All **12 advertised refs**, including the new branch and PRs 1–9, are included
in reachable history; [scope](evidence/alpha-readiness-20261016/remote-audit-scope.json)
records their exact commits. This ref snapshot precedes the final reporting
commit; that commit's proposed content is audited separately below. Main remained
11ac519 and no module tags exist.
The final proposed tracked content is included alongside history, not just HEAD.

Gitleaks **v8.30.1** scanned all reachable history (`--all --full-history`) with
inline ignores disabled, plus a tracked/proposed-file snapshot. Findings were
redacted and individually checked against source blob/line identities. Only the
exact intentionally public Mozilla fixture key and four uses of RFC 6455's
public sample WebSocket nonce remain; none is an unknown credential. Sanitized
[history](evidence/alpha-readiness-20261016/history-gitleaks-dispositions.json) and
[tree](evidence/alpha-readiness-20261016/tree-gitleaks-dispositions.json) dispositions
contain hashes and attribution, not secret values.

The bounded complementary audit scans complete private-key PEM, recognized
GitHub/OpenAI token formats and raw/profile filenames over reachable file blobs
and the final staged tracked content. Its only key exception is the exact Mozilla
public fixture SHA-256
**`0560d06fc2d28c8c66b873f823542dea3abd99b212b2ced76e9d5f331a1afccd`**
at the two historical/current fixture paths, with Mozilla/MPL attribution intact.
No generated key, browser profile or raw sensitive log is tracked. Exact counts
and limitations are in [the final audit](evidence/alpha-readiness-20261016/content-audit-history.json).

All **30 decompressed members**, including executable bytes, of the four actual
candidate archives were scanned with **no private-key/token/profile exception**;
none was found. [Artifact audit](evidence/alpha-readiness-20261016/content-audit-candidate.json).
Archive allowlists and actual production dependency checks provide additional
coverage. Hidden/inaccessible remote refs, reflogs, unreachable Git objects,
external forks and unknown secret encodings are outside this bounded audit; it
is not a universal proof of absence. Generated raw evidence remains ignored.

## Publication gate and remaining launch actions

`release/publication.json` remains **`approved: false`**, with no manufactured
owner decision/reporting URL. The standard MIT licence and all recorded legal
hashes match. `SECURITY.md` retains the user-supplied primary email
**tamas.czank@gmail.com**, with **best-effort support and no response guarantee**.
Inbox monitoring is not independently established.

The gate now supports an explicitly approved **single literal mailto mailbox**
whose exact URL is in SECURITY.md. Lists, query/header fields, fragments, escaping,
invalid labels/local syntax and substring matches fail. HTTPS and verified GitHub
reporting remain available. Full mocked approved gate tests preserve exact source,
legal hashes, immutable module origins/tags, reviewed manifest, exact-source manual
main workflows, reproducible rebuild, existing-release refusal and uncertain-write
protections. No additional administration token requirement is claimed.

Actual read-only repository inspection confirmed **PRIVATE** and no tags.
Environment/deployment-policy and private-reporting reads returned **404**; these
settings are **unverified**, not claimed enabled or disabled. A real preflight
was run **without `--publish`**, using the selected source, actual hosted candidate
run and reviewed digest. It correctly refused the unmerged source before further
launch checks. [Exact invocation/result](evidence/alpha-readiness-20261016/live-preflight-result.json).
This is negative guard evidence, **not a successful approved/main preflight**.

The single [maintainer launch checklist](alpha-release.md#remaining-owner-actions-before-public-launch)
identifies each prerequisite, action and success proof: owner scope/source/legal
approval; usable approved reporting route; verified protected publication
environment; visibility decision; reviewed merge and immutable C/D/E tag sequence;
successful exact-source manual main integration/candidate runs; reviewed candidate
and independent rebuild; successful live preflight; separately authorized one-time
publication; anonymous release-asset and Go/xcaddy installation verification.

Those are explicit external launch actions, not missing local acceptance. No
merge, release/tag publication, visibility/security/protection change, website
deploy, DNS/account change or third-party message was performed. The experimental
alpha does not promise production support, commercial issuer enrollment, trusted-
list qualification from a supplied root, browser revocation/expiry enforcement,
HA/distributed state, or runtime support for untested cross-compiled targets.
