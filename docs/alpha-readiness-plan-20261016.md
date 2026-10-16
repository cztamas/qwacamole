# Experimental public alpha readiness plan — 2026-10-16

Base: main `11ac519f76b1ce063b62d0a46433bb15c68ac67c`, including merged PR #9.
Branch: `codex/alpha-readiness`. This is preparation and review only; publication
approval stays false. No merge, tags, visibility/settings changes, website deploy,
external DNS/accounts or third-party messages are authorized.

## Assessment and engineering review

The existing setup/readiness commands, strict default binding profile, explicit
authority selection, binding-first activation and durable lifecycle remain the
implementation owners. Keep core stdlib-only; installed binaries must exclude
devtools. Reuse current package verifier, publication gate, Docker permission
acceptance, real integration harnesses and native browser suites.

The assessed base source pinned Caddy 2.10.2, CertMagic 0.24.0, quic-go 0.54.0 and Go 1.27.1.
Official upstream checks on 16 October identify Caddy 2.11.7, CertMagic 0.25.6
and Go 1.27.2 as current stable/security baselines. Confirm the resolved graph
and scanner results before accepting those versions. Pin govulncheck v1.9.0,
resolved from `golang.org/x/vuln@latest` on this date.

Upgrade risks requiring source inspection and runtime evidence:

- CertMagic events/cache loading and Caddy handshake/connection hooks must still
  reject TLS activation before durable binding coverage exists. Inspect the
  new event-interest filtering as well as existing issuance/load events.
- Proxy deadlines, HTTP/2 rejection/cancellation, streaming and shutdown changed
  upstream; preserve existing real proxy tests, held connections and recovery.
- Exact toolchain and version checks appear in several build/CI/package paths.
  Update them together; retain strict manifest and dependency-origin validation.
  The new Google API/GAX graph imports `encoding/json/jsontext`, which cannot
  compile with the previous `nojsonv2` workaround. Caddy 2.11.7 fixes the old
  RawMessage alias lookup with `reflect.TypeFor`; use explicit `jsonv2`, verify
  its exact runtime identity, and rerun serialization/config/signature tests.
- Independent devtools currently names older production modules. Push real
  implementation commits and resolve actual pseudo-versions before selecting
  the candidate; never invent versions or ship local replacements.
- README and licensing inventory contradict existing packaged installation.
  Canonical operator guides remain the source; maintainer decisions/evidence
  remain outside product routes and search.
- SECURITY.md has an email while publication accepts HTTPS/GitHub reporting.
  Add narrow explicit mailto approval support, preserving separate owner approval
  and all existing immutable-source/legal/tag/workflow/rebuild/write protections.

## Execution and acceptance ledger

The dated report and linked sanitized evidence record newly executed acceptance. Historical results are not substituted for the selected candidate.

| Area | Work and authoritative completion evidence | State |
| --- | --- | --- |
| Upstream/dependencies | Official advisories/release notes, Go security release, coherent Caddy/CertMagic/transitive graph, API/event review, exact compiler/scanner pins across builds/CI/manifests | Complete: reviewed baseline and actual hook regression |
| Security scans | Independent source scans of all four modules and production commands, module-level inventory/dispositions, binary scans of every extracted agent target and packaged Caddy, database retrieval timestamps, enforcing CI | Complete: ten scans; exact OpenPGP disposition; CI enforced |
| Routing regression | Run the configured escaped-path denial against packaged Caddy; both `/admin%2Fpanel` and `/ADMIN%2Fpanel` return 403; separate from binding-gate assertions | Complete: actual old control fails, new package passes |
| Operator documentation | README leads to packages and canonical quickstarts; source installation linked; targets/prerequisites, IPv4/no-AAAA, strict/Firefox, trust/legal/revocation limits, notices/support/release notes/checklist agree | Complete: canonical guides and reviewed private candidate identities |
| Publication policy | Narrow reporting-channel validation and meaningful gate tests; approval false; exact legal hashes/copies/notices; read-only live settings/preflight evidence; concise external-action checklist | Complete: 35 regressions, negative live preflight; owner launch remains separate |
| Content/history audit | Refresh reachable refs/history/current tracked/artifact audit without disclosing secrets; exact Mozilla fixture identity/attribution exception only; scope/limitations recorded | Complete: reachable refs/history, final proposed tree and actual archives |
| Candidate provenance | Real pushed core/agent/Caddy/devtools pins; independent remote installation; two deterministic exact-source builds; all archive/checksum/mode/inventory/notice/installer checks; no existing installation overwrite | Complete: repeated/local/hosted identical; independent installs and extracted runtimes |
| Supported Ubuntu | Clean Ubuntu 24.04 amd64 extracted packages through both quickstarts, real HTTPS and binding verification, umasks 077/0777/repeat/preservation, credential/prerequisite/conflict rejection, ACME/hook renewal, restart/runtime recreation/boot, failures/expiry/recovery/retained coverage | Complete: actual installed packages, both full reboots and both complete lifecycle smokes |
| Repository/lifecycle | Four workspace and independent-module race suites, boundaries, actual installed-binary Caddy and Nginx smokes; every new failure investigated without weakened assertions | Complete: workspace/independent/consumer/remote smokes pass |
| Native browsers | Sequential fresh Caddy Chrome/Firefox then Nginx Chrome/Firefox; trust/identity/cache/renewal/replacement/held connections; preserved Chromium expired-binding negative assertion with current behavior investigated | Complete: all lifecycle runs pass; preserved Chrome expired-exp incompatibility |
| Website | Check/build/internal links/search/publication boundary, revised installation pages reviewed desktop/mobile | Complete: check/build/link/search/boundary and 16 browser checks; screenshots reviewed |
| Delivery | Final diff review, dated report with exact sources/tools/candidates/results/limits, direct experimental-alpha readiness verdict, focused PR against main with precise pending launch actions | Complete: final report and diff review; delivered through the associated focused PR |

Use ignored `.local/alpha-readiness/` for raw evidence and generated fixtures.
Keep historical reports unchanged. The final report must distinguish newly run
checks from inherited evidence, runtime targets from cross-compilation, verified
settings from uninspectable settings, and technical readiness from owner launch
actions. Environment-dependent missing checks prevent claiming their guarantees.

Before completion, audit every row against actual files, command results,
candidate contents, native/runtime behavior and PR/CI state. A green build alone
does not establish any broader requirement.
