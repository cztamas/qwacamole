# Installable single-host Caddy integration — 6 October 2026

Starting commit: `bd1238f524653219da997a765a786da62a50c61b`.
Feature branch: `codex/installable-caddy-plugin`. No merge, release/tag publication,
visibility change, or license selection is authorized by this stage.

## Scope and review

The existing core/plugin/devtools boundaries and pre-return issuance gate stay.
The plugin currently lacks a retrievable core dependency, Caddyfile support,
explicit local QWAC trust validation, configuration-lifetime ownership, and
application-safe headers. Historical native results are in
`evidence-monorepo-20261005.md`; new checks must be recorded separately.

1. Extend core PEM loading and explicit local chain/profile/hostname checks.
   Add read-only recovery and transactional same-authority credential replacement
   preserving every published TLS ID. Publish/verify before switching material.
2. Replace process-lifetime sharing with configuration leases over one serialized
   state owner. Validate candidates without activating them; activate replacement
   only on use by the new configuration. Old leases must never roll back newer
   credentials. Reference-count worker/file-lock lifetime and test rejected loads.
3. Add a small site `qwac` Caddyfile directive wiring the sole ACME issuer and
   middleware, plus explicit issuer/middleware unmarshalling. Delegate the nested
   ACME block to Caddy's existing parser. Validate pairing, fallback and hostname
   scope early; retain JSON with documented explicit-trust migration.
4. Merge Link at response commit, preserve application Cache-Control and response
   behavior, constrain content-addressed binding endpoints and GET/HEAD, log
   actionable errors through Caddy's logger.
5. Add plugin-only adaptation/validation, application/Pebble renewal/restart/
   reload/credential replacement/negative-reload checks and portable race CI.
6. Push core changes; resolve that real commit with Go to its canonical immutable
   version. Push implemented plugin and build its exact ref outside the checkout,
   GOWORK=off, fresh module cache, no local replacements/devtools. Record inventory.
7. Run fresh Chromium then Firefox native acceptance, all lifecycle controls,
   same-profile renewal and HTTP/2 through expiry. Accept only Chromium's existing
   ignored-exp failure. Keep development trust and fixtures outside plugin.
8. Audit each goal requirement, document commands, examples, evidence and limits,
   commit/push feature branch and mark complete only when all required proof exists.

## Constraints and risks

Single host, one literal DNS hostname per protected site/state and ACME-managed
TLS. QWAC is supplied RSA material, with explicitly supplied local authority;
local validation establishes no browser trust or legal qualification. No watching,
authority migration, enrollment, distributed storage or retention redesign.
State/doc size grows with all TLS history. Reload must retain serialized state and
held connections; provisioning must not install candidate credentials. Invalid
credentials/configuration must reject before issuance or activation. Cross-process
ownership needs an OS lock, including a clear error for a live conflicting owner.
Remote install requires authorized private-repository Git authentication and scoped
GOPRIVATE. Core/plugin immutable refs are sequential because the plugin dependency
must refer to an already retrievable core commit.

## Execution and final audit

Completed on 6 October 2026; [the new evidence report](evidence-plugin-20261006.md)
contains the requirement-by-requirement audit, canonical remote versions, exact
implemented-ref installation, local/Linux/remote runtime checks and fresh native
Chrome/Firefox proof. All thirteen installed-plugin operational checks passed.
Chrome kept the exact accepted expired-exp suite failure/exit 1; its verified
HTTP/2 interval crossed leaf expiry without reconnecting. Firefox passed all
eleven controls and renewed in the same profile with unchanged trust/QWAC identity.
The user confirmed both current visible native owner/status panels. Owned demo
services were stopped and evidence preserved. No goal requirement remains pending.
Feature-branch commits were pushed; no merge, tags/releases, visibility or license
choice is part of this delivery.
