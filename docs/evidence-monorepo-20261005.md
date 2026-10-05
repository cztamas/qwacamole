# Monorepo verification — 5 October 2026

Starting commit: `7cf2fcf3d53580aec6d237c100655d9625a49c27`.
The extraction separates a standard-library core, Caddy integration, and development
tooling. This report records new execution; [the earlier evidence](evidence.md)
and [Firefox's initial implementation evidence](firefox.md) remain historical.

## Baseline

`make build` and `make test` passed before source edits. Stock Chrome
154.0.8037.95 with genuine PKIMetadata 1793 and stock Firefox 157.0 were both run
visibly. The owner confirmed their native qualified status and signed owners.

- [Chrome baseline native panel](images/monorepo-baseline-chrome-20261005.png):
  `Monorepo Baseline TEST [HU]`.
- [Firefox baseline native panel](images/monorepo-baseline-firefox-20261005.png):
  `Monorepo Firefox Baseline TEST`, HU, qualified label and EU trust mark.

These images are baseline proof, not screenshots of the extracted build.
Raw baseline evidence: `.local/refactor-20261005/baseline/`. The first fixture
attempt lost its subprocess and returned connection refusals; it was rejected.
The persistent-session replay exited **1**, with ten passes and only
`BROWSER INCOMPATIBILITY: native verifier accepted expired exp header` failing.

## Modules and builds

Executed from the repository root:

```sh
make build
make test
make plugin-build
scripts/check-consumer.sh
python3 .local/refactor-20261005/check-legacy-state.py
```

All passed. `make test` runs `go test -race ./...` separately at the root, in
`caddy`, and in `devtools`, then checks dependency graphs. The core graph contains
no Caddy/CertMagic/browser/PKI generator. The plugin graph and plugin-only binary
contain no devtools module or public Mozilla test CA/key fixture. Production
registrations are exactly `tls.issuance.qwac` and `http.handlers.qwac`; no demo
registrations appear. The outside-workspace consumer builds with `GOWORK=off`
and explicit local core/plugin replacements, then verifies those registrations.

The preserved PKI-dependent tests now exercise exported core APIs from devtools.
They cover strict and Firefox-compatible rotation, signing and manifest commit
failures, concurrent readers, unadvertised candidates, recovery, and expired-leaf
retention. An injected corrupted signature must fail independent verification
before any immutable document is written. Production middleware tests prove
application routes pass to the next handler, including paths that used to be
reserved for diagnostics, and production operations ignore demo fault markers.
Loader and atomic-write tests run in the root module. The legacy-state consumer
successfully opens both pre-refactor Chrome and Firefox manifests, retaining
exact owner data and TLS IDs. It uses copies, preserving the baseline evidence.

Pins remain Go 1.27.1, Caddy 2.10.2, CertMagic 0.24.0, xcaddy 0.4.5,
Pebble 2.8.0, chromedp 0.14.2 and protobuf 1.36.10. Shared dependency versions
were compared against the starting go.mod and remain unchanged.
`GOEXPERIMENT=nojsonv2` remains enabled. No modules or tags were published.

Build/race/boundary/consumer logs: `.local/refactor-20261005/`. Plugin module
inventory: `plugin-build-modules.txt`, registrations: `plugin-modules.txt`.

## Fresh Chrome execution

Fresh state: `.local/refactor-20261005/final/browser-demo-4060613876/`.
All commands below used the refactored binary; `C` denotes that absolute path:

```sh
bin/qwacamole browser-demo -state .local/refactor-20261005/final -organization 'Monorepo Chrome TEST'
bin/qwacamole browser -state "$C" -ct-exception -tls-root "$C/pki/pebble-root.pem" -profile "$C/profiles/lifecycle" -name initial
bin/qwacamole browser -state "$C" -ct-exception -tls-root "$C/pki/pebble-root.pem" -name through-expiry -through-tls-expiry
bin/qwacamole lifecycle -state "$C"
bin/qwacamole browser -state "$C" -ct-exception -tls-root "$C/pki/pebble-root.pem" -profile "$C/profiles/lifecycle" -reuse-profile -name renewed
bin/qwacamole refresh-test -state "$C"
bin/qwacamole fixture -state "$C"
bin/qwacamole browser-suite -state "$C"
# After through-expiry completed:
touch "$C/restart-request"
bin/qwacamole browser -state "$C" -ct-exception -tls-root "$C/pki/pebble-root.pem" -profile "$C/profiles/lifecycle" -reuse-profile -name restarted
bin/qwacamole failure-test -state "$C"
bin/qwacamole browser -state "$C" -ct-exception -tls-root "$C/pki/pebble-root.pem" -profile "$C/profiles/lifecycle" -reuse-profile -name failure-recovery
```

All lifecycle/native commands exited **0**. The native suite exited **1** with
exactly ten passes and the unchanged ignored-exp failure. Services and fixture
were held in persistent owned sessions and stopped before starting Firefox.
The visible Chrome supervisor also printed `PASS native Chrome QWAC verification`
on shutdown; its native NetLog verifies the injected QWAC authority and binding.
Final visible native-panel inspection passed. Computer Use first reported a
locked Mac, then unavailable permissions after unlocking; the owner instead
supplied [the final native Chrome Security panel screenshot](images/monorepo-final-chrome-20261005.png)
at 15:35 on 5 October. It shows secure HTTPS, EU Qualified Status and exactly
`Monorepo Chrome TEST [HU]`. This is the native panel, not the demo page.

For this final visual check, the unchanged refactored binary was launched again
with fresh isolated state after stopping Firefox:

```sh
bin/qwacamole browser-demo -state .local/refactor-20261005/final-ui -organization 'Monorepo Chrome TEST'
```

Its separate state is `.local/refactor-20261005/final-ui/browser-demo-526978856/`.
The full lifecycle/negative/expiry evidence remains in the original final Chrome
state above; the second run provides the newly inspected native panel. Its
supervisor exited **0** and printed `PASS native Chrome QWAC verification` after
owned shutdown. `evidence/interactive-result.json` and the flushed raw NetLog
again prove native binding success, root-store version 1000000 and the exact
injected QWAC root. The owner also explicitly confirmed that qualified status
and owner matched. Both final demo supervisors and fixtures were stopped; raw
keys, profiles and evidence are preserved locally, outside version control.

Initial native TLS SHA-256:
`a2d5a7a2c011103bd433f1f9416e69ccd69dd9e56057408dc1f8154d8ca1c775`.
Renewed native TLS SHA-256:
`d52afc5e7d0ce8571f10982bf3722ac3b6caba777c84cb01bb56278de93ecb5a`.
The exact same lifecycle profile and trust were reused. The overlap probe saw
replacement `a39c3c15918c40d9ce2c1169bda1f378d6b790ce754280aaca38f78cee573c17`
while retaining the original HTTP/1 connection; both hashes were bound.
Caddy/Pebble logs record automatic issuance and cache replacement; no manual
binding generation, restart or profile patch caused renewal.

The HTTP/2 expiry check retained the initial TLS ID above through its
`2026-10-05T09:59:26Z` expiry, checking again at
`2026-10-05T10:00:13.026286Z` with **the identical TLS ID** and native acceptance.
NetLog's 18 main navigations used one `h2` session (source 310); native binding
fetches used a separate session (369). This distinguishes the held application
connection from the verifier's fetch connection. `http2-session-proof.json`
records that mapping; the full raw NetLog preserves the headers and source IDs.

Independent refresh changed the binding URL in 4.01 seconds with an unchanged
TLS ID set. Caddy-only restart changed PID 19415 to 21973 while Pebble remained
19413. Both failure stages retained TLS ID
`08ef951e364856d70462e77297826612aa9ba98d2438dc1e099f9cad2c6eb79e`,
with valid old HTTPS/binding and replacement activation rejected. Automatic retry
recovered with
`fa56ddeac0cb0b6b28fead8e830532926fe260b95a64579e9339df8fea6f94b4`;
the same profile then passed native verification.

Within `$C/evidence/`, see `initial-result.json`, `renewed-result.json`,
`rotation.json`, `refresh-test.json`, `restart.json`, `failure-test.json`,
`failure-recovery-result.json`, `through-expiry-result.json`, their raw NetLogs,
`browser-suite.json`, and `interactive-result.json`. The suite's exit and exact
failure are also recorded at `final-chrome-suite-outcome.json` in the parent
refactor directory.

## Fresh Firefox execution

Fresh state: `.local/refactor-20261005/final/browser-demo-3419361122/`.
The native checks, renewal runner, suite and visible supervisor passed (exit **0**). After the owner supplied its final panel screenshot, the owned Firefox supervisor was stopped; it printed `PASS native Firefox QWAC verification` and cleaned up its children. `F` denotes that absolute state path:

```sh
bin/qwacamole firefox-demo -state .local/refactor-20261005/final -organization 'Monorepo Firefox TEST'
bin/qwacamole firefox-check -state "$F" -profile "$F/profiles/renewal" -name before-renewal
bin/qwacamole fixture -state "$F"
make firefox-test STATE="$F"
# Waited for a new TLS ID in current.json, without restarting Caddy or changing trust:
bin/qwacamole firefox-check -state "$F" -profile "$F/profiles/renewal" -reuse-profile -name after-renewal
```

The visible native panel reports `Monorepo Firefox TEST`, HU, `secure-etsi`,
matching generated QWAC fingerprint and visible qualified text, with
`acceptInsecureCerts=false`. The owner supplied [the final native Firefox panel screenshot](images/monorepo-final-firefox-20261005.png) at 15:33 on 5 October. It confirms the expected organization, HU, EU trust mark and qualified label. This is final refactor evidence, separate from the baseline screenshot; the native UI assertions are also recorded independently.

Old TLS SHA-256:
`1394dfab0e563b4842a3f2ca08188f8b1bfb7158d5d4f08c89d94800d2075d8e`.
Automatically renewed TLS SHA-256:
`d052fc5f01b97ba8f33683ab5a91ad9c98d51bd1ec12547cf3e34e6d2a2a87c9`.
Both are in the current binding. The unchanged QWAC SHA-256 is
`1328810a366fcb61069c11cc910b26cde8fada4378c269e35d530f6f2a51a1df`.
Both checks used the exact same `profiles/renewal` directory without repatching
NSS or preferences. Native owner/country, `secure-etsi`, qualified text and
visibility matched before/after. The timing/compare runner is preserved at
`.local/refactor-20261005/firefox-renewal.py`; its assertions and Caddy logs record
real automatic renewal, not a manual certificate swap.

All eleven native fixture cases passed: positive, QWAC trust removal, TLS trust
removal, corrupted signature, wrong TLS hash, QWAC hostname, binding expiry,
QWAC profile, TLS hostname, TLS expiry and standard-crit rejection. Firefox
rejected expired exp with `header has expired`; no Chrome exception was applied.

Within `$F/evidence/`, see `firefox-result.json`,
`firefox-before-renewal/firefox-result.json`,
`firefox-after-renewal/firefox-result.json`, `firefox-renewal.json`,
`firefox-suite.json`, per-case native JSON/console files and Caddy/Pebble logs.
Gecko screenshots omit macOS popup windows; native UI visibility/identity is
read through official WebDriver chrome context. No page text supplies acceptance.

## Compatibility and limits

The README documents all three module paths, APIs, xcaddy replacement arguments,
external-consumer setup and configuration migration. Binding credentials now use
explicit `chain_files` and `key_file` instead of a synthetic `pki` directory.
Diagnostic routes and fault injection require the explicitly development-only
module IDs. Manifest fields, URLs, profile markers, signed document format and
TLS identifiers remain compatible. Core signing is strict by default; Firefox
compatibility is explicitly selected and never inferred from User-Agent.

The unchanged retention policy keeps all published TLS certificates and immutable
documents for the state lifetime, including expired leaves on authenticated
connections. Disk usage and binding size grow with renewals; this stage adds no
GC or distributed storage. Mozilla's public test root/key remains devtools-only,
with attribution and its 5 February 2027 expiry. Stock Chrome still ignores exp;
its failing assertion/nonzero suite remains visible. Test trust, Firefox legacy
panel selection, and Chrome's isolated process-wide CT-only exception are
unchanged. Nothing here asserts legal qualification or production readiness.
