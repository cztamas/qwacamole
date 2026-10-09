# MIT licensing and attribution inventory

The owner selected the standard **MIT License** on 9 October 2026. Root
`LICENSE` contains its unmodified terms with `Copyright (c) 2026 Czank Tamás`;
identical LICENSE/NOTICE copies accompany the agent, Caddy and devtools modules.
[The standard MIT text](https://opensource.org/license/mit) is the reference.
Third-party material retains its existing licences and attribution.

License selection is separate from repository visibility and release publication.
Candidate packages include the selected LICENSE/NOTICE, but
`release/publication.json` remains `approved: false` until the remaining release
prerequisites are approved. Its legal-file hashes now identify the selected files.
Earlier candidate manifests/reports describe the former Apache proposal at older
commits; they remain historical evidence.

| Material | Current scope | Attribution/release action |
| --- | --- | --- |
| Original project core/agent/Caddy/devtools code and docs | MIT | Preserve LICENSE/copyright notice in distributions; module copies match root |
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
publication; do not relabel them MIT. Mozilla's
[MPL text](https://www.mozilla.org/MPL/2.0/) explains covered source requirements.

The four Go manifests/checksums identify fetched dependencies; the core and
production agent have no third-party Go dependencies beyond core/stdlib. Caddy
and devtools have distinct larger graphs. A future custom Caddy distribution or
bundled devtools product requires its own full transitive notice inventory; this
release does neither. The candidate manifest and binary dependency check enforce
that distinction.

Before publication, review third-party notices and the tracked-content/history
report, record the release approval URL in `release/publication.json`, and verify
its LICENSE/NOTICE hashes still match. The publication script continues to require
explicit release approval and the other security/source/CI gates. Adopting MIT
does not establish legal qualification of a QWAC or a production support promise.
