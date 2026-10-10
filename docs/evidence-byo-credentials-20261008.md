# Supplied 2-QWAC credential workflow acceptance, 8 October 2026

Starting `main`: `502ff74`. The [assessment and engineering plan](byo-credentials-plan.md)
was recorded before implementation. Final reusable core is
`4b3cd1c4a5f184b4c0efd1011da87839d6c59e6f`; production agent/Caddy source is
`02f7569a230a5cf9d39f81b440dcc819281b4740`. The acceptance harness, tests,
documentation and real dependency pins are at
`2629d41abe537feccbde4170d677dd734a39d875`. Subsequent evidence changes are
documentation only. No merge, release, tag, visibility or licensing change was made.

## Delivered behavior

The existing production CLI now provides key generation, verified PKCS#10 CSR
creation, public credential inspection and offline import into a new private
directory. Core owns reusable preparation and supplied-credential validation;
agent and Caddy share it. The agent accepts a root omitted from the supplied
chain, matching Caddy. Root selection stays explicit and separate from import.
The [operator guide](guides/byo-credentials.md) covers subject fields, local key handling,
trust, initial configuration, stable paths, replacement and rejected-update recovery.

Strict checks cover RSA matching/minimum size, exact QNCP-w-gen and binding-only
EKU, signing key usage, required QC statements, malformed/duplicate/mixed types,
hostnames, dates, ordered signatures and explicit-root path verification. CA EKU
constraints are checked explicitly because Go's Any usage is needed for the
custom leaf EKU. Ordinary TLS QWACs and seal/PSD2 substitutes are rejected.

Caddy's serving fallback preserves the old activated credential after failed
replacement publication only while the committed binding is valid and covers
every stored TLS leaf. New TLS issuance still requires publication. Existing
agent/store transactions, deployment journal, gate and retained TLS history are
preserved. Same-key/new-key replacement and restart are tested for both consumers.

Local profile/chain validation does not independently establish EU trusted-list
qualification, revocation or native browser trust. Production has no trusted-list
retrieval service to substitute for the existing native browser checks. Synthetic
authorities are used only by explicit development tests. No commercial issuer,
account, remote enrollment, HSM or automatic trust expansion is required or added.

## Environment and installation

macOS arm64, Go 1.27.1 with `GOEXPERIMENT=nojsonv2`, Caddy 2.10.2, xcaddy 0.4.5,
Nginx 1.30.5, Certbot 5.8.0 and Pebble 2.8.0. Stock Chrome 154.0.8037.98 uses
genuine PKIMetadata 1798; Firefox 157.0.1 uses geckodriver 0.37.1 and NSS certutil.
Browser profiles/state are fresh and isolated. The existing CT-only test exception
and Firefox's explicit compatibility/test-authority mode remain unchanged; no
certificate-error bypass or normal-profile/system trust change is used.

The Nginx metadata harness initially could not bind Linux's unassigned `127.0.0.2`
on macOS. It now selects an already assigned distinct IPv4 loopback alias (this
host has `127.0.2.2`), preserving the assertion that browser and agent-hop addresses
differ. It never changes interfaces or falls back to `127.0.0.1`. Linux behavior
remains unchanged; the prerequisite is documented in CONTRIBUTING.md.

Both production integrations were independently installed from immutable
`02f7569a230a5cf9d39f81b440dcc819281b4740` in fresh module caches, outside the
workspace, with `GOWORK=off` and no replacements. Metadata records core
`v0.0.0-20261008191916-4b3cd1c4a5f1`, and agent/Caddy
`v0.0.0-20261008191952-02f7569a230a`. Production inventories contain no
development/browser/PKI dependencies. Retrieval is authenticated while private;
this does not claim anonymous installation. Raw records are in
`.local/evidence/remote-agent/` and `.local/evidence/remote-install/`.

The remotely installed agent generated a dedicated 3072-bit key and a CSR with
organizationIdentifier and two DNS SANs. OpenSSL independently reports
`Certificate request self-signature verify OK`; requested KU, EKU and policy
extensions are present. The installed CLI also inspected/imported synthetic
issued credentials. Actual key/import modes are 0600/0700, respectively. Unit
tests additionally issue a synthetic credential from the generated CSR and
exercise the complete inspect/import path, source preservation, rejected import,
missing trust and refused overwrites. [Public CLI summary](evidence/byo-credentials-20261008/cli.json).

## Checks and native demonstrations

`make build agent-build plugin-build`, `make test`,
`scripts/check-independent-modules.sh` and `git diff --check` pass. All four
modules run race tests; independent checks use real pins without go.work or
replacements. Plugin registrations/build metadata and the core/Caddy/agent
dependency boundaries pass. Focused synthetic tests include key mismatch, weak
key, unsupported/malformed key encoding, wrong policy/EKU/KU, missing/malformed
statements, conflicting web/seal type, hostname mismatch, invalid dates, missing
intermediates, untrusted roots and issuer EKU constraints. Replacement failures
preserve the old public document and a usable old signer; restart preserves
signer and retained TLS coverage. Raw required-check output is
`.local/evidence/byo-required-checks.log` and
`.local/evidence/byo-independent-modules.log`.

Hosted [CI run 37831444995](https://github.com/cztamas/qwacamole/actions/runs/37831444995)
passes at implementation SHA `2629d41abe537feccbde4170d677dd734a39d875`:
workspace/independent races, boundaries, production builds, default/nondefault
Nginx lifecycle and faults, immutable merged-main upgrade with interruption and
repeat execution, and real Caddy lifecycle. No check was skipped or weakened.
[Step results](evidence/byo-credentials-20261008/hosted-ci.json) preserve exact
head identity and all 21 step outcomes.

Fresh Caddy Chromium acceptance exits **1**, with **10/11** fixtures passing.
Only `expired-binding` fails: the native verifier accepts expired `exp`.
This reproduces the documented browser limitation; the failing assertion remains
unchanged. Initial/renewed binding, independent refresh, restart and native
failure recovery pass. The same original HTTP/2 TLS fingerprint is checked at
19:28:34 UTC, beyond its 19:27:47 expiry and a complete 45-second binding lifetime.
Native results contain `CERT_VERIFY_PROC_2QWAC_BINDING` with
`is_valid_2qwac_binding: true` and isolated root-store version 1000000. No other
browser failure is accepted. [Chromium summary](evidence/byo-credentials-20261008/caddy-chrome.json).

Caddy Firefox exits **0** with all **11/11** fixtures passing, including expired
binding rejection. Six native phases cover initial/direct and cached access,
renewal, credential replacement, restart and automatic failure recovery. Every
positive phase reports `secure-etsi`, visible qualification,
`certificate_overridden: false` and `accept_insecure_certs: false`. Profile/trust
remain unchanged; signing/publication failures preserve old HTTPS and binding.
[Firefox summary](evidence/byo-credentials-20261008/caddy-firefox.json).

The remotely installed production agent passes Nginx demonstrations in **both**
Chromium and Firefox, **22 checks each**. Each browser verifies initial, renewed
and replacement identities through unchanged cached discovery, with identical
trust digests across the three native phases. Chromium records native binding
acceptance and root-store version 1000000; Firefox reports visible native
qualification without certificate overrides. Both preserve the original active
TLS stream across renewal, independent refresh, cache behavior, authenticated
collection, unavailability rejection and agent restart reconciliation.
[Nginx Chromium](evidence/byo-credentials-20261008/nginx-chrome.json) and
[Nginx Firefox](evidence/byo-credentials-20261008/nginx-firefox.json).

The fresh remotely installed Caddy build passes **22/22** runtime checks,
including same-path credential replacement, rejected reload preservation,
restart/history recovery and collection. The remotely installed agent passes
**32/32** real Nginx lifecycle/fault checks, including signing/document/manifest
failures, reload failure and zero-exit nonactivation rollback, delayed response
and WebSocket rejection at actual signed expiry, repair/reopening and capacity
rejection without pruning retained coverage.
[Caddy runtime](evidence/byo-credentials-20261008/caddy-runtime.json) and
[Nginx runtime](evidence/byo-credentials-20261008/nginx-runtime.json).

The [command ledger](evidence/byo-credentials-20261008/commands.json) records
browser and remote-runtime arguments, exact source identities, exits and raw log
paths. All five sequential Firefox/Nginx/remote-runtime commands exit zero;
Caddy Chromium's separate exit one is explained above. Only public summaries are
committed. Generated private keys, browser profiles, caches and raw NetLogs remain
under ignored local state. No commercial credential or external issuer account
was used.
