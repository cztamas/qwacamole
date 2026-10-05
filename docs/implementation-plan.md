# Local 2-QWAC implementation plan

Objective: stock Chrome must verify a genuine Annex B binding through a separate
test QWAC authority, including after Caddy automatically renews ACME TLS.
Generated keys, profiles, and raw evidence remain in ignored `.local/`.

1. Inspect ETSI TS 119 411-5 V2.1.1 and exact installed Chrome source; pin Go,
   Caddy, CertMagic, xcaddy, and Pebble. Establish issuer lifecycle ordering.
2. Implement Go test PKI and RS256 Annex B bindings, plus a lossless protobuf
   component patcher and isolated browser launcher. First prove real TLS trust
   and native QWAC verification for an externally generated binding.
3. Implement shared atomic/versioned binding publication and a Caddy ACME issuer
   wrapper that signs, verifies, and persists before returning issued material.
   Cover old/new TLS certificates, startup recovery, and independent refresh.
4. Implement Go local orchestration, Pebble routing with enabled challenges,
   short-lived issuance, and browser NetLog assertions. Observe automatic renewal
   without changing browser trust or restarting services.
5. Exercise trust separation, all specified malformed bindings and TLS failures,
   failure injection, old connections, restart, and independent refresh.
6. Deliver reproducible Make targets, diagnostic page, README, evidence report,
   and scoped cleanup. Mark complete only with observed browser acceptance and
   all acceptance tests passing.

Initial environment: macOS arm64; Chrome 154.0.8037.95; installed PKIMetadata
1793. Docker Desktop available. Go absent, so install checksum-verified Go 1.27.1
inside ignored `.tools/`. Demo hostname `qwac.example.com`; loopback services only.

Progress: implementation and native positive/renewal, trust separation, negative
TLS/profile/signature/hash, failure recovery, restart, and refresh evidence are
present. Chrome 154 accepts expired binding headers. Both official Canary
distributions were investigated; neither supplied usable alternative expiry
proof. See `docs/evidence.md` and the final fresh run at
`.local/integration-20261004-185437/`. Source snapshots are in `.local/upstream/`.

Historical blocked audit: the same native expiry incompatibility remained across three
consecutive goal turns. The final audit rechecked the installed browser version,
the failing native expiry result, initial/renewed acceptance, strengthened live
publication-failure recovery, race-test results, and cleanup. No project service
remains running. Further progress on the required expiry negative depends on a
compatible stock browser that enforces `exp` was needed for the original strict
criterion.

Accepted exception, 4 October 2026: the repository owner explicitly accepted the
demonstration with the existing Chrome expiry implementation gap and requested a
comprehensive first-time/manual-validation README, followed by commit, push, and
goal completion. The failing expiry assertion stays visible. This acceptance
supersedes the earlier blocked completion condition; it does not claim native
expiry rejection has been achieved.

The comprehensive README is implemented and its fresh-state manual setup,
trusted HTTPS, native initial/renewed/restarted verification, independent refresh,
and individual trust-separation controls were exercised successfully. The source,
config examples, and documentation are ready for the requested commit and push;
generated keys, profiles, and raw logs stay ignored and local.

## Visible browser extension

The follow-up goal is a visible stock-browser quickstart with configurable signed
owner data and observed native EU Qualified Status, matching the supplied
Security-panel reference. The implementation adds `make browser-demo`, creating
fresh marked state, generating the selected QWAC identity, starting real Pebble /
Caddy / DNS, preparing isolated trust, and keeping Chrome open for inspection.
The page and README explain how to inspect Chrome's native panel.

Native UI proof was supplied by the user: Chrome shows the expected synthetic
organization and HU country, and its QWAC certificate viewer fingerprint matches
the generated leaf exactly. Screenshots are preserved in `docs/images/`.
Race tests and the complete native rejection-control suite pass except for the
previously accepted Chromium expiry gap. Shutdown was exercised. A later user
session exposed a separate retention bug: Chrome kept its initial TLS connection
beyond leaf expiry, but refresh pruned that leaf from the current binding.

The binding manager now retains previously served TLS hashes for the lifetime of
the local demo state. Effective binding expiry remains bounded by the latest TLS
expiry and QWAC expiry; new TLS handshakes still reject expired certificates.
Unit coverage checks expired-leaf overlap through publication, refresh, and
restart. Browser positive assertions now reject later failures after earlier
success. A documented `-through-tls-expiry` check keeps one browser connection
active past the original certificate's expiry and fails if it reconnects.
The exact Make quickstart passed. The real Chrome regression retained the same
TLS leaf through expiry and a complete subsequent binding lifetime, with 19
successful native binding checks and no errors. The final native acceptance was
45 seconds after the original leaf's expiry. The race suite and all other native
controls passed; the previously accepted `exp` gap remains explicit. Raw evidence
remains in `.local/`.


## Firefox extension

Implemented `make firefox-demo`, sharing the owned Pebble/Caddy/DNS supervisor
and synthetic owner flags with the visible Chrome command. Firefox 157.0 accepts
real TLS through an NSS root imported only into a new profile. QWAC trust uses
Mozilla's independent compiled test-authority list; fresh intermediate/leaf
certificates descend from its pinned, already-public test CA/key. Binding signing,
verification, refresh and recovery select an explicit Firefox compatibility
variant without `crit`; the strict Chrome path is preserved.

Native Firefox returns the exact generated QWAC fingerprint, matching owner and
country, `secure-etsi`, and visible qualified text. The user confirmed the panel
and supplied its screenshot. Automatic TLS renewal was accepted using the exact
same NSS profile with distinct old/new TLS IDs. All eleven native rejection
controls passed, including both trust domains, cryptographic/hash/profile/hostname
failures, binding expiry and ordinary TLS hostname/expiry. Race tests exercise
both profiles through atomic rotation, injected failure and restart recovery.
The README includes setup, manual native inspection, renewal checks, fixtures,
evidence and cleanup. [Firefox details](firefox.md) document the public test CA,
its expiry, the `crit` incompatibility, and the stock-panel preference workaround.
