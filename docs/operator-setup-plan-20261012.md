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

- [x] Firefox parser examples, strict default and no UA-selected compatibility.
- [x] Agent three targets, Linux amd64 Caddy, exact identities and notices.
- [x] Clean extracted checksum-verified installs, executable modes/version,
      corrupt-package rejection, integration startup without development tools.
- [x] Ubuntu setup input/capability/conflict/ownership failures before writes.
- [x] Dry run, identical repeat, preservation of state/edited files/credentials.
- [x] Real HTTPS/discovery/signer-authority/current TLS coverage/readiness report.
- [x] Dedicated service start/restart and runtime recreation after host reboot.
- [x] Real Certbot issue/renew installed hook; replacement/recovery invariants.
- [x] Workspace and independently pinned four-module races/boundaries,
      installed binaries and real integrations; sequential native browser checks
      preserving the exact known Chromium expired-binding failure.
- [x] Website checks, publication map/search, desktop/mobile quickstart review.
- [x] Dated evidence of executed commands/platforms/artifacts and unavailable
      checks, final diff review, push and focused follow-up PR against main.

## Final implementation and evidence

Follow-up PR: https://github.com/cztamas/qwacamole/pull/9.
Final production source: c13676e0805fae7389b1c3de32f91cb6521c9c3b.
Real core pin: v0.0.0-20261012130422-b3fb51784945.

See [dated acceptance evidence](evidence-operator-setup-20261012.md) for exact
package/workflow identities, commands, clean Ubuntu container and systemd guest
execution, private runtime recreation after full host reboots, real Certbot 2.9/5.8
issue/renew hooks, same-authority replacement and preserved state, thirteen setup
negatives, four-module races/boundaries and installed remote integrations.

Both quickstarts use four primary steps, with optional build/JSON/maintenance
work linked separately. Website publication/search boundaries, twelve browser
tests and desktop/mobile production review pass. Strict bindings remain default;
Firefox's bare switch and JSON boolean are parser-tested. Native Caddy Chrome's
exact known expired-binding failure remains explicit; Firefox and both Nginx
native lifecycle runs pass. No assertions were weakened.

Actual acceptance exposed and corrected worker ownership, deterministic build
inventory, complete retained-TLS verification and the agent's canonical binding
subdirectory. Identical setup preserves state; updates require explicit maintenance.
Final diff review preserves module boundaries, explicit authority selection,
binding-first publication and complete old/new failure recovery. No release,
website deployment, merge, visibility or external account/DNS write was performed.
