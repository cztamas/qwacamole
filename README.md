# Qwacamole

Qwacamole validates operator-supplied **2-QWAC credentials** and binds them to
the TLS certificates actually served by Caddy or a dedicated Certbot/Nginx
installation. Shared signing, durable publication and lifecycle policy gate TLS
activation and protected responses. Credential preparation, inspection and import
use the production `qwacamole-agent` CLI.

This is an **experimental, unpublished alpha**. The repository is public.
Original project code is MIT licensed. Owner publication approval for
`0.1.0-alpha.1` is recorded in [the preparation PR](https://github.com/cztamas/qwacamole/pull/14);
the version has not been tagged or released. Configured authority-chain trust
does not establish EU trusted-list qualification, revocation status or browser
trust. Synthetic demonstrations make no legal identity claim.

## Choose an integration

Start with the [Caddy quickstart](docs/guides/caddy-quickstart.md) or
[Certbot/Nginx quickstart](docs/guides/nginx-quickstart.md). The
[binding overview](docs/guides/site-overview.md) explains the two certificate roles.

| Component | Distribution and entry point |
| --- | --- |
| Shared core | Root Go module; stdlib-only credentials, profiles, bindings, storage and lifecycle |
| Caddy integration | Plugin-included Caddy package and separate Go module; [installation/configuration](docs/guides/caddy-plugin.md) |
| Certbot/Nginx agent | Separate `agent` module and production executable; [installation/operation](docs/guides/agent-nginx.md) |
| Credential workflow | [Local RSA key and CSR, inspection, import and replacement](docs/guides/byo-credentials.md), usable for both integrations |
| Development demos | Separate `devtools` module and `qwacamole` executable; [Chrome/Firefox setup and checks](docs/demos.md), [Firefox details](docs/firefox.md) |

Production supports supplied RSA keys of at least 2048 bits, RS256 bindings,
explicitly selected trust, one literal DNS hostname per state and same-authority
credential replacement. Caddy can host independent protected sites; the agent
owns one complete Nginx configuration and master. Existing shared multi-site
Nginx instances, automatic QWAC enrollment, authority migration, distributed
state, HSMs and arbitrary upstream CDN TLS termination are unsupported.

## Install and configure

Start with [packaged installation](docs/guides/binary-installation.md), then follow
one canonical quickstart:

- [Caddy: install → configure → start → verify](docs/guides/caddy-quickstart.md).
  Its separate package already includes Caddy and the qwacamole plugin.
- [Certbot/Nginx: install → configure → start → verify](docs/guides/nginx-quickstart.md).
  Install Nginx and Certbot separately; the package provides the production agent.

Packages require neither Go nor a repository clone. Automated setup supports
**Ubuntu 24.04 amd64** with a dedicated integration host and supplied credentials.
The Nginx quickstart requires public IPv4/A-record reachability and no AAAA record.
See [supported targets and tested versions](docs/guides/site-support.md) for
runtime-tested platforms and cross-compiled targets.

**Public releases and downloads are not available yet.** The installation guide
provides the authenticated GitHub Actions candidate path, identity/checksum
verification and the future public procedure. A candidate is a review
artifact; its existence does not approve publication.

For custom builds, use the linked [advanced source/xcaddy installation](docs/guides/source-installation.md).
Keep installation commands in those canonical guides so source revisions,
prerequisites and package identities stay consistent.

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
coverage to hide failure. See [operations/recovery](docs/guides/operations.md),
[agent migration](docs/guides/agent-migration.md) and [credential replacement](docs/guides/byo-credentials.md#replacement-and-rejected-update-recovery).

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
[the demo guide](docs/demos.md) and [agent guide](docs/guides/agent-nginx.md).
`make build` includes synthetic development adapters; only `make plugin-build`
and `make agent-build` produce production components. Root tests alone miss the
three nested modules. Generated keys, profiles and raw logs stay ignored.

## Release preparation and evidence

The static documentation website is isolated under [website/](website/README.md):
install Node 24.21.0 and run `npm --prefix website ci`, then start it from the
repository root with `./scripts/start-website.sh`. Browse
`http://127.0.0.1:4321/`; stop with Ctrl+C. Use `--port 4400` or `PORT=4400`
to select another localhost port. The script also works from another directory.
For a production preview, run `npm --prefix website run build`, then
`npm --prefix website run preview -- --ignore-lock` (Ctrl+C to stop).
See [local setup and testing](website/README.md) for exact Node commands,
checks and preview/search behavior. It renders the same operator Markdown used here, with static
search and no Go/build credentials. The published site is [qwacamole.com](https://qwacamole.com/).
[Cloudflare Pages deployment](docs/website-deployment.md) is enabled for checked
main builds; it is independent of software release publication.
The [audience and ordering report](docs/evidence-website-audience-20261010.md) records
new deployment regressions, product-route boundaries and local browser review.
The [dated website report](docs/evidence-website-20261009.md) includes responsive
screenshots, website checks and fresh native demonstration results.

[The alpha checklist](docs/alpha-release.md) defines support, separate module tags,
licensing/security prerequisites, candidate packaging and the exact manual
publication sequence. [Draft release notes](docs/release-notes-alpha.md) describe
the proposed first alpha. [Licensing and attribution inventory](docs/guides/licensing.md)
describes MIT and the retained third-party notices. [SECURITY.md](SECURITY.md) describes current reporting limits.

The [current alpha-readiness report](docs/evidence-alpha-readiness-20261016.md)
records the dependency/security update, selected candidate and newly executed
checks. Earlier reports retain their original source/tool identities.

The [9 October release-preparation report](docs/evidence-release-preparation-20261009.md)
records newly executed checks separately from inherited evidence. Previous dated
reports remain intact: [stream rejection](docs/evidence-caddy-stream-rejection-20261009.md),
[lifecycle and credentials](docs/evidence-lifecycle-20261008.md), and
[Nginx serving/migration](docs/evidence-agent-serving-20261008.md).
