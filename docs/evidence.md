# Execution evidence — 4 October 2026

The repository owner accepted the demonstration on **4 October 2026**, with
Chrome's native acceptance of expired binding `exp` as a documented implementation
gap. The strict expiry assertion remains failing; it has not been removed,
skipped, or relabeled as a pass. The original goal was blocked on this requirement
before that explicit acceptance. The remaining requirements have observed proof
below. No browser extension, custom Chromium build, JavaScript status indicator,
or certificate error bypass supplied any acceptance evidence.

## Commands actually run

From `/Users/cztamas/dev/personal/qwacamole`:

```sh
# Downloaded Go 1.27.1 arm64; verified SHA-256 before extracting under .tools.
# Installed pinned xcaddy 0.4.5 and Pebble 2.8.0 with project-local GOPATH/GOBIN.
# Built the custom Caddy 2.10.2 binary using xcaddy --with this repository.
make test
make browser-test > .local/evidence/final-integration-console.log \
  2> .local/evidence/final-integration-stderr.log
```

Earlier exploratory runs used `bin/qwacamole pki`, `fixture`, `browser`, `demo`,
`lifecycle`, `refresh-test`, and `failure-test`, and a real `caddy stop` / `caddy
run` restart. Those raw artifacts remain in `.local/evidence/`, `.local/profiles/`,
and `.local/initial-state/`. The final reproducible runner generated fresh state
at `.local/integration-20261004-185437/`; keys, profiles, and logs are ignored by
Git. Config examples in `config/` were derived from the actually executed
configuration; `{STATE}` denotes the absolute generated state directory.

## Final acceptance results

| Requirement | Observed result |
|---|---|
| Fresh setup / xcaddy / test PKI / Pebble / Caddy / isolated browser | Passed through `make browser-test` |
| Real ACME issuance and native initial binding verification | Passed |
| Automatic renewal and native replacement verification, unchanged profile | Passed |
| Existing connection during rotation, overlap coverage | Passed |
| QWAC trust removed, TLS preserved | Passed |
| TLS trust removed, QWAC preserved | Passed; `ERR_CERT_AUTHORITY_INVALID` |
| Corrupted signature | Passed; native `binding signature invalid` |
| Wrong bound TLS certificate | Passed; native `TLS cert not bound` |
| Wrong QWAC hostname | Passed; native QWAC `net_error: -200` |
| Expired binding header | **Failed**; native verifier accepts it |
| Incorrect QWAC profile | Passed; native QWAC `net_error: -207` |
| Ordinary TLS hostname / expiry negatives | Passed; exact intended errors |
| Signing/publication failures during automatic renewal | Passed; old cert stayed live, automatic retry recovered |
| Caddy restart / independent binding refresh | Passed |
| Go concurrency, recovery, profile/hash/expiry, and fallback tests | Passed with race detector |

The full native negative suite has 10 passing cases and one failing case.
Accordingly, `make browser-test` returns nonzero. A prior run had an additional
truncated-NetLog capture; that result was rejected as evidence. Graceful browser
shutdown now waits for process exit before parsing the log; the final signature
case passed.

## Browser and trust

Positive/lifecycle evidence: **Google Chrome 154.0.8037.95**, macOS arm64,
genuine installed PKIMetadata **1793**. Compiled root-store version **39**;
patched version **1000000** observed in native NetLog. The copied component
preserved its manifest, metadata, existing roots, and unknown protobuf bytes.
The two appended authorities had mutually exclusive TLS and EUTL trust roles.

The isolated lifecycle profile was reused unchanged after renewal and restart.
Every main navigation was fresh, after a TLS-only initialization page; CDP HTTP
caching was disabled and server responses used `no-store`. The renewed browser
run was a new process, forcing a fresh TLS handshake.

CT control without an exception failed specifically with
`ERR_CERTIFICATE_TRANSPARENCY_REQUIRED`. Positive tests used
`--disable-features=CertificateTransparencyAskBeforeEnabling`, the stock
browser's source-defined CT emergency feature switch. Its scope is all CT in
that **isolated process**, not a hostname-specific exception. TLS chain,
hostname, and expiry validation remained active, as demonstrated by independent
negative tests. No `--ignore-certificate-errors` or SPKI ignore flag was used.

## Renewal and native evidence

Final observed old TLS certificate SHA-256:

```text
0843dcd8a903e0fb58074ae5775db6031b1cea1e1650d4d2131c2fd5fefa1727
```

Replacement TLS certificate SHA-256:

```text
a1461ed8682b5cb940c2afa56354a228aaa700e77cea01a6be3da081884ca8c0
```

The rotation probe waited **68.19 seconds** after opening the old connection.
The certificates had a configured 360-second lifetime; Caddy checked every two
seconds, retained ARI, and used a 0.8 renewal-window ratio. See `caddy.log` for
exact issuance, ARI selection, renewal, and cache-replacement timestamps. No
manual binding generation, server restart, or profile patch caused renewal.

The existing HTTP/1 connection continued reporting the old certificate while a
fresh handshake reported the replacement. The same published binding covered
both, and old immutable documents stayed available. `rotation.json` and the
before/after manifest snapshots contain the identifiers and binding URL.

Relevant final NetLog end-event excerpts, captured by the **native verifier**:

```json
{"type":"CERT_VERIFY_PROC_CHROME_ROOT_STORE_VERSION",
 "params":{"version_major":1000000}}
{"type":"CERT_VERIFY_PROC_2QWAC_BINDING","phase":2,
 "params":{"is_valid_2qwac_binding":true}}
```

`initial-result.json` binds the old TLS ID above; `renewed-result.json` binds the
replacement ID; `restarted-result.json` again verifies that replacement. All
verified QWAC chains end at the injected root with SHA-256:

```text
97660a796ecfb5d85dffa916c3c59598d5eff93e8bb182fbb4e51381fbac23ad
```

The issuer intermediate and leaf IDs in those verified chains are:

```text
afc19bffdd8e295a8f507f16ae91c4ed6c462a3a9ab3aec04cf7e276bd799df6
1df61814a22fada0fbff6f8561c9a2619d865ebe62136e01db3bff45eb66a7f0
```

Independent refresh changed the binding URL in **25.05 seconds** while retaining
exactly the same TLS ID set. `refresh-test.json` records both URLs. Restart
records contain distinct Caddy process IDs and an unchanged Pebble process ID.
Signing and publication failure checks observed actual renewal errors, verified
fresh trusted handshakes remained on the old certificate, and checked that its
binding stayed valid during retry. `failure-test.json` preserves the exact
error-log excerpts and recovery certificate ID; `failure-recovery-result.json`
contains native browser acceptance after recovery.

## Remaining browser incompatibility and attempted alternatives

Chrome 154 accepts a correctly signed binding whose `exp` is already in the past
while HTTPS and its QWAC chain remain valid. The Go verifier correctly rejects
that same validity state. The browser assertion deliberately reports:

```text
BROWSER INCOMPATIBILITY: native verifier accepted expired exp header
```

This matches the [exact browser source](https://chromium.googlesource.com/chromium/src/+/154.0.8037.95/net/cert/two_qwac.cc):
the parser removes `exp` and does not enforce it. Current upstream HEAD and the
inspected 157.0.8083.0 source still use that behavior.

An official stock **Chrome for Testing Canary 157.0.8083.0** was downloaded from
the Chrome for Testing distribution. Its component installer reported version
1793 ready, but observed verification stayed at compiled root-store version 40;
the existing-profile retry also failed to adopt the patched root store.

An official branded **Chrome Canary 157.0.8085.0** was extracted into `.tools/`,
without installation into `/Applications`. It adopted version 1000000 but its
TLS path builder reported `No matching issuer found` for the injected test TLS
root and failed with `ERR_CERT_AUTHORITY_INVALID`. Thus it did **not** furnish
an expired-binding result; that unrelated failure was not counted as success.
Raw version, component, and native logs are preserved under
`.local/evidence/canary-*` and `.local/evidence/branded-canary-*`.

The smallest dependency change needed for full acceptance is a stock browser
and compatible PKI Metadata pair that both support the isolated trust setup and
natively enforce the Annex B expiry rule. Changing server behavior, counting the
Go verifier, or substituting a different native failure would not satisfy the
requested expired-binding test. No such compatible pair was established.

All test services were supervised project resources. Integration cleanup stops
only its children; generated evidence and private keys remain local. Initial
Caddy autosave state accidentally used its default user-level directory; that
project-specific file was verified, preserved locally, and removed. Final runs
isolate all Caddy XDG state in their generated directory.

## Final diagnostic smoke check

After the full integration run, `make build` and `make test` passed again. The
diagnostic page was checked with:

```sh
bin/qwacamole browser -state .local -name diagnostic-page-final -screenshot \
  -ct-exception -tls-root .local/pki/pebble-root.pem
```

The native result again contained `is_valid_2qwac_binding: true` and the screenshot
confirmed the synthetic owner, certificate identifiers, and wrapping binding URL.
Artifacts are `.local/evidence/diagnostic-page-final-result.json` and
`.local/evidence/diagnostic-page-final-page.png`. Public issuer certificate/CRL
responses matched generated bytes; a private-key URL returned 404. The foreground
demo was then stopped with Ctrl-C, preserving all evidence. `git diff --check`
passed, and generated keys, browser profiles, downloaded tools, and logs are
excluded by Git.

## Follow-up failure-stage audit

The original failure markers both stopped before signing. The publication
injection now occurs after the immutable candidate is signed, verified, and
durably written, but before `current.json` switches. The race test proves the
old in-memory and durable manifests remain unchanged and restart recovery ignores
the orphan candidate. Signing injection still occurs before signing. Refresh of
the already active certificate remains available under either marker.

The strengthened implementation was rebuilt and tested with `make build` and
`make test`, then exercised with fresh state:

```sh
bin/qwacamole demo -state .local/failure-stage-audit
bin/qwacamole failure-test -state .local/failure-stage-audit
bin/qwacamole browser -state .local/failure-stage-audit \
  -profile .local/failure-stage-audit/profiles/failure-stage \
  -name during-failure -ct-exception \
  -tls-root .local/failure-stage-audit/pki/pebble-root.pem
bin/qwacamole browser -state .local/failure-stage-audit \
  -profile .local/failure-stage-audit/profiles/failure-stage -reuse-profile \
  -name after-failure -ct-exception \
  -tls-root .local/failure-stage-audit/pki/pebble-root.pem
```

Both actual automatic renewal failures preserved trusted HTTPS and a valid old
binding. CertMagic retried after 60 and 120 seconds and recovered automatically.
The previous TLS ID was
`b73bc3c70c7cd949041965430fce3c594ecb3f9b1be34cc61a0ee1bbddcaea33`;
the recovery TLS ID was
`ced94a971292bf70984ad4e1de13e95da8ec1f3f868d1c5e57d5d0cf8c9335c1`.
Stock Chrome natively accepted the old deployment during the failure window and
the replacement after recovery, using the same unchanged browser profile. Both
browser commands returned success and recorded the verified injected QWAC chain
and `is_valid_2qwac_binding: true`.
Evidence is under `.local/failure-stage-audit/evidence/`; race/build output is
under `.local/evidence/failure-stage-*`.

Current Chromium HEAD was fetched again to
`.local/upstream/expiry-audit-head.cc`; lines 183–187 still explicitly remove and
ignore `exp`. This confirms the same external blocker remains. The full acceptance
suite remains failing under its original strict expiry requirement. The owner's
subsequent acceptance permits completion of the demonstration with this recorded
browser implementation gap.

## README walkthrough validation after acceptance

After the owner accepted the Chromium expiry gap, the README was expanded with
first-time prerequisites, fresh-state setup, each manual validation step, expected
native results, trust controls, troubleshooting, and scoped cleanup. `make build`
and `make test` passed again; their output is preserved in
`.local/evidence/readme-build.log` and `readme-go-test.log`.

The documented manual commands were run with fresh `.local/manual` state and
`Example Synthetic Owner TEST`, HU, Budapest, `NTRHU-TEST-0001`. Trusted `curl`
without `-k` returned HTTPS 200 and the relative binding Link. Stock Chrome
154.0.8037.95 accepted the initial binding and again accepted it after automatic
renewal and Caddy-only restart using the unchanged `profiles/lifecycle` profile.
The held connection retained TLS ID
`2a410349b8e100b1537276eaf9d7e7081e4906c0cc6ba3df416dc29196dc26d7`;
the fresh handshake observed
`fd013f839d032349fbfca81833dae37855da981a04150159e6eb8c4cc4e39960`.
Rotation was observed in 22.07 seconds from the lifecycle observer's start;
overlapping coverage passed. Independent refresh passed in 25.05 seconds with
the exact TLS ID set unchanged.

The README's individual negative-control commands also passed: removing QWAC
trust preserved HTTPS and caused native QWAC authority rejection; removing TLS
trust caused `ERR_CERT_AUTHORITY_INVALID`. Evidence is in
`.local/manual/evidence/`. The demo and fixture were stopped afterward. Failure
recovery and the entire native suite remain covered by the preceding full and
failure-stage runs; the accepted expiry gap remains a visible failing assertion.

## Visible Chrome quickstart and expired-connection regression

The visible launcher was exercised with configurable signed owner data, fresh
local state, real Pebble HTTP-01 issuance, and isolated Chrome trust. The user
inspected the native Security panel and supplied the unmodified captures in
`docs/images/`: `EU Qualified Status`, `Issued to: Qwacamole Browser Demo TEST
[HU]`, and a separate QWAC certificate viewer. The viewer's leaf SHA-256,
`0dc96de753f6ac0530aff2c3ed22a9b85ea681aaf9f5eccd8817c92a3f722a85`,
matches `.local/browser-demo-3976375600/pki/qwac-leaf.pem` exactly. Computer Use
permissions were unavailable; this native UI evidence was supplied by the user,
not observed through automated UI access. The launcher stayed open beyond its
startup timeout and exited successfully on Ctrl-C with native NetLog acceptance.

The user's subsequent `Giant Hairy Horses Zrt` session exposed a separate bug,
unrelated to Chrome's ignored binding `exp` header. The browser retained the
TLS leaf with ID
`5c2f22f3b0592f252f215b6d6b61d4b4323ce47a1e9dc33bc273fbb6fd3bb55b`,
issued at 19:13:36 UTC and expiring at 19:19:35 UTC. Caddy's log contains repeated
successful automatic renewals and cache replacements through 19:22:01 UTC.
Nevertheless, refresh pruned that original leaf from the advertised binding
once it expired. The saved interactive NetLog contains five successful native
binding verifications followed by two exact `TLS cert not bound` errors.
Evidence is in `.local/browser-demo-2850885269/evidence/`. The screenshots of the
missing status and old TLS viewer therefore identify a real overlap-retention
bug; successful fresh-browser renewal checks did not cover this connection.

The manager now retains all previously published TLS certificates for the life
of the short-lived local state, including across refresh and restart. It still
caps binding expiry by the longest-lived TLS leaf and QWAC leaf, as defined in
[ETSI Annex B](https://www.etsi.org/deliver/etsi_ts/119400_119499/11941105/02.01.01_60/ts_11941105v020101p.pdf).
An all-expired TLS set cannot extend binding lifetime, and new TLS connections
still enforce certificate expiry. Positive browser assertions also reject any
later native binding failure instead of letting an earlier success hide it.

Commands executed for the fix:

```sh
make build
make test
make browser-demo DEMO_ARGS='-organization "Example Synthetic Owner TEST" -country HU -locality Budapest -registration NTRHU-TEST-0001'
bin/qwacamole browser -state .local/browser-demo-4114156420 \
  -name through-expiry-refresh -ct-exception \
  -tls-root .local/browser-demo-4114156420/pki/pebble-root.pem \
  -through-tls-expiry
bin/qwacamole fixture -state .local/browser-demo-4114156420
bin/qwacamole browser-suite -state .local/browser-demo-4114156420
```

Console output was redirected to evidence files. The exact Make quickstart
opened visible Chrome successfully with the selected signed owner. A separate
second launch failed immediately on occupied TCP 8443 before creating state;
real socket unit tests also verify port reservations are released on success
and failure. Go race tests passed, including expired-leaf publication/refresh/
restart and later-failure assertion regressions. The native suite passed all ten
other cases; only the previously accepted expired-binding `exp` case failed.
Raw evidence is in `.local/browser-demo-4114156420/evidence/` and
`.local/evidence/expiry-fix-*.log`. The through-expiry check keeps one browser
connection active and waits through a full 45-second binding lifetime after its
original leaf expires; it explicitly fails if Chrome reconnects.

The complete regression passed. Chrome retained TLS ID
`67a26055a1951cd741ef61535a34032460a1892281922b302b3b52312593475a`
before and after its 19:42:30 UTC expiry. The final native binding success was
recorded at 19:43:15.024 UTC, 45 seconds after expiry; the browser certificate
comparison completed at 19:43:17 UTC. All 19 native binding completions were
successful, with no binding errors. Caddy renewed repeatedly without a restart
or profile modification. This specifically covers the previously failing
existing connection, rather than substituting a new connection with a new leaf.

A subsequent fresh-browser check (`-name fresh-after-expiry`, same state and
Pebble root) also passed native verification with replacement TLS ID
`a36838473a6e3a39ef9b9bc0061c3b8a1830776ef7fb7cbf8e4d16a4a9e2045a`.
The owned visible demo then printed `PASS native Chrome QWAC verification` on
Ctrl-C, Caddy shut down normally, and its HTTPS listener was gone. The fixture
and both headless checks were stopped; generated state and raw evidence were
preserved. The outer Make invocation reported its expected interrupted status.


## Firefox extension, 5 October 2026

Stock **Firefox 157.0**, build `20260924084938`, was tested with geckodriver
0.37.1, an isolated NSS profile and real TLS verification
(`acceptInsecureCerts=false`). See [Firefox research and executed checks](firefox.md)
for pinned upstream findings, commands, certificate IDs, all native rejection
controls and manual reproduction. The user supplied
[the native qualified panel screenshot](images/firefox-qualified-status.png)
and confirmed that the selected organization and qualified label are visible.

Executed `make firefox-demo` with `Firefox Synthetic Owner TEST`, HU, Budapest,
NTRHU-TEST-0001. The real native panel returned `secure-etsi`, visible qualified
text, matching signed organization/country, and the generated QWAC fingerprint.
Fresh native checks before and after automatic TLS renewal used the exact same
NSS profile without patching trust; distinct TLS fingerprints were observed,
with unchanged accepted QWAC identity. `make firefox-test` passed all **eleven**
controls, requiring exact native verifier messages or TLS error codes. Firefox
rejects expired bindings; the accepted Chromium `exp` gap remains Chrome-specific.
The Go race suite passed with strict and Firefox-compatible persistence tests.

This uses Mozilla's compiled public test QWAC authority, enabled only in the
isolated profile. Its already-public fixture key is intentionally versioned with
pinned attribution and MPL notice. Generated intermediate/leaf keys remain ignored.
Firefox's explicit compatibility binding omits `crit` because the stock parser
rejects it; this does not claim the strict JAdES profile used by Chrome. The
profile selects the stock legacy identity panel because the tested new Trust
Panel has a failing subview call. Neither verifier nor native UI is modified.
Raw evidence remains in `.local/browser-demo-1558749535/` and
`.local/firefox-research/`.
