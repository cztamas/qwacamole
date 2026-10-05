# Firefox research and implementation

Research date: 5 October 2026. Installed stock Firefox: **157.0**, build
`20260924084938`, Mozilla release revision
`8eb25af4acf031ab1e06abf1a912275083c820ed`.

## Findings

Mozilla implemented the [2-QWAC verifier](https://bugzilla.mozilla.org/show_bug.cgi?id=1996724)
and [desktop QWAC UI](https://bugzilla.mozilla.org/show_bug.cgi?id=2003902) in
Firefox 148, then [enabled desktop QWACs by default in Firefox 153](https://bugzilla.mozilla.org/show_bug.cgi?id=2043399).
The installed binary contains the native `QWACs.sys.mjs` verifier and identity
panel code. Its English qualified label is
`Qualified as specified in Regulation (EU) 2024/1183.` QWAC verification starts
when the user opens the identity panel; page navigation alone is insufficient.

Ordinary extra TLS roots can be imported with NSS `certutil` into a fresh
profile's `cert9.db`, then Firefox can launch with that profile. This avoids the
system trust store and the user's regular Firefox profile. TLS verification must
remain enabled (`acceptInsecureCerts: false` during WebDriver checks).

Firefox's QWAC trust domain is separate from NSS TLS trust. Importing a root
into NSS does not make it a QWAC authority. The stock verifier reads its compiled
QWAC authority list and supports a separate compiled **test** list behind
`security.qwacs.enable_test_trust_anchors`. It does not expose arbitrary QWAC
authority injection through NSS. See the exact release
[trust domain](https://hg.mozilla.org/releases/mozilla-release/raw-file/8eb25af4acf031ab1e06abf1a912275083c820ed/security/manager/ssl/QWACTrustDomain.cpp)
and [test-anchor implementation](https://hg.mozilla.org/releases/mozilla-release/raw-file/8eb25af4acf031ab1e06abf1a912275083c820ed/security/manager/ssl/qwac_trust_anchors/src/lib.rs).

Mozilla publishes the test CA certificate and its test-only RSA key material.
Issuing our generated synthetic QWAC intermediate/leaf beneath that recognized
test authority permits native verification in the tested stock Firefox 157.
This authority is publicly forgeable test material, enabled only in the isolated
demo profile; it must never be used as a production CA.

The installed Firefox binding parser rejects `crit` as an unlisted header.
Our Chrome binding marks `sigD` critical following JAdES. Firefox therefore
needs an explicit compatibility variant that omits `crit`, with the difference
visible in configuration and documentation. The remaining signed Annex B
fields, chain, signature, bound TLS hashes, and expiry checks must remain real.
Chrome's default binding must remain unchanged. Firefox does enforce `exp`;
the existing Chrome expiry exception must not be assumed to apply to Firefox.

## Implementation and validation plan

1. Preserve the existing Chrome quickstart and strict binding verifier.
2. Add a Firefox-specific synthetic PKI generator using the published Mozilla
   test authority, with fresh generated intermediate/leaf keys and signed owner
   data. Pin fixture provenance and verify the root/key match.
3. Add an explicit Firefox binding compatibility setting to signing, local
   verification, persistence, renewal and recovery.
4. Prepare a fresh NSS profile, trust only Pebble for TLS, enable Firefox's
   QWAC test anchors, and map the demo hostname through profile DNS preferences.
5. Add `make firefox-demo`, sharing the existing owned service lifecycle and
   opening visible stock Firefox. Record exact browser version and native UI
   evidence; leave the native qualified panel visible for inspection.
6. Validate native owner/qualified UI, real HTTPS, automatic renewal with the
   same profile, trust separation, signature/binding failures, expiry rejection,
   and owned-process shutdown. Add README quickstart and report executed results.

## Observed native results

The exact `make firefox-demo` quickstart passed with `Firefox Synthetic Owner
TEST`, Budapest, HU, NTRHU-TEST-0001. The user confirmed both the organization
and qualified label were visible and supplied
[the native panel screenshot](images/firefox-qualified-status.png).
The native EU trust mark is present. The custom TLS root warning is expected;
the panel's “Verified by” label names the ordinary Pebble TLS issuer.

Mozilla WebDriver's official [chrome-context access](https://firefox-source-docs.mozilla.org/testing/geckodriver/Flags.html)
(`--allow-system-access`) opens the real browser identity panel and reads its
native state. It does not assign the QWAC result, edit the panel, or use page HTML
as verification. Positive assertions require `secure-etsi`, visible qualified
text, matching signed organization/country, and the exact generated QWAC leaf's
SHA-256 fingerprint. `acceptInsecureCerts` remains false. TLS negatives assert
the native failed-certificate security-info error code. Binding negatives
require the intended native verifier console message as well as trusted TLS and
absence of qualified UI.

The profile pins the stock legacy identity panel using
`browser.urlbar.trustPanel.featureGate=false`. Under the tested WebDriver launch, the 157 build's new Trust
Panel accepts the binding but fails to open its details subview with
`showSubView is not a function`. The legacy panel displays the native qualified
state correctly. Gecko's screenshot API omits macOS popup windows even when
native `checkVisibility()` is true; the committed image was supplied by the user.

Raw state: `.local/browser-demo-1558749535/`. Raw research, commands and suite
output: `.local/firefox-research/`. These directories stay ignored.

- QWAC leaf SHA-256:
  `45c5047f11f33f85369909b5b69d6332b114a7341e673fca194aa5866b6313a1`.
- Initial TLS leaf SHA-256:
  `447923209c1e1dac96a10b0a5086d079c734334717583e568c24d6de70ba3b7a`.
- Automatically renewed TLS leaf SHA-256:
  `20705dab8592b315a0c57b39ac8edf88fee6da5797cfbaadbaafafcc4a67505b`.
- Before and after renewal, the exact same `profiles/renewal` NSS profile
  verified `secure-etsi`, visible qualified text, the same signed owner, and the
  same QWAC fingerprint. Caddy was not restarted and trust was not patched.
  The current manifest retained both TLS identifiers.
- Removing only QWAC test-authority trust left HTTPS working (`secure`), with no
  qualified owner or QWAC certificate. Removing only TLS trust failed with
  `SEC_ERROR_UNKNOWN_ISSUER` despite QWAC trust being enabled.
- Signature corruption: `invalid signature`; wrong TLS hash:
  `TLS binding does not cover server certificate`; wrong QWAC hostname/profile:
  `signing certificate not 2-QWAC`; expired binding: `header has expired`.
- TLS hostname mismatch: `SSL_ERROR_BAD_CERT_DOMAIN`; expired TLS leaf:
  `SEC_ERROR_EXPIRED_CERTIFICATE`.
- A binding containing `crit` is rejected with `header contains invalid parameter`.
  The explicit compatibility variant is accepted. This is a Firefox profile
  difference, not a change to the default Chrome binding.

## Reproduce the checks

From the repository root, run the README's `make firefox-demo` command and copy
its printed state path. In another terminal:

```sh
STATE=/absolute/path/to/.local/browser-demo-123456
make firefox-check STATE="$STATE"

# Reuse one unchanged NSS profile across automatic TLS renewal.
bin/qwacamole firefox-check -state "$STATE" -profile "$STATE/profiles/renewal" -name before-renewal
# Wait for a new TLS ID in bindings/current.json (typically a few minutes).
bin/qwacamole firefox-check -state "$STATE" -profile "$STATE/profiles/renewal" -reuse-profile -name after-renewal
```

Compare `native_ui.tls_fingerprint` in the two result JSON files under
`evidence/firefox-before-renewal/` and `evidence/firefox-after-renewal/`. They
must differ; the QWAC fingerprint, signed owner and qualified state must match.
The original visible session can retain its old TLS connection; these two checks
make fresh connections with unchanged trust.

To run the native rejection suite, start its separate test fixture server in
another terminal, using the same Firefox-generated state:

```sh
bin/qwacamole fixture -state "$STATE"
```

Then, from a third terminal with `STATE` set to that same path:

```sh
make firefox-test STATE="$STATE"
```

The suite requires eleven passing cases, including signature, TLS hash,
QWAC hostname/profile, binding expiry, both trust-removal controls, both TLS
hostname/expiry failures, and the `crit` incompatibility control. It writes
`evidence/firefox-suite.json` and per-case native result/console files. Stop the
fixture terminal with Ctrl-C afterward. It uses loopback ports 9443–9445.

Individual trust checks are also available against the running ACME demo:

```sh
bin/qwacamole firefox-check -state "$STATE" -qwac-trust=false -name no-qwac-trust -expect no-qwac-trust
bin/qwacamole firefox-check -state "$STATE" -tls-trust=false -name no-tls-trust -expect tls-untrusted
```

Go race tests cover both strict and Firefox-compatible atomic rotation,
signing/publication failure, concurrent readers and restart recovery. The
compatibility binding tests additionally check signatures, wrong hashes,
hostnames, expiry, strict-profile separation and Mozilla-root CRL signatures.
Generated keys/profiles stay local; only Mozilla's already-public fixture key
is intentionally versioned, with pinned attribution and license notice.


Final execution: `make firefox-test STATE=.local/browser-demo-1558749535`
passed all eleven cases after clearing console evidence on every new run.
`make test` passed. The original `make browser-demo` quickstart and a separate
`make browser-check` against its ACME state passed Chrome 154 native binding
verification. Both visible demo supervisors printed their native verification
PASS when interrupted; owned service/fixture processes were stopped and raw
state was preserved.
