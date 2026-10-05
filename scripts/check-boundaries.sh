#!/bin/sh
# Dependency graphs, including production package imports, enforce the layers.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
go="$root/.tools/go/bin/go"
export GOEXPERIMENT=nojsonv2 GOPATH="$root/.tools/gopath" GOCACHE="$root/.tools/gocache"
core=$($go list -deps ./...)
if printf '%s\n' "$core" | grep -E 'caddyserver|chromedp|qwacamole/(caddy|devtools)|protobuf'; then
 echo 'core dependency boundary violated' >&2; exit 1
fi
plugin=$(cd caddy && "$go" list -deps ./...)
if printf '%s\n' "$plugin" | grep -E 'qwacamole/devtools'; then
 echo 'plugin dependency boundary violated' >&2; exit 1
fi
printf '%s\n' 'PASS core and production plugin dependency boundaries'
