# Operator installation and setup plan — 2026-10-12

Scope: implement install → configure → start → verify, retaining strict explicit
authority validation, single-host ownership, binding-first activation, durable
recovery and the product/maintainer publication boundary. PR #8 is merged; work
is based on main 49b5659 in a follow-up branch. No publication, tags, website
deployment, account, visibility or DNS changes are authorized.

## Implementation and engineering review

1. Correct and parser-test the Firefox bare Caddyfile switch; JSON stays boolean.
2. Add a stdlib-only shared verification report using the existing binding
   verifier and normal HTTPS validation; expose human and JSON output in the
   agent and packaged Caddy, retaining existing automation diagnostics.
3. Add Ubuntu 24.04 amd64 fresh setup to the production agent: explicit operator
   inputs, existing validators/rendering/control path, deterministic files,
   preflight before writes, dry run, generated dedicated services and hook.
   Repeated identical setup is read-only; changed files/inputs require explicit
   maintenance/recovery, never implicit replacement. Symlinks, unsafe owners,
   shared masters and occupied listeners are rejected. RuntimeDirectory handles
   private control state at boot. Setup does not start or stop services.
4. Package the agent's three existing targets plus Linux amd64 Caddy in the
   existing exact-source candidate process. Record binary/dependency/source
   identities, include templates and installation information, retain strict
   archive allowlists and publication rebuild/digest/environment controls.
   Provide authenticated artifact installation and future checksum-verified
   public installation without fabricating current release URLs.
5. Use one minimal Caddyfile with tested defaults and an explicit service
   identity. Infer only an unambiguous literal site hostname if the Caddy helper
   API supports it; otherwise retain the explicit choice. Never infer authority.
6. Rewrite canonical quickstarts and linked install/reference material around
   implemented commands, keeping advanced installs and maintenance separate.

Risks to resolve: setup must reject conflicts before mutation; partial writes
must be recoverable without changing existing state; systemd and Certbot must
use the same owned deployment/hook, and package identity verification must cover
Caddy's external dependencies/notices as well as module exclusion. No new
deployment engine or runtime dependency on development tooling is acceptable.

## Acceptance ledger

- [ ] Firefox parser examples, strict default and no UA-selected compatibility.
- [ ] Agent three targets, Linux amd64 Caddy, exact identities and notices.
- [ ] Clean extracted checksum-verified installs, executable modes/version,
      corrupt-package rejection, integration startup without development tools.
- [ ] Ubuntu setup input/capability/conflict/ownership failures before writes.
- [ ] Dry run, identical repeat, preservation of state/edited files/credentials.
- [ ] Real HTTPS/discovery/signer-authority/current TLS coverage/readiness report.
- [ ] Dedicated service start/restart and runtime recreation after host reboot.
- [ ] Real Certbot issue/renew installed hook; replacement/recovery invariants.
- [ ] Workspace and independently pinned four-module races/boundaries,
      installed binaries and real integrations; sequential native browser checks
      preserving the exact known Chromium expired-binding failure.
- [ ] Website checks, publication map/search, desktop/mobile quickstart review.
- [ ] Dated evidence of executed commands/platforms/artifacts and unavailable
      checks, final diff review, push and focused follow-up PR against main.

Evidence is gathered anew; historical runs are context only. Docker is available
on local ARM64; amd64 emulation and systemd/reboot feasibility remain to verify.
