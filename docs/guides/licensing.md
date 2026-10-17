# Licence and attribution

Qwacamole's original code and documentation use the standard **MIT License**,
with `Copyright (c) 2026 Czank Tamás`. Read the actual [LICENSE](../../LICENSE)
and [NOTICE](../../NOTICE). Preserve the copyright and permission notices when
redistributing the software. Agent, Caddy and devtools modules carry identical
copies. Third-party material retains its own licences.

## Third-party material

| Material | Licence and distribution scope |
| --- | --- |
| Go runtime and standard library | BSD-style Go LICENSE and PATENTS, plus applicable vendored notices. Retain the notices supplied with the toolchain or linked distribution. |
| Caddy / CertMagic and their dependencies | Apache-2.0 for those projects; the separate plugin-included Caddy candidate package bundles their linked code and retains actual dependency licences/notices in `DEPENDENCY-NOTICES.txt`, plus Go notices. Public releases/downloads are not yet available. |
| Nginx / Certbot / Pebble / browser tools | Separately installed prerequisites with their own licences; not bundled in the production agent. |
| Mozilla public test authority and test key | MPL-2.0 covered devtools fixture. Test use only; never use it for production identity. |

The Mozilla fixture's public key is intentionally publicly distributed test
material. Its [fixture notice](../../devtools/internal/pki/firefox-fixtures/NOTICE)
and [MPL terms](../../devtools/internal/pki/firefox-fixtures/MPL-2.0.txt) remain
beside it. Do not relabel that covered material MIT. The fixture root expires
5 February 2027.

A licence grants permissions to use the code. It does not establish a QWAC's
legal qualification, browser trust, a production support promise or public
repository access. The source repository is public. Retained GitHub Actions
candidates require GitHub authentication; public release tags and download
assets are unavailable until publication.
