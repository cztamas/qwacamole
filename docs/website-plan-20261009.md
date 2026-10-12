# Documentation website plan — 9 October 2026

## Baseline and scope
Release-preparation PR #7 merged as e24ed28. The repository remains private, MIT
licensed, without release tags/assets; installation refs in README are real
authenticated installs. No Go code, module manifest, trust, signing, state or
runtime deployment changes are needed. Four module boundaries stay intact.

## Implementation and engineering review
- Isolated website/ uses pinned Node 24.21.0, Astro/Starlight, npm lockfile and
  credential-free static output. No adapter, Functions or backend.
- Explicit shared-document route map generates ignored Markdown for Starlight.
  Repository docs remain canonical. Parse Markdown links/references/images,
  map included documents to routes and allowlisted assets to static files;
  unrendered historical reports link to GitHub. Reject broken local sources.
- Tailored landing explains supplied credentials, ordinary TLS and integration
  choices. Shared quickstarts give installation, configuration and verification.
  Static Pagefind search, accessible responsive Starlight documentation.
- Dedicated CI filters include every shared source and asset, checks TypeScript,
  build, local links/anchors/assets and focused real-browser behavior. PR jobs
  receive no Cloudflare secrets. Main push/manual deployment is separate, gated
  by explicit variable, uses exact successful artifact and pinned Wrangler.
- Main-only guard and serialized deployment; enabled missing setup fails.
  Include commit identity in static artifact and workflow deployment summary.
  Direct Upload alone; no preview deployments, Git integration or DNS mutation.
- Maintainer guide covers account-scoped Pages token, production branch, apex
  nameservers, canonical hostname, verification, free-plan limits and rollback.

## Risks and mitigations
Private-source links require authorized access, so site states current status.
A build is not native QWAC proof: fresh module/build checks and sequential
existing native browser suites remain separate evidence. Preserve Chromium's
known expired-exp nonzero fixture result. Copy only explicit public doc assets,
never generated state/keys/logs. Do not edit historical reports to imply newer
acceptance. No deployment or domain claim without authoritative evidence.

## Acceptance
Record final source identities and checks in a dated website report, capture
landing and both quickstarts at desktop/mobile sizes, inspect navigation/search,
keyboard focus/contrast/overflow/404, run repository gates and existing Chrome/
Firefox integration demonstrations, create the second preparation PR against
main and require its website CI green before completion. No merge,
visibility/release/DNS changes.
