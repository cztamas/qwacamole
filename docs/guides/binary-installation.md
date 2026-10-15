# Install a packaged binary

Public releases and downloads are not available yet. Authorized repository users
can install a retained, successful candidate from GitHub Actions. You need the
GitHub CLI (`gh`), repository access, `tar` and SHA-256 tools; **Go and a repository
clone are not needed**. Obtain the successful run ID, full source commit and
reviewed manifest SHA-256 from the person providing the candidate.

Agent archives: Linux amd64, Linux arm64 and macOS arm64. Linux arm64 is
cross-compiled only. The packaged Caddy binary targets Linux amd64. Automated
setup targets **Ubuntu 24.04 amd64** and one dedicated integration per host.
Other deployment arrangements use the integration references.

## Download and check the candidate

Replace the three identity placeholders below. Keep `VERSION` pinned to the
candidate's version. Download into a new directory:

```sh
VERSION=0.1.0-alpha.1
SOURCE=FULL_SOURCE_COMMIT
RUN_ID=SUCCESSFUL_CANDIDATE_RUN_ID
MANIFEST_SHA256=REVIEWED_MANIFEST_SHA256
mkdir qwacamole-candidate
cd qwacamole-candidate
gh run view "$RUN_ID" --repo cztamas/qwacamole --json conclusion,headSha
# Confirm success and that headSha is exactly $SOURCE before proceeding.
gh run download "$RUN_ID" --repo cztamas/qwacamole \
  --name "agent-candidate-$SOURCE" --dir .
printf '%s  manifest.json\n' "$MANIFEST_SHA256" | sha256sum -c -
sha256sum -c SHA256SUMS
```

On macOS use `shasum -a 256 -c` for both checksum commands. Stop if any identity
or checksum differs. Checksums detect corruption; the separately reviewed
manifest and authenticated run identify the supplied candidate.

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
It refuses an existing prefix; for another candidate or upgrade, choose a new
version/source-specific directory and follow the [maintenance guide](operations.md).
It never overwrites an existing Caddy binary or starts a service.

On the other agent targets, select the matching archive and a new installation
prefix, then use [manual deployment](agent-nginx.md). Continue with the
[Caddy quickstart](caddy-quickstart.md) or [Nginx quickstart](nginx-quickstart.md).
[Source installation](source-installation.md) remains available for custom builds.

## After public publication

Only after a release exists, replace the authenticated candidate download with
`gh release download "v$VERSION" --repo cztamas/qwacamole --dir .` in a new
directory. Use that release's independently obtained manifest digest and the
same checksum, extraction and installation steps. This command is a future
procedure, not evidence that the proposed release is available now.
