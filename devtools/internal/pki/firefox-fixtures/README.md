# Mozilla's public test authority

These are publicly distributed test fixtures. The private key is **already
public**, and is not a generated demo credential or production secret.
Fresh intermediate and leaf keys are generated into ignored local state.

`test-ca.pem` is copied unchanged from the installed Firefox 157 release source:
https://hg.mozilla.org/releases/mozilla-release/raw-file/8eb25af4acf031ab1e06abf1a912275083c820ed/security/manager/ssl/qwac_trust_anchors/test_trust_anchors/test-ca.pem

Its SHA-256 fingerprint is
`0ada38b1dc50cc9d3b092b9ae8452bbf2448cb49a66c054c9eac625dd617d0ec`.
It expires on 5 February 2027. The generator checks validity before issuing.

`public-test-ca-key.pem` is the PKCS#1 encoding of the `RSAKey.sharedRSA_*`
constants openly published in Mozilla's `pykey.py`, pinned at:
https://raw.githubusercontent.com/mozilla-firefox/firefox/a39d9f886066d353c0c4efab177ffd862f03e56c/security/manager/tools/pykey.py

Both public keys match (DER SPKI SHA-256
`54222598f33d36480542daece0e6b94de15c0eee8c5914ca48d75e3c484e803f`).
The source is available under Mozilla Public License 2.0:
https://www.mozilla.org/MPL/2.0/

The demo enables Firefox's built-in QWAC test-anchor list only in a fresh,
isolated profile. This authority is never imported as an NSS TLS trust anchor.
