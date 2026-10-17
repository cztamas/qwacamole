#!/bin/sh
# Install the actual pushed agent module from an immutable ref in a fresh cache.
set -eu
ref=${1:?Usage: check-remote-agent.sh immutable-agent-commit}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go=${GO:-$root/.tools/go/bin/go}
case "$go" in /*) ;; *) go=$(command -v "$go");; esac
scratch=$(mktemp -d "${TMPDIR:-/tmp}/qwacamole-agent-remote.XXXXXX")
chmod 700 "$scratch"
export GOWORK=off GOEXPERIMENT=jsonv2
export GOPRIVATE="github.com/cztamas/qwacamole${GOPRIVATE:+,$GOPRIVATE}"
export GOPATH="$scratch/gopath" GOMODCACHE="$scratch/modules" GOBIN="$scratch/bin"
export GOCACHE="${GOCACHE:-$scratch/build-cache}"
cd "$scratch"
"$go" list -m -json "github.com/cztamas/qwacamole/agent@$ref" > agent-resolved.json
version=$("$go" list -m -f '{{.Version}}' "github.com/cztamas/qwacamole/agent@$ref")
# agent/ is on main, so ordinary immutable package installation works.
"$go" install "github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@$ref" > install.log 2>&1
"$go" version -m bin/qwacamole-agent > build-modules.txt
if grep -E '=>|qwacamole/(caddy|devtools)|caddyserver|chromedp|k8s.io' build-modules.txt; then echo 'Production agent boundary violated' >&2;exit 1;fi
mkdir -p "$root/.local/evidence/remote-agent" "$root/bin"
cp agent-resolved.json install.log build-modules.txt "$root/.local/evidence/remote-agent/"
cp bin/qwacamole-agent "$root/bin/qwacamole-agent-remote"
printf '%s\n' "$scratch" > "$root/.local/evidence/remote-agent/scratch-path.txt"
printf '%s\n' 'PASS remote agent installation without workspace/replacements/demo dependencies'
cat build-modules.txt
