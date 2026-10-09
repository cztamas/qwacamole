# Release preparation: 0.1.0-alpha.1, 9 October 2026

This prepares a PR, not publication. Baseline: merged main
`08872250a0813f232faaca9794a9e5b481b4d19b`. The four modules and binding,
deployment and browser lifecycles remain intact. No release tags, visibility
change, Caddy registration or website work is authorized here.

## Assessment and engineering review

Core already supplies stdlib-only strict RSA credentials, explicit trust,
bindings, transaction/recovery and lifecycle policy. Caddy is independently
installable and gates issuance/response commitment. The agent already owns a
dedicated Nginx deployment and Certbot hook; devtools owns synthetic/browser
dependencies. Both integrations have recent native acceptance, including the
preserved Chromium expired-exp failure. These are inherited evidence until rerun.

Missing: config-free agent version output; deterministic production archives;
source/target/checksum manifests; retained candidate CI; fail-closed manual
publication tooling; anonymous installation verifier. README/checklist predate
the agent and have stale refs. Licence, visibility and a verified confidential
security channel remain owner decisions.

Use the existing agent CLI, scripts, CI and guides. Add stdlib build identity in
the agent command; package only that executable and legal/install documents.
Use Python stdlib for deterministic archives and verification, no new Go module
or core dependency. Separate a root release source from nested release source:
publish the root first, then update nested requirements to the real version,
verify and publish nested tags. Until then preserve actual immutable pins.

Release automation is manual and refuses missing legal/security approval,
non-main source, wrong tags/pins, incomplete checks, existing releases and changed
artifacts. Candidate CI has read-only contents permission and never publishes.
Draft licensing files confer no project licence. Avoid build timestamps and
build-path metadata; all manifests identify the actual source/toolchain.

Primary risks: fictional module versions, source/binary disagreement, packaging
development secrets, promoting unapproved licensing, bypassing native assertions,
and using authenticated installation as public evidence. Verify each directly.

## Work and completion ledger

- [ ] Config-free version identity, meaningful CLI tests, production-only build.
- [ ] Deterministic three-target archives, manifest/checksums, extracted native
  binary smoke, contents/modes/negative verification and reproducibility checks.
- [ ] Candidate retention CI and manual publication gates with negative tests.
- [ ] Current README/guides, support contract, exact multi-source tag sequence,
  source and package installs, update/rollback instructions and draft legal files.
- [ ] Tracked content/history audit; confidential reporting and owner checklist.
- [ ] Four-module races/boundaries/independence and production/plugin builds.
- [ ] Fresh-cache authenticated immutable agent/Caddy installs, metadata and real
  integration smoke using those binaries; anonymous check prepared separately.
- [ ] Sequential fresh isolated Caddy Chrome/Firefox and Nginx Chrome/Firefox
  native acceptance; only documented Chrome exp failure remains nonzero.
- [ ] Fresh dated evidence with exact source/package/tool identities.
- [ ] Final requirement-by-requirement audit, diff review and PR against main.

Owner decisions/public-only verification may remain explicitly pending. Available
engineering checks must complete before this ledger and goal can be closed.
