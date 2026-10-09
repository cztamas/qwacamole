# Licensing recommendation and attribution inventory

No project licence is selected. Recommend **Apache-2.0** for owner review: it
provides permissive copyright terms and an explicit contributor patent grant,
with redistribution/notice requirements. The proposal is not an approval or a
project licence grant. Review ownership and contribution rights before adoption.
[Apache's licence text](https://www.apache.org/licenses/LICENSE-2.0) describes those
terms. `release/LICENSE-PROPOSAL.txt` contains the full text with an unapproved
preamble; `release/NOTICE-PROPOSAL.txt` is a draft notice, not settled copyright
wording. Root LICENSE/NOTICE are intentionally absent until owner approval.

| Material | Current scope | Attribution/release action |
| --- | --- | --- |
| Project core/agent/Caddy code and docs | Owner's unlicensed private source | Owner selects licence, copyright holders and contribution treatment; add approved LICENSE/NOTICE to root and nested production module archives |
| Go runtime and standard library | Linked into agent | BSD-style Go LICENSE and PATENTS, plus vendored standard-library licences; packager copies complete texts from the exact toolchain into GO-NOTICES.txt |
| Caddy 2.10.2 / CertMagic 0.24.0 / dependencies | Installed by operator's xcaddy build | Apache-2.0 for those projects; retain their and transitive dependencies' own notices in any downstream custom distribution. This PR distributes no prebuilt custom Caddy or copied dependency source |
| Nginx / Certbot / Pebble / browser tools | External runtime or development prerequisites | Their licences remain with separately installed packages; none is bundled in agent archives |
| Mozilla test authority certificate and public test key | Devtools fixture only | MPL-2.0 covered source; retain fixture NOTICE, pinned sources and modification explanation, now with full MPL-2.0.txt. Never include in production packages |
| Generated authorities, signing keys, profiles, raw browser logs | Ignored per-run development state | Exclude from commits/releases; they are not the public Mozilla fixture |

The Mozilla certificate is copied unchanged from Firefox's pinned release source.
The fixture key is the PKCS#1 PEM encoding of already-public `RSAKey.sharedRSA_*`
constants; it is not an accidentally published private production key. The
[fixture README](../devtools/internal/pki/firefox-fixtures/README.md) records exact
source URLs, certificate/public-key fingerprints and its 5 February 2027 expiry.
Keep these notices beside the covered files and the public source available on
publication; do not relabel them Apache-2.0. Mozilla's
[MPL text](https://www.mozilla.org/MPL/2.0/) explains covered source requirements.

The four Go manifests/checksums identify fetched dependencies; the core and
production agent have no third-party Go dependencies beyond core/stdlib. Caddy
and devtools have distinct larger graphs. A future custom Caddy distribution or
bundled devtools product requires its own full transitive notice inventory; this
release does neither. The candidate manifest and binary dependency check enforce
that distinction.

Before publication, the owner must approve actual root LICENSE/NOTICE, review
third-party source notices, record the decision URL and legal-file hashes in
`release/publication.json`, and review the tracked-content/history report. The
publication script requires those exact approved files; it will not promote a
proposal automatically. Choosing Apache-2.0 must not be interpreted as legal
qualification of any QWAC or a production support promise.
