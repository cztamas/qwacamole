# PR review follow-up — 7 October 2026

Reviewed [PR #1](https://github.com/cztamas/qwacamole/pull/1), whose findings
refer to `2dc28d945b12ab1b30806626ae7ef292be889693`. The relevant code fix is
`f6261bde56fc8a536c8f8340be8e540915b2e46e` on the existing feature branch.
No merge, release/tag, privacy or license change was made.

## Findings and disposition

- [Wildcard policy bypass](https://github.com/cztamas/qwacamole/pull/1#discussion_r4199044209)
  is relevant and reproduced. With an earlier ordinary `*.example.com` policy
  and a later exact QWAC policy, issuer-only validation incorrectly succeeded.
  Both issuer and middleware now use pinned Caddy's own
  `certmagic.MatchWildcard` semantics when selecting the first applicable policy.
  Matching wildcard policies reject rather than bypass the issuance gate;
  unrelated wildcards remain valid. This also avoids middleware incorrectly
  treating `*.com` as matching `qwac.example.com`.
- [Stale bundle at replacement](https://github.com/cztamas/qwacamole/pull/1#discussion_r4199044199)
  does not reproduce with the active configuration. The smoke retains the
  adapted **Pebble** config, whose `chain_files` explicitly name the standalone
  `qwac-leaf.pem` and `qwac-intermediate.pem`. Replacement changes that leaf and
  its matching key. `QWAC_CHAIN=bundle.pem` is used by the separate proxy/static
  examples, and the bundle is rebuilt before the later static-example reload.
  The unchanged baseline completed replacement, rejected-reload recovery,
  restart and static serving, exit **0**. No bundle behavior change was needed.

## Regression and runtime verification

The actual plugin-only binary is exercised by four added portable smoke cases:
matching earlier wildcard with issuer only, matching earlier wildcard with paired
middleware, unrelated wildcard with issuer only, and a nonmatching parent
wildcard with paired middleware. Rejections must contain the issuer policy error,
so unrelated validation errors cannot satisfy those cases.

Against the old binary, the new suite failed with exit **1**:
`validation shadowing-wildcard-issuer-only wanted success=false: <nil>`.
Against the fixed binary, all four cases and the full **fourteen** smoke checks
passed, exit **0**, including real Pebble renewal, application/header/streaming,
unchanged reloads, same-authority credential replacement, rejected reloads,
exclusive ownership and restart. `make test` passed all-module race tests and
dependency boundaries; `git diff --check` passed.

Local evidence:

- `.local/pr-review-20261007/baseline/plugin-smoke-3654581230/` and `baseline-smoke.log`.
- `.local/pr-review-20261007/regression-before/plugin-smoke-3796642041/` and `regression-before.log`.
- `.local/pr-review-20261007/fixed/plugin-smoke-2320158704/` and `fixed-smoke.log`.
- `.local/pr-review-20261007/tests.log` and `plugin-build.log`.

The fix commit passed [hosted CI run 37587370609](https://github.com/cztamas/qwacamole/actions/runs/37587370609),
including the same race/boundary and portable smoke checks on Linux.
A fresh-cache authenticated remote installation of the exact fixed ref also
exited **0**, with `GOWORK=off`, no local replacements or devtools:

```sh
export GOPRIVATE=github.com/cztamas/qwacamole
export GOWORK=off GOEXPERIMENT=nojsonv2
xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@f6261bd
```

Full immutable ref: `f6261bde56fc8a536c8f8340be8e540915b2e46e`.
Go resolved plugin version `v0.0.0-20261007072646-f6261bde56fc` with the unchanged
core `v0.0.0-20261006174406-ced6241dd061`. Build metadata contains no replacements
or devtools dependency, and inventory contains only the two QWAC production
modules alongside standard Caddy modules. Go 1.27.1, xcaddy 0.4.5 and Caddy 2.10.2
pins and the JSON workaround remain unchanged. Evidence is copied to
`.local/pr-review-20261007/remote-install/`; console log is `remote-install.log`.
The installation guide now selects this corrected immutable ref.

The downloaded `bin/caddy-remote` then passed the same **fourteen** smoke checks,
exit **0**, including the new wildcard cases and real replacement/restart.
Evidence is in
`.local/pr-review-20261007/remote-runtime/plugin-smoke-3272995200/evidence/`,
with console log `.local/pr-review-20261007/remote-smoke.log`.
All owned smoke processes were cleaned up.

Native browsers were not rerun for this configuration-selection-only change.
Certificate generation, signing/publication, trust profiles and browser harnesses
are unchanged. The fresh native runs and exact accepted Chromium expiry gap
remain separately recorded in [the 6 October report](evidence-plugin-20261006.md).
