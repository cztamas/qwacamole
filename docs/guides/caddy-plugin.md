# Supplied-QWAC Caddy integration

See the [bring-your-own credential workflow](byo-credentials.md) for local key/CSR
preparation, shared validation, import and safe configuration/reload replacement.

This plugin supports **single-host, ACME-managed TLS** with supplied RSA 2-QWAC
credentials. The core signs and independently verifies a binding, durably commits
it, and only then lets the ACME issuer return a replacement TLS certificate.
It does not enroll QWACs, establish browser trust, check EU Trusted List status,
fetch authority material, or claim full production readiness/legal qualification.

## Installation from the private GitHub repository

Authorized repository access is required. Set scoped private-module discovery:

```sh
export GOPRIVATE=github.com/cztamas/qwacamole
export GOEXPERIMENT=nojsonv2
# Preserve other existing GOPRIVATE entries if you already have them.
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@9322bd566e825a2c2c645e464a630e09d7aa68d6
```

The pinned plugin resolves to `v0.0.0-20261009072701-9322bd566e82`.
After an authenticated clone, use `git checkout main` for configuration examples.
The immutable installation command above does not require a checkout.
Use Go **1.27.1** (other toolchains require separate validation), xcaddy **0.4.5**,
Caddy **2.10.2**. The `nojsonv2` workaround preserves the reflected identity of
`encoding/json.RawMessage` expected by Caddy 2.10.2. No downstream `go.work` or
`replace` is needed. This is an authenticated private installation, not anonymous
public distribution. Normal Git authentication must work noninteractively: an
HTTPS credential helper or an SSH agent/key authorized for this repository.
For an existing SSH setup, a temporary per-process rewrite is sufficient:

```sh
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=url.ssh://git@github.com/.insteadOf
export GIT_CONFIG_VALUE_0=https://github.com/
```

Do not overwrite an existing `GIT_CONFIG_*` environment; append/select your usual
Git authentication instead. No key/token belongs in a Caddyfile or repository.
The [Go private-module reference](https://go.dev/ref/mod#private-modules) explains
GOPRIVATE and credential handling. [xcaddy](https://github.com/caddyserver/xcaddy)
accepts commit/pseudo-version plugin refs. Version verification:

```sh
./caddy list-modules | grep qwac
go version -m ./caddy
```

Expect `tls.issuance.qwac` and `http.handlers.qwac`, no `qwac_demo` modules and no
`qwacamole/devtools` dependency. The core dependency is the remotely resolved
`v0.0.0-20261008203612-8e62f4b8c340` (commit
`8e62f4b8c340e572584f6d71355a8f89a7d0be42`). There are no published alpha tags or prebuilt custom Caddy packages.
Use the immutable installation command above; hosted-builder distribution is
unsupported.

For an update, build a separate binary at the selected immutable/module version,
inspect `list-modules`/`go version -m`, and validate config before replacement.
Preserve the previous binary and complete Caddy TLS storage, binding state,
credentials and configuration. Follow [operations](operations.md) for full-stop
policy/state migrations and offline retirement. Start with the same owned paths
and confirm a protected request and binding/TLS identity. Roll back only to a
compatible binary and consistent complete state; expired prior inputs are not a
usable rollback target. Do not delete retained coverage to force an upgrade, or
use an unpinned automatic module upgrade as release validation.

## Configuration

The small site-level `qwac` directive sets up **both** middleware and exactly one
QWAC ACME issuer. Shared settings are written once. It runs before redirection,
respond/proxy/file handlers, and handle/route blocks so `/bindings/*` stays
reachable. Avoid adding another `tls issuer`, global fallback issuer, on-demand
TLS, external certificate manager, manual TLS leaf, wildcard/IP host or a second
hostname to a protected site. Keep one literal lower-case DNS hostname per
site/state. Internationalized names must use their ASCII punycode spelling.
Ordinary unprotected sites may use their own Caddy TLS and application settings.

Complete examples:

- [Reverse proxy](../../config/Caddyfile.reverse-proxy).
- [File server](../../config/Caddyfile.file-server).
- [Local Pebble smoke](../../config/Caddyfile.pebble).
- [Native JSON](../../config/plugin.json.example) (replace `{STATE}` with an absolute
  directory; it is a documentation placeholder, not a Caddy runtime variable).

Example site:

```caddyfile
qwac.example.com {
    qwac {
        hostname qwac.example.com
        directory /var/lib/caddy/qwac
        chain_files /etc/caddy/qwac-chain.pem
        key_file /etc/caddy/qwac-key.pem
        trust_files /etc/caddy/qwac-authority.pem
        lifetime 1h
        acme {
            dir https://acme-v02.api.letsencrypt.org/directory
            email admin@example.com
        }
    }
    reverse_proxy 127.0.0.1:8080
}
```

Use your real hostname/owner credentials and ACME reachability; the example name
is for local testing. Examples accept optional `QWAC_SITE`, `QWAC_HOST`,
`QWAC_STATE`, `QWAC_CHAIN`, `QWAC_KEY`, `QWAC_TRUST`, `ACME_DIRECTORY`, `ACME_EMAIL`,
`UPSTREAM`, and `DOCUMENT_ROOT` environment overrides, with readable defaults in
those files. For Pebble, `STATE` is an absolute test directory prepared by the
smoke runner, and no public CA is contacted.

The nested `acme` block delegates to Caddy's pinned ACME issuer parser; ordinary
options such as directory/test directory, email, EAB, DNS challenge provider,
resolvers, timeouts, alternate challenge ports, profiles and CA roots remain
available. DNS providers require their usual additional Caddy modules. ACME
revocation and renewal-information interfaces remain supported. Any ACME retry
inside that wrapped issuer still passes the same binding gate.

Advanced explicit form: `tls { issuer qwac { <same settings and acme block> } }`
paired with `qwac_binding { <identical binding settings, no acme block> }`.
This is useful for existing layouts, but doubles the settings; validation rejects
inconsistent pairs. In manual `route` ordering, put `qwac_binding` first and
ensure it catches binding-document paths. Prefer the site-level directive.
[Caddy's extension reference](https://caddyserver.com/docs/extending-caddy/caddyfile)
describes adaptation and directive ordering.

## Credentials and state

`chain_files` is a leaf-first chain, in one PEM bundle or separate PEM files.
The last certificate is the configured QWAC authority; it may be omitted from
the bundle because the explicit `trust_files` authority is appended. Supply
exactly one authority certificate/file in `trust_files`. The plugin checks the
RSA key match, chain order/signatures/validity, supported 2-QWAC profile, SAN for
the configured hostname and chain verification against that explicit authority
before attempting issuance. The authority is for local consistency only, separate
from ordinary TLS/ACME roots and browser trusted/QWAC authorities. No root is
trusted because it happened to arrive in a leaf bundle. No AIA network fetching,
CA/key generation or browser trust changes happen in provisioning.

`key_file` supports an **unencrypted RSA PKCS#1 or PKCS#8** PEM key. RS256 is the
only signing algorithm. Encrypted keys, HSM integration, filesystem watching and
automatic enrollment are not supported. The supported certificate profile is
`QNCP-w-gen`, only `tls-binding` EKU, digitalSignature, legal-person identity,
QC compliance/web-type statements, SAN, AIA and CRL metadata. Presence of those
fields does not establish revocation freshness or legal qualification.

The binding lifetime defaults to **1h**, minimum **3s**. It is capped by the
earliest QWAC-chain expiry and the latest retained TLS leaf expiry. Older TLS
leaves stay covered for authenticated held connections alongside renewed leaves.
Refresh uses actual signed expiry,
with a default margin of one third of the configured/effective lifetime. JSON
accepts readable durations or legacy numeric nanoseconds. See the
[operational policy](operations.md) for limits, diagnostics and offline retirement.
Strict standard bindings are default. Set the explicit
`firefox_compatibility` switch (JSON `true`) for the known Firefox crit-header
compatibility variant; it is never chosen from User-Agent.

Use absolute credential/state paths in deployment. Caddy's OS account must read
the credentials and exclusively write state; provision state directories mode
0700 and key files mode 0600, with backups/access restricted accordingly. Newly
created state/document/manifest files use those modes. Existing directory
permissions are the operator's responsibility. The live manager holds an OS
file lock; another process cannot provision the same state. Do not remove its
lock file while Caddy runs. Lock ownership ends at final configuration cleanup
or process exit; an existing unlocked lock file is harmless. A separate
`caddy validate` process cannot open state owned by a running server; validate
before startup, or use the live admin reload for authoritative validation.

## Reload and credential replacement

```sh
./caddy adapt --config Caddyfile --adapter caddyfile --pretty > caddy.json
./caddy validate --config caddy.json   # before starting the state owner
./caddy run --config Caddyfile --adapter caddyfile
# Later, after replacing the supplied leaf/key files:
./caddy reload --config Caddyfile --adapter caddyfile --force
```

Explicit `--force` reload is necessary when configuration bytes remain unchanged
and only files change. Replace the leaf/bundle and its matching key consistently;
prefer atomic file replacement before reload, avoid in-place truncation of live
files. Inconsistent files reject provisioning without installing the candidate.
Each new configuration rereads and validates the supplied files. An overlapping
config stages credentials and all retained/stored/new TLS leaves together; at
first live use one complete replacement binding is signed, independently verified
and durably published before switching signer, coverage or generation. Old handlers
and issuance tasks continue sharing current material/state and cannot reinstall
an older credential generation. Only one refresh worker exists per state owner;
old cleanup cannot stop a new lease. Rejected validation/reload starts no new
worker and retains no unused lease. Publication failure keeps the prior binding
and credentials active and protected requests fail closed if it expires.

Only leaf/key replacement **under the same configured authority** is supported.
Authority migration requires a separately designed operational/browser trust
transition and is unsupported. Hostname, profile, lifetime/state ownership
changes are rejected while overlapping with a conflicting live owner; plan a
stop/reconfigure rather than mutating shared state settings during reload.
Restart validates/reuses the atomic manifest and immutable documents, reconstructs
retained coverage, and republishes before advertising recovered material.

## HTTP behavior and failure handling

The advertised discovery endpoint is **`/bindings/current.jose`**, stable across
refresh, renewal and QWAC replacement. GET returns a verified copy of one committed
current document, `application/jose`, `Cache-Control: no-store`. Expired/unavailable
current state returns 503. Binding responses never advertise themselves.

Immutable **`/bindings/<64 lower-case hex SHA-256>.jose`** artifacts remain separate
for recovery and grace-protected historical access. GET returns independently
verified, content-addressed, retained `application/jose`,
`Cache-Control: no-store`. HEAD has identical response metadata without a body;
other methods return 405 with `Allow: GET, HEAD`. Invalid names, traversal,
symlinks, unknown files or modified/unverifiable documents return 404. Historical
immutable documents remain available until collection eligibility (including
expired documents, whose signed expiry the consumer must evaluate).
The endpoint never maps arbitrary files/private material.

For ordinary protected responses the middleware appends the stable discovery Link
at final response commit, after downstream headers. Other application Link values
and Cache-Control survive. Status, bodies and streaming are passed through without
buffering. Applications should reserve the binding relation for this middleware.
An unavailable/expired binding produces 503 instead of running the protected
application; unrelated hostnames pass through. A new TLS certificate cannot be
returned by the issuer until signing, verification and durable publication all
succeed. Refresh/publication/recovery errors use Caddy structured logs. Existing
valid TLS/bindings remain usable during recoverable failures and automatic ACME
retry/independent refresh can recover; inspect Caddy logs before changing trust.

Every published TLS leaf remains in binding coverage, including an authenticated
HTTP/2 connection that outlives leaf expiry. This does not waive expiry checks on
new handshakes. Automatic document collection preserves current/staged artifacts
and a default 24h grace after both signed expiry and file modification. It never
prunes TLS coverage. Enforced defaults refuse additions beyond 256 retained leaves
or publications above 1 MiB; exceeding capacity can prevent TLS renewal. Follow
the [full drain/offline retirement procedure](operations.md#offline-tls-retirement)
before capacity is reached. Distributed storage is unsupported; multiple
independent single-host owners may run in one Caddy instance with separate state.

Caddy evaluates `handle_errors` outside the normal middleware chain. Put a
`qwac_binding` handler with identical settings first inside custom error routes
that must advertise bindings. This preserves arbitrary error-handler Link/cache
headers at commit; the site-level normal handler cannot wrap that separate chain.
In advanced JSON, pair the same middleware in `servers.*.errors.routes`.

## Troubleshooting

| Symptom | Check and recovery |
|---|---|
| Private module cannot be fetched | Confirm repository access with your normal Git credentials, scoped `GOPRIVATE`, and the immutable ref above. The repository is private. |
| Caddy JSON module decoding fails with the tested Go toolchain | Build with `GOEXPERIMENT=nojsonv2` and the documented Caddy/xcaddy pins. |
| Credentials fail before issuance | Check leaf-first PEM order, unencrypted RSA key match, validity, hostname SAN/profile, and the separately supplied authority. Inspect the complete provisioning error; TLS roots are not QWAC authorities. |
| State is already owned | Stop the other owner, or use a separate state directory. A standalone validation process also conflicts with a running owner. Do not delete its lock file. |
| Reload does not pick up new files | Use `caddy reload --force`; unchanged configuration bytes otherwise need not reprovision. Check the reload exit status and the activated signer in structured logs. |
| Reload is rejected | Repair the supplied files or inconsistent settings and retry. The previous live configuration remains active; no candidate worker is started by validation. |
| Protected response is 503 | Inspect refresh/publication logs, credential validity, state permissions and disk space. Restore signing/publication so independent refresh or gated issuance can recover. Do not bypass the gate or erase retained history. |
| HTTPS works but the browser shows no qualified identity | Local plugin validation is separate from the browser's QWAC authority list, feature support and binding profile. Use the isolated demonstration and browser compatibility guide; a page label is insufficient. |

## Evaluate before deployment

Use the [isolated demonstration](demonstration.md) to see native identity and
binding refresh with synthetic credentials. Check [support and browser
compatibility](site-support.md) before interpreting native identity indicators.
Original project code is [MIT licensed](licensing.md).
