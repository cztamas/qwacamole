#!/bin/sh
# An authenticated remote build of the IMPLEMENTED immutable ref, with no replace.
set -eu
ref=${1:?Usage: check-remote-install.sh immutable-plugin-commit}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go=${GO:-$root/.tools/go/bin/go}
xcaddy=${XCADDY:-$root/.tools/bin/xcaddy}
case "$go" in /*) ;; *) go=$(command -v "$go");; esac
case "$xcaddy" in /*) ;; *) xcaddy=$(command -v "$xcaddy");; esac
scratch=$(mktemp -d "${TMPDIR:-/tmp}/qwacamole-remote.XXXXXX")
chmod 700 "$scratch"
export GOWORK=off GOEXPERIMENT=nojsonv2
export GOPRIVATE="github.com/cztamas/qwacamole${GOPRIVATE:+,$GOPRIVATE}"
export GOPATH="$scratch/gopath" GOMODCACHE="$scratch/modules"
export GOCACHE="${GOCACHE:-$scratch/build-cache}"
export PATH="$(dirname "$go"):$PATH"
# Use normal existing Git authentication; callers may select SSH with Git's
# per-process GIT_CONFIG_* URL rewrite, or use their usual HTTPS credential helper.
cd "$scratch"
"$go" version > toolchain.txt
"$xcaddy" version >> toolchain.txt
"$go" list -m -json "github.com/cztamas/qwacamole/caddy@$ref" > plugin-resolved.json
"$xcaddy" build v2.10.2 --with "github.com/cztamas/qwacamole/caddy@$ref" --output "$scratch/caddy" > build.log 2>&1
./caddy list-modules > modules.txt
"$go" version -m ./caddy > build-modules.txt
for id in tls.issuance.qwac http.handlers.qwac; do
 grep -qx "$id" modules.txt || { echo "Missing $id" >&2; exit 1; }
done
if grep -E 'qwac_demo|qwacamole/devtools' modules.txt build-modules.txt; then
 echo 'Development dependency/module leaked' >&2; exit 1
fi
if grep -q '=>' build-modules.txt; then
 echo 'Remote binary contains a module replacement' >&2; exit 1
fi
# Go build metadata is authoritative for both resolved immutable module versions.
grep 'github.com/cztamas/qwacamole' build-modules.txt > resolved-versions.txt
mkdir -p "$root/.local/evidence/remote-install"
cp toolchain.txt plugin-resolved.json build.log modules.txt build-modules.txt resolved-versions.txt "$root/.local/evidence/remote-install/"
cp caddy "$root/bin/caddy-remote"
printf '%s\n' "$scratch" > "$root/.local/evidence/remote-install/scratch-path.txt"
printf 'PASS remote install without replacements or devtools\nScratch: %s\n' "$scratch"
cat resolved-versions.txt
