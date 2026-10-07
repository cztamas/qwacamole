#!/bin/sh
# Real dependency pins only: deliberately no workspace or replacements.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go=${GO:-$root/.tools/go/bin/go}
case "$go" in /*) ;; *) go=$(command -v "$go");; esac
export GOWORK=off GOEXPERIMENT=nojsonv2
export GOPRIVATE="github.com/cztamas/qwacamole${GOPRIVATE:+,$GOPRIVATE}"
if [ -z "${GO:-}" ]; then export GOPATH="$root/.tools/gopath" GOCACHE="$root/.tools/gocache"; fi
for module in . caddy agent devtools; do
 (
  cd "$root/$module"
  if "$go" list -m -json all | grep -q '"Replace"'; then echo "Independent $module uses replacement" >&2;exit 1;fi
  "$go" test -race ./...
 )
done
printf '%s\n' 'PASS independently pinned module races without go.work/replacements'
