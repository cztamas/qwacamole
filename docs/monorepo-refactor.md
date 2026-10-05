# Monorepo extraction — 5 October 2026

Starting commit: `7cf2fcf3d53580aec6d237c100655d9625a49c27`.
Baseline `make build` and `make test` (race detector) passed. Pins: Go 1.27.1,
Caddy 2.10.2, CertMagic 0.24.0, xcaddy 0.4.5, Pebble 2.8.0,
Chrome 154.0.8037.95 / genuine PKIMetadata 1793, Firefox 157.0,
geckodriver 0.37.1. Preserve `GOEXPERIMENT=nojsonv2`.

Baseline commands and raw evidence are under `.local/refactor-20261005/baseline/`.
Visible Chrome `browser-demo` with `Monorepo Baseline TEST` was confirmed by the
owner and their native-panel screenshot. A separate native browser check passed.
Visible Firefox `firefox-demo` with `Monorepo Firefox Baseline TEST` reported
`secure-etsi`, matching organization/country and QWAC fingerprint, and visible
qualified text; `acceptInsecureCerts=false`. These are baseline observations,
not verification of the extraction.

The first Chrome fixture suite was invalid: the fixture subprocess exited and
all cases received connection refusals. It is preserved separately and replayed
with a persistent fixture session. Only the exact ignored-exp failure is an
accepted incompatibility; no other nonzero outcome is accepted.

## Implementation plan

1. Extract standard-library core packages `credentials`, `profile`, `binding`,
   and `store`; supply explicit credentials, preserve strict/default and explicit
   Firefox profiles and manifest fields. Inject signing and manifest commits at
   their actual boundaries instead of reading demo fault files in production.
2. Extract the Caddy module with unchanged production module IDs and ACME gate.
   Make its HTTP handler middleware. Keep shared managers at the integration
   boundary; move diagnostic HTTP routes and fault adapters into devtools.
3. Move CLI, PKI, fixtures, browsers and PKI-dependent integration tests into
   devtools. Connect three modules with a versioned go.work; build xcaddy with
   explicit local replacements. Preserve pins and root workflows.
4. Run race tests across all modules, enforce dependency boundaries, build the
   plugin alone and an outside-workspace consumer. Test configuration migration.
5. Run fresh native Chrome and Firefox acceptance, real renewal using unchanged
   profiles, all rejection controls, restart/refresh/failure recovery and Chrome
   HTTP/2 through expiry. Record dated results separately from historical evidence.

No retention redesign, dependency upgrades, module publishing, or extra server
integrations are included. Documents and TLS certificate hashes remain retained
for the state lifetime, including expired leaves on authenticated connections.

## Executed result

The extraction, all-module race tests, dependency-boundary checks, plugin-only
xcaddy build and outside-workspace consumer passed. Old Chrome/Firefox manifests
load through the extracted core without an identity/coverage migration. Fresh
Chrome lifecycle/expiry/failure/recovery checks passed; its negative suite keeps
exactly the known exp failure and exit 1. Fresh Firefox renewal with unchanged
trust and all eleven native fixture checks passed. Final visible-panel inspection
is pending because Computer Use reported the Mac locked. See
[the separate dated execution report](evidence-monorepo-20261005.md) for exact
commands, fingerprints, raw evidence paths and limits.
