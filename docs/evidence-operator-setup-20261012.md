# Packaged operator workflow acceptance — 2026-10-12

The supported path is **install → configure → start → verify**, using supplied
credentials and an explicitly selected authority. Automated setup targets Ubuntu
24.04 amd64, one hostname and a dedicated integration host. Advanced source,
custom Caddy and manual deployment instructions remain separate. PR #8 was already
merged; this work follows main `49b565988715a212f1ff739fe9bb75e36e28b933` in
[PR #9](https://github.com/cztamas/qwacamole/pull/9).

This report records execution during this implementation. Earlier dated reports
are historical context, not substituted acceptance evidence. No public release,
tag, website deployment, visibility, external DNS or external account change was
performed.

## Final source and packages

Final production implementation: `c13676e0805fae7389b1c3de32f91cb6521c9c3b`.
Real shared-core requirement: `v0.0.0-20261012130422-b3fb51784945`, resolving
`b3fb517849451b5f201a5ad6682ed7516d9ab858`. Caddy is v2.10.2, CertMagic v0.24.0;
Go is 1.27.1 with `GOEXPERIMENT=nojsonv2`, CGO disabled. The manifest records
the complete linked Caddy dependency identities and checksums. Archives include
third-party notices, installation information, templates and executable installers.

Executed twice from separate exact-source staging directories:

```sh
GOPRIVATE=github.com/cztamas/qwacamole python3 scripts/release-agent.py build \
  --source c13676e0805fae7389b1c3de32f91cb6521c9c3b --version 0.1.0-alpha.1 \
  --output .local/operator-candidate-v8 --go .tools/go/bin/go
# Second invocation uses .local/operator-candidate-v8-repeat.
cmp .local/operator-candidate-v8/SHA256SUMS .local/operator-candidate-v8-repeat/SHA256SUMS
python3 scripts/release-agent.py verify --smoke --require-native .local/operator-candidate-v8
```

All four archives and manifest were byte-identical. Authenticated download of
[candidate run 38254907098](https://github.com/cztamas/qwacamole/actions/runs/38254907098)
using `gh run download --name agent-candidate-c13676e0805fae7389b1c3de32f91cb6521c9c3b`
also matched both local builds; strict archive and extracted macOS version checks
passed. The reviewed manifest SHA-256 is
`eab797956fd53dd6ddc38a737511dc13e1707646670e90302f5080e820b9b1e9`.
[Manifest](evidence/operator-setup-20261012/candidate-manifest.json) and
[archive checksums](evidence/operator-setup-20261012/candidate-SHA256SUMS) are retained.
These are authenticated candidates, not published releases.

| Component/target | Execution established |
| --- | --- |
| Agent Linux amd64 | Extracted installer, setup, services, real ACME/HTTPS and lifecycle on Ubuntu 24.04 |
| Agent macOS arm64 | Extracted version identity and native integration execution |
| Agent Linux arm64 | Cross-compiled archive, identity, contents/modes/checksums; runtime untested |
| Caddy Linux amd64 | Extracted version/module identity, setup, service, ACME/HTTPS and lifecycle on Ubuntu 24.04 |

Clean Ubuntu containers contained Nginx/systemd/Certbot, without Go, a source
checkout or browser/development PKI tooling. Both final setup implementations
were freshly applied at `1825f77b0cfe49b6a4778eedde33f83e01a11d6b`; the subsequent
agent correction changes only verification's state-directory selection. Final
`c13676e` packages were also extracted/installed and started in the guest below.
Installers produced mode 0755 binaries, reported exact source/version, preserved
existing prefixes and rejected wrong-platform packages. Corrupt archives failed
SHA-256 verification before extraction/installation. Setup locks were root-owned
0600 files under root-owned `/run`.
[Installation negatives](evidence/operator-setup-20261012/installation-negatives.json).

## Supported guest and operator lifecycle

A disposable QEMU guest ran the official checksum-verified Ubuntu 24.04 amd64
cloud image on a software-emulated x86 CPU, with real systemd and full guest
reboots. Its localhost-only SSH forwarding was port 22022. Product deployment
used `/opt/qwacamole-agent` and `/opt/qwacamole-caddy`; no Go or clone was installed
in the guest. Dedicated test-only services supplied an HTTP application on 8080
and Pebble 2.8.0, with actual HTTP-01 validation on port 80 (always-valid disabled).
QWAC inputs were separately generated synthetic fixtures. Synthetic ordinary
TLS roots were explicitly installed only in this disposable guest; setup itself
does not install trust or enroll credentials.

| Check | Newly executed result |
| --- | --- |
| Agent setup `--dry-run`, apply and repeat | Pass; validates complete supplied chain/key/authority, writes owned configuration/hook/services, preserves live state on identical repeat |
| Caddy `qwac-setup --dry-run`, apply and repeat | Pass; actual minimal Caddyfile validation, conventional state, private inputs and dedicated root service |
| Real Certbot issuance and renewal | Pass with Certbot 5.8.0 and Ubuntu's packaged 2.9.0, Nginx 1.24.0; saved installed deploy hook publishes binding before activation; renewed TLS fingerprint changes |
| Caddy ACME issuance | Pass through packaged gated issuer against real Pebble challenge validation; the selected challenge type was not retained |
| Human and JSON verification | HTTPS, discovery, selected authority/signature, current binding and served TLS coverage pass; agent also reports ready Nginx deployment |
| Service restarts | Agent, dedicated Nginx and packaged Caddy restart and return to verified readiness |
| Agent host reboot | Private `/run/qwacamole` recreation, removed volatile marker, new boot ID, verified retained deployment; upload directory remains 0700/www-data |
| Caddy host reboot | Final package starts automatically, preserves state and replacement signer; human/JSON readiness succeeds |
| Generated-setup replacement | New-key, same-authority pair imported first; full stop, protected backup, fixed-path replacement and restart preserve binding/TLS history; both integrations advertise the inspected new leaf fingerprint |
| Expired synthetic TLS recovery | Agent stays closed with an actionable fresh-candidate error; real Certbot renewal through the installed hook recovers without deleting state |

Thirteen setup/preservation checks passed in the actual guest: identical repeat,
valid changed input refusal, wrong hostname, mismatched key, incomplete chain,
wrong authority, operator edit, public key-file mode, unsafe ownership, missing
Certbot, occupied port, unrelated Nginx master, and missing Nginx capability.
The unrelated master remained running. The capability-negative uses an explicit
Nginx `-V` stub; other binaries/listeners and the master test are real.
[Setup negatives](evidence/operator-setup-20261012/setup-negatives.json).

The master-title test did not work under Docker's cross-architecture binfmt
emulation, which hides Nginx's rewritten process title; it passed in the x86
guest. This container result is not claimed as master-detection proof. Pebble
loses its synthetic account database on reboot: the Certbot retest initially
reported `accountDoesNotExist`. New fixture registration was created separately
and copied alongside preserved account files before successful issuance/renewal.
No product state/accounts were deleted to obtain readiness.

Actual execution exposed and corrected three issues: explicit www-data upload
ownership; non-reproducible temporary paths in packaged build metadata; and
readiness verification during TLS rotation. The latter now reads the bounded
committed TLS set, checks it matches the exact public binding generation and
passes all stored/observed leaves to the unchanged strict verifier. The agent
uses its existing `StoreConfig()` path; Caddy defaults to the generated service's
binding directory and accepts `--state` for manual deployments. Generation races
report a retry action rather than accepting mixed state. A real HTTPS regression
covers longer-lived retained certificates, generation mismatch and uncovered
leaves; an agent regression combines private status with actual HTTPS and its
committed binding subdirectory.

Raw execution remains in ignored `.local/operator-*` logs and the protected guest
state. Relevant logs: `operator-vm-v5-fresh-lifecycle`, `operator-vm-v5-after-reboot`,
`operator-vm-caddy-install-start-verify`, `operator-vm-distro-certbot-success`,
`operator-vm-replacement`, `operator-vm-v8-final`, and both
`operator-vm-v8-*-after-reboot`. Failed iterations are retained alongside successes.
Only public, sanitized reports/screenshots are committed; generated keys,
browser profiles and full private state are excluded.

## Module, installed-binary and browser acceptance

Workspace races and independently pinned races passed for all four modules.
The final agent also passed `GOWORK=off` races after the canonical-state fix.
Dependency boundaries passed; all 28 packaging/publication tests passed,
including the complete approved mocked gate and artifact/API/overwrite rejection
cases. Bare Caddyfile Firefox switch and actual JSON booleans are parser-tested;
omission is strict and no production User-Agent profile selection exists.

```sh
scripts/check-independent-modules.sh
scripts/check-boundaries.sh
scripts/check-remote-agent.sh c13676e0805fae7389b1c3de32f91cb6521c9c3b
scripts/check-remote-install.sh 1825f77b0cfe49b6a4778eedde33f83e01a11d6b
bin/qwacamole plugin-smoke -plugin-binary bin/caddy-remote -state .local/operator-final-remote-smoke
bin/qwacamole nginx-smoke -agent-binary bin/qwacamole-agent-remote -state .local/operator-final-remote-nginx
```

Both remote binaries resolve the real `b3fb517` core without local replacements.
Caddy's implementation is unchanged after `1825f77`; the final agent's extra
verification regression/path correction is independently installed at `c13676e`.
The final remote smoke pair was executed sequentially with the `1825f77` binaries:
both pass actual proxying, stable delivery, WebSockets, independent refresh,
renewal/retained connections, same-key/new-key replacement, fail-closed expiry
and recovery. Agent signing/document/manifest/reload/zero-exit nonactivation,
invalid inputs/configuration and over-capacity candidates preserve prior activation.
Caddy also passes independent sites, unchanged unprotected routes, error handlers,
real file serving and full-stop retirement preserving normal TLS storage.
Raw logs: `.local/operator-final-{remote-smoke,remote-nginx,independent-modules}.log`.

Four fresh native runs were sequential: Caddy Chrome, Caddy Firefox, Nginx Chrome,
Nginx Firefox. Stock Chrome was **154.0.8037.99**, genuine PKIMetadata **1798**;
Firefox **157.0.1**, geckodriver **0.37.1**, local Nginx **1.30.5**, Certbot **5.8.0**.

| Native run | Result |
| --- | --- |
| Caddy Chrome | Lifecycle/cached discovery/held connection/replacement/restart/recovery pass; fixture suite exits 1 solely for the known expired-binding assertion |
| Caddy Firefox | Lifecycle and all eleven fixtures pass with explicit compatibility profile |
| Nginx Chrome | Native initial, renewed cached identity and same-authority replacement pass; real Certbot/held-stream/upload/failure/restart checks pass |
| Nginx Firefox | Same lifecycle/native checks pass with explicit compatibility profile |

Exact retained Chrome failure: **`BROWSER INCOMPATIBILITY: native verifier accepted
expired exp header`**. No assertion was skipped or weakened and no additional
native failure occurred. [Chrome fixture report](evidence/operator-setup-20261012/chrome-suite.json)
and [Firefox report](evidence/operator-setup-20261012/firefox-suite.json) are retained.
These workspace browser binaries intentionally use local module replacements;
their VCS metadata records modified builds at Caddy/harness `35d1f16` and agent
`4090534` (worker fix present). They are lifecycle/browser evidence, separate
from final candidate/remote provenance. Later changes concern packaging/setup
locks and verification commands, which the final Linux guest/remote tests cover.

## Documentation and hosted checks

Canonical guides have four primary steps and expected verification output.
Source installation, JSON/custom arrangements, credential maintenance and
synthetic demonstrations are linked separately. Caddy retains the explicit
literal hostname because the adapter helper exposes no stable single-site
hostname API; optional state/lifetime use tested defaults. Product navigation,
search and assets use the explicit allowlist; this report and the engineering
plan remain unpublished.

Final website execution: Node **24.21.0**; `npm run check` has zero errors,
warnings/hints and **14 script tests**. Build validates **16 HTML pages, 703
internal references, 14 indexed product pages and 29 search assets**.
All **12 Playwright tests pass**, including exact Pagefind routes, excluded
maintainer routes, mobile menu/search focus, keyboard code scrolling, contrast
and accessibility. The new short apt block exposed a stale “first code block”
fixture; it now selects the long setup command, retaining the scrolling assertion.
Six production-preview screenshots cover quickstarts/install at 1440 and 390px;
visual review, no page overflow and zero axe violations pass.
[Layout evidence](evidence/operator-setup-20261012/website-review.json).
An exploratory dev preview left running during builds reported generated-cache
temporary-file errors; final review used the completed production preview and
the production browser suite above. Both owned preview servers were stopped.

Hosted checks passed at final review head
`25d62d17d687739e723d4cd8b91646785793d6c4`:
[candidate packages/macOS execution](https://github.com/cztamas/qwacamole/actions/runs/38256579106),
[documentation checks](https://github.com/cztamas/qwacamole/actions/runs/38256579135)
and [portable integration](https://github.com/cztamas/qwacamole/actions/runs/38256579101).
The portable job passed all module/independent races, boundaries, default/custom
Nginx policy, offline migration and real Caddy lifecycle checks. Website deployment
was skipped. Candidate/documentation checks also passed at the installed production
source `c13676e` (runs 38254907098 and 38254907361). Intermediate documentation
failure at `1825f77` was the code-block fixture above, fixed and proven
locally/hosted at `c13676e`. This final evidence-only update records the completed
run and clarifies the unrecorded Caddy challenge type; it changes no product code,
public guides or publication map. Subsequent documentation-only commits receive
their own PR checks; this report does not claim those future results.

Remaining limits: public releases/downloads and anonymous install are unavailable;
Linux arm64 runtime, Windows, macOS service setup/notarization and other service
managers were not tested. Tests use synthetic credentials and ACME, not a
commercial qualified issuer or public Let's Encrypt/DNS. Server readiness does
not establish native browser trust, legal qualification or revocation status.
Support remains best-effort. MIT/security contact and publication approval
controls are preserved.
