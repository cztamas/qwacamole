# Unpublished public-alpha release checklist

No tags, release, repository visibility/license change, Caddy account or package
registration has been performed by this task. This checklist prepares reviewable
owner actions; it does not claim that an alpha has been published.

## Engineering gates

- [ ] Complete the [hardening ledger](public-alpha-plan.md) against the original
  objective, with a separately dated evidence report and no missing browser gate.
- [ ] Pass all-module race tests, boundaries, portable actual-Caddy checks,
  state/config migration, failure/crash recovery, held connection and cache checks.
- [ ] Build the exact pushed immutable plugin ref outside the checkout with fresh
  module cache, `GOWORK=off`, no local replacements or devtools; verify runtime.
- [ ] Record Go 1.27.1, `GOEXPERIMENT=nojsonv2`, xcaddy 0.4.5 and Caddy 2.10.2;
  recheck toolchain compatibility rather than assuming newer versions work.
- [ ] Run fresh native Chrome then Firefox sequentially, unchanged trust across
  renewal, all existing fixtures and lifecycle controls, and Firefox cached-Link
  acceptance across binding expiry and credential replacement. Chrome must have
  only its exact documented ignored-exp suite failure, with nonzero exit retained.
- [ ] Update README/plugin guide's installation ref, resolved core/plugin
  versions, current evidence links and compatibility statements after proof.

## Owner decisions before public availability

- [ ] Choose and review a license; audit third-party notices, including Mozilla's
  public Firefox test authority/key fixture attribution. No license is inferred.
- [ ] Decide repository visibility and review committed content/history. Confirm
  generated private keys, profiles and raw browsing evidence remain excluded.
- [ ] Enable GitHub private vulnerability reporting or publish a monitored
  confidential channel; finalize the response/support policy in SECURITY.md.
- [ ] Choose alpha version numbers, release support scope and publication timing.
  Review operational capacity/downtime and browser-compatibility limits explicitly.

## Module publication sequence (owner executes)

The root and nested Caddy module have distinct tag namespaces. For example, an
owner-approved `0.1.0-alpha.1` sequence would be:

1. Finalize and push the core source commit. Create/push root tag
   **`v0.1.0-alpha.1`** on the selected core commit. Confirm Go can retrieve
   `github.com/cztamas/qwacamole@v0.1.0-alpha.1` remotely.
2. Update `caddy/go.mod` to require that real root version; use the pinned
   toolchain, tidy the nested module outside workspace replacement assumptions,
   run all checks, and push the plugin commit.
3. Create/push nested tag **`caddy/v0.1.0-alpha.1`** on that plugin commit. Go
   consumers request `github.com/cztamas/qwacamole/caddy@v0.1.0-alpha.1`; the
   `caddy/` prefix belongs to the Git tag, not the requested Go version suffix.
4. After the owner makes the repository public, verify anonymous installation
   from a clean environment with no Git credentials/SSH rewrite, `GOPRIVATE`,
   `GONOPROXY`, `GONOSUMDB`, workspace or replacements. A fresh unauthenticated
   CI/container environment is preferable to a developer's credentialed shell:

   ```sh
   export GOWORK=off GOEXPERIMENT=nojsonv2
   go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
   xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@v0.1.0-alpha.1
   ./caddy list-modules
   go version -m ./caddy
   ```

   Expect only `tls.issuance.qwac` / `http.handlers.qwac` for this project, the
   intended tagged core/plugin versions and no replacement or devtools module.
   Repeat runtime smoke against this anonymously fetched binary. Private
   authenticated exact-ref checks do not prove public anonymous availability.
5. Publish the reviewed release notes and exact supported combinations. If a
   devtools release is wanted later, it needs its own `devtools/vX.Y.Z` tag;
   the installed plugin must remain independent of it.

## Caddy distribution (owner executes separately)

- [ ] Register/log in through the package-registration link in
  [Caddy's extension documentation](https://caddyserver.com/docs/extending-caddy)
  and add
  **`github.com/cztamas/qwacamole/caddy`** as the plugin package. Do not register
  the root core or development module instead.
- [ ] Review generated module documentation for `tls.issuance.qwac` and
  `http.handlers.qwac`, package/import path, field comments and alpha limitations.
- [ ] Test the actual hosted Caddy download/builder with this package/ref. The
  required Go 1.27.1 toolchain and `nojsonv2` workaround may not be available in
  that service. Manual xcaddy success does not prove hosted-builder support.
  If unsupported, document manual installation only; do not advertise the hosted
  installer until it produces and runs the correctly versioned binary.

No automatic enrollment, new server integration, HSM, distributed storage or
browser patch is part of this alpha.
