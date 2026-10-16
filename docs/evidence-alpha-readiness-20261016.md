# Experimental public alpha readiness — started 2026-10-16

**Current verdict: preparation in progress; candidate readiness is not yet proven.**
The remaining validation and final source/candidate ledger below must be completed
before this report can give a final technical-readiness verdict. Publication is
separate and `release/publication.json` remains `approved: false`.

## Scope and provenance

Base main: `11ac519f76b1ce063b62d0a46433bb15c68ac67c`, merged PR #9.
Working branch: `codex/alpha-readiness`. The
[execution/acceptance ledger](alpha-readiness-plan-20261016.md) preserves the full
objective and engineering review. No merge, tags, visibility/security/protection
changes, website deployment, DNS/account changes or third-party messages occur.

Final implementation/core/production-module/devtools pins, selected candidate
source, manifest digest, CI identities and final documentation revision: pending.
Existing October 9/12/15 reports remain historical; their packages and lifecycle
results are not substituted for the upgraded candidate's acceptance.

## Dependency/security assessment

Official upstream checks selected Caddy **2.11.7**, CertMagic **0.25.6**,
quic-go **0.63.0** and Go **1.27.2**. The current Go release includes security
fixes and replaces 1.27.1. The coherent graph also upgrades `golang.org/x/net` to
0.60.0 for the October HTTP/2 fixes; MVS selects the required x/text 0.42.0.
govulncheck is pinned at **v1.9.0**. The history audit uses the official gitleaks
Go module **v8.30.1**, identified through Go build metadata.

Primary evidence:

- [Caddy stable release](https://github.com/caddyserver/caddy/releases/tag/v2.11.7)
  and [CertMagic release](https://github.com/caddyserver/certmagic/releases/tag/v0.25.6).
- [Escaped-path routing advisory](https://github.com/advisories/GHSA-g7pc-pc7g-h8jh):
  Caddy below 2.11.1 is affected. This is distinct from qwacamole's binding gate.
- [QPACK header expansion](https://github.com/advisories/GHSA-g754-hx8w-x2g6):
  quic-go below 0.57.0 is affected.
- [QPACK trailer expansion](https://github.com/advisories/GHSA-vvgj-x9jq-8cj9):
  quic-go through 0.59.0 is affected, fixed in 0.59.1.
- [Go release/security history](https://go.dev/doc/devel/release#go1.27.2).

The upgraded Google API/GAX graph imports `encoding/json/jsontext`; the old
`nojsonv2` workaround cannot compile it. Caddy now compares RawMessage types with
`reflect.TypeFor`, fixing the original alias lookup. Builds explicitly use
`GOEXPERIMENT=jsonv2`; measured runtime identity is exactly `go1.27.2`, and build
metadata records `GOEXPERIMENT=jsonv2`. Package verification retains exact checks.

CertMagic's new `ShouldEmitFunc` can skip the abortable `tls_get_certificate`
event when Caddy sees no subscribers. The issuer explicitly retains its handshake
and managed-cache events while preserving prior event interests and callbacks.
A regression invokes the actual CertMagic certificate-selection callback with a
rejecting startup gate. Remaining event/cache/stream/shutdown behavior requires
the real lifecycle/native runs below; compilation alone does not establish it.

Independent source scans cover core, agent, Caddy and devtools, plus separate
actual agent/Caddy command scans. No reachable findings remain in those scans.
One module-level finding remains: **GO-2026-5932**, x/crypto **v0.57.0**, covering
unmaintained OpenPGP packages with no fixed version. The affected packages are not
imported by these commands/modules; other maintained x/crypto APIs are used.
The [exact disposition](../release/vulnerability-dispositions.json) cites
[the Go advisory](https://pkg.go.dev/vuln/GO-2026-5932) and
[upstream deprecation](https://go.dev/issue/44226). It cannot suppress reachable
symbols and becomes invalid if an excluded package appears. Extracted-binary
scan results are still required before selecting the final candidate.

The enforcing scanner records actual vulnerability-database retrieval time,
metadata digest and each scan's database last-modified/version/mode. It parses
JSON findings because JSON mode can exit zero with vulnerabilities. CI covers
independent sources/commands and every extracted production target, and runs a
packaged-Caddy denial regression for both escaped-path case variants.

## Newly executed checks so far

- Go 1.27.2 macOS arm64 archive checksum verified against official download
  metadata: `76812b213b1b2302c978d28fa52fa92d541704b9e7d9d5db8002c50e4018c4c5`.
- Production Caddy race tests, including the actual handshake event-filter
  regression: passed.
- `make test`: all four workspace race suites and dependency boundaries passed.
- `make agent-build plugin-build build`: passed with Caddy 2.11.7 and jsonv2.
- `python3 -m unittest discover -s scripts -p 'test_release*.py'`: 35 passed,
  including approved mailto full-gate behavior, malformed/multiple/header mailbox
  refusal, security-result parsing and dependency-baseline rejection.
- Independent source/command security scans: no reachable findings, only the
  specific OpenPGP module-only disposition above.
- Redacted gitleaks history/current-tree scans found the exact public Mozilla
  fixture key and four uses of RFC 6455's public example WebSocket nonce. Each
  finding was checked against its source blob/line; no unknown finding remains
  in those scans. A complete PEM/token/profile/artifact audit is still pending.

Raw new logs, scanner JSON, database metadata, upstream release/advisory JSON,
redacted audit results and reviewed finding metadata are under ignored
`.local/alpha-readiness/`. Generated private keys/profiles are not committed.

## Pending candidate/runtime/delivery evidence

- Real pushed core/production/devtools pins and independently installed binaries.
- Two identical exact-source candidate builds, extracted target/version/mode/
  checksum/notices/installer checks and all four binary vulnerability scans.
- Packaged Caddy escaped-path denial regression, including an old-binary control.
- Clean supported Ubuntu quickstarts, HTTP(S)/binding verification, restrictive
  masks/preservation/conflicts, real ACME/Certbot hooks, service restart/reboot/
  runtime recreation, replacement/failures/expiry/recovery/retained coverage.
- Independent-module races, real installed-binary smokes and sequential fresh
  native Chrome/Firefox acceptance for both integrations, preserved negative
  assertions and investigation of every new failure.
- Website check/build/links/search/publication boundary and desktop/mobile review.
- Complete refreshed history/tracked/artifact audit, final legal/hash/notice
  verification, exact final candidate/source ledger, final diff review and PR.

## External launch actions

Use the single [maintainer launch checklist](alpha-release.md#remaining-owner-actions-before-public-launch).
Repository visibility is verified **private**; no module tags exist. Read-only
publication-environment and private-vulnerability-reporting APIs returned **404**,
so those settings are unverified, not claimed enabled/disabled. The current email
contact is real user-supplied policy text; inbox monitoring is not independently
established and support remains best-effort. The new gate accepts a separately
approved literal mailto channel without weakening other publication protections.

Owner approval, usable approved reporting route, verified protected environment,
visibility decision, merge, immutable C/D/E source/tag sequence, successful exact-
source manual main workflows, reviewed approved candidate, successful live
preflight without `--publish`, explicit publication and anonymous distribution
verification remain external actions. This preparation does not perform them.
