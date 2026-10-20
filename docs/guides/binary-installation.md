# Install a packaged binary

Install the published experimental **0.1.0-alpha.1** release. Downloads and
verified artifact identities are available on the
[release page](https://github.com/cztamas/qwacamole/releases/tag/v0.1.0-alpha.1).
Use the GitHub CLI (`gh`), `jq`, `tar` and SHA-256 tools; **Go and a repository clone
are not needed**. The release page also provides direct public download links.

Agent archives: Linux amd64, Linux arm64 and macOS arm64. Linux arm64 is
cross-compiled only. The packaged Caddy binary targets Linux amd64. Automated
setup targets **Ubuntu 24.04 amd64** and one dedicated integration per host.
Other deployment arrangements use the integration references.

## Download and verify

Download into a new directory. The reviewed manifest digest comes from the
release notes, separately from the downloaded checksum file:

```sh
VERSION=0.1.0-alpha.1
mkdir qwacamole-release
cd qwacamole-release
gh release view "v$VERSION" --repo cztamas/qwacamole
MANIFEST_SHA256=$(gh release view "v$VERSION" --repo cztamas/qwacamole \
  --json body --jq '.body | capture("Reviewed manifest SHA256: `(?<digest>[0-9a-f]{64})`").digest')
gh release download "v$VERSION" --repo cztamas/qwacamole --dir .
printf '%s  manifest.json\n' "$MANIFEST_SHA256" | sha256sum -c -
jq -r '.archives[] | "\(.sha256)  \(.name)"' manifest.json | sha256sum -c -
sha256sum -c SHA256SUMS
```

On macOS use `shasum -a 256 -c` for all checksum commands. Stop if the release,
reviewed digest or any asset is missing, or if any checksum differs. Check the
manifest's version, source/core identities and target against the release notes.
The manifest's archive digests authenticate each downloaded archive; the mutable
`SHA256SUMS` asset alone cannot establish that link. The separately reviewed
manifest and release record identify the artifacts. Never substitute an
unreviewed Actions candidate.

## Install your integration

For the supported Ubuntu agent setup:

```sh
tar -xzf "qwacamole-agent-$VERSION-linux-amd64.tar.gz"
sudo "./qwacamole-agent-$VERSION-linux-amd64/install.sh" \
  --prefix /opt/qwacamole-agent
```

For Caddy on Linux amd64:

```sh
tar -xzf "qwacamole-caddy-$VERSION-linux-amd64.tar.gz"
sudo "./qwacamole-caddy-$VERSION-linux-amd64/install.sh" \
  --prefix /opt/qwacamole-caddy
```

The installer checks the platform, installs executable permissions, retains
instructions/templates/notices and prints the installed version and source.
It refuses an existing prefix; for an upgrade, choose a new
version/source-specific directory and follow the [maintenance guide](operations.md).
It never overwrites an existing Caddy binary or starts a service.

On the other agent targets, select the matching archive and a new installation
prefix, then use [manual deployment](agent-nginx.md). Continue with the
[Caddy quickstart](caddy-quickstart.md) or [Nginx quickstart](nginx-quickstart.md).
[Source installation](source-installation.md) remains available for custom builds.
