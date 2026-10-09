# Qwacamole

Qwacamole validates operator-supplied **2-QWAC credentials** and binds them to
the TLS certificates actually served by Caddy or a dedicated Certbot/Nginx
installation. Shared signing, durable publication and lifecycle policy gate TLS
activation and protected responses. Credential preparation, inspection and import
use the production `qwacamole-agent` CLI.

This is an **experimental, unpublished alpha**. The repository is private.
Original project code is MIT licensed; visibility and publication require owner
approval. The proposed `0.1.0-alpha.1` has not been tagged or released. Configured authority-chain trust
does not establish EU trusted-list qualification, revocation status or browser
trust. Synthetic demonstrations make no legal identity claim.

## Choose an integration

Start with the [Caddy quickstart](docs/caddy-quickstart.md) or
[Certbot/Nginx quickstart](docs/nginx-quickstart.md). The
[binding overview](docs/site-overview.md) explains the two certificate roles.

| Component | Distribution and entry point |
| --- | --- |
| Shared core | Root Go module; stdlib-only credentials, profiles, bindings, storage and lifecycle |
| Caddy integration | Separate `caddy` module, installed with xcaddy; [installation/configuration](docs/caddy-plugin.md) |
| Certbot/Nginx agent | Separate `agent` module and production executable; [installation/operation](docs/agent-nginx.md) |
| Credential workflow | [Local RSA key and CSR, inspection, import and replacement](docs/byo-credentials.md), usable for both integrations |
| Development demos | Separate `devtools` module and `qwacamole` executable; [Chrome/Firefox setup and checks](docs/demos.md), [Firefox details](docs/firefox.md) |

Production supports supplied RSA keys of at least 2048 bits, RS256 bindings,
explicitly selected trust, one literal DNS hostname per state and same-authority
credential replacement. Caddy can host independent protected sites; the agent
owns one complete Nginx configuration and master. Existing shared multi-site
Nginx instances, automatic QWAC enrollment, authority migration, distributed
state, HSMs and arbitrary upstream CDN TLS termination are unsupported.

## Install the current implemented sources

Use Go **1.27.1**, `GOEXPERIMENT=nojsonv2`, xcaddy **0.4.5**, Caddy **2.10.2**
and CertMagic **0.24.0**. Normal authorized Git authentication is required while
the repository is private. These immutable commands work now; no workspace or
local replacement is needed:

```sh
export GOPRIVATE=github.com/cztamas/qwacamole
export GOWORK=off GOEXPERIMENT=nojsonv2
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@9322bd566e825a2c2c645e464a630e09d7aa68d6
./caddy list-modules
go version -m ./caddy

go install github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@bb911a8f69a8743faab70967232ff166c828c4b7
qwacamole-agent version -json
```

Add your Go binary directory to PATH or invoke the installed absolute path. Keep
other existing GOPRIVATE entries if needed. Caddy should list `tls.issuance.qwac`
and `http.handlers.qwac`, with no devtools or replacements. Agent installation
resolves `v0.0.0-20261009145422-bb911a8f69a8`; Caddy resolves
`v0.0.0-20261009072701-9322bd566e82`; both use core
`v0.0.0-20261008203612-8e62f4b8c340`. See each guide for fresh-cache installation,
file ownership, service startup, status, updates and rollback.

The release plan provides Linux amd64, Linux arm64 and macOS arm64 agent archives.
Candidate packages are private review artifacts carrying the selected MIT license
and third-party notices; publication approval remains pending.
After publication, use [the packaged installation procedure](docs/agent-nginx.md#packaged-agent)
with the matching target, SHA256SUMS, manifest and `version` identity. Nginx and
Certbot remain external dependencies. Caddy distribution uses the module and
xcaddy; no custom prebuilt Caddy or hosted-builder support is claimed.

## Browser behavior and lifecycle

Strict bindings are the default. Firefox's compatibility profile is explicitly
configured and confined to documented setup. Stock Chromium accepts an expired
binding `exp`: its preserved eleven-case fixture suite must return nonzero for
that case, while the other ten pass. Firefox rejects that case and passes eleven.
The native tests preserve trust, cached discovery, replacement identity,
automatic renewal, refresh, restart, failure recovery and held TLS connections.

Bindings publish and verify before TLS activation. Replacement candidates are
validated before activation; rejected updates preserve the current valid signer
and complete retained TLS coverage. Maintenance refreshes independently of TLS
renewal. Expiry closes availability until repair; capacity refusal cannot discard
coverage to hide failure. See [operations/recovery](docs/operations.md),
[agent migration](docs/agent-migration.md) and [credential replacement](docs/byo-credentials.md#replacement-and-rejected-update-recovery).

## Develop and verify

On the tested macOS arm64 environment:

```sh
make build agent-build plugin-build
make test
scripts/check-independent-modules.sh
make plugin-smoke STATE=.local/new-caddy-smoke
make nginx-smoke STATE=.local/new-nginx-smoke
```

Run native checks sequentially in distinct isolated state, following
[the demo guide](docs/demos.md) and [agent guide](docs/agent-nginx.md).
`make build` includes synthetic development adapters; only `make plugin-build`
and `make agent-build` produce production components. Root tests alone miss the
three nested modules. Generated keys, profiles and raw logs stay ignored.

## Release preparation and evidence

The static documentation website is isolated under [website/](website/README.md):
Node 24.21.0, `npm ci`, `npm run check`, `npm run build` and
`npm run dev`. It renders the same operator Markdown used here, with static
search and no Go/build credentials. [Cloudflare Pages deployment](docs/website-deployment.md)
defaults disabled until account setup; qwacamole.com hosting is not yet verified.
The [dated website report](docs/evidence-website-20261009.md) includes responsive
screenshots, website checks and fresh native demonstration results.

[The alpha checklist](docs/alpha-release.md) defines support, separate module tags,
licensing/security prerequisites, candidate packaging and the exact manual
publication sequence. [Draft release notes](docs/release-notes-alpha.md) describe
the proposed first alpha. [Licensing and attribution inventory](docs/licensing.md)
describes MIT and the retained third-party notices. [SECURITY.md](SECURITY.md) describes current reporting limits.

The [9 October release-preparation report](docs/evidence-release-preparation-20261009.md)
records newly executed checks separately from inherited evidence. Previous dated
reports remain intact: [stream rejection](docs/evidence-caddy-stream-rejection-20261009.md),
[lifecycle and credentials](docs/evidence-lifecycle-20261008.md), and
[Nginx serving/migration](docs/evidence-agent-serving-20261008.md).
