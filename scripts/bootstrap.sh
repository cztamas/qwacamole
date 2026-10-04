#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools .local/evidence
if [ ! -x .tools/go/bin/go ]; then
  case "$(uname -s)-$(uname -m)" in
    Darwin-arm64) archive=go1.27.1.darwin-arm64.tar.gz; digest=ee215d57e0ec269c60cc9ceca68e6bda321ba9ee5afe24f4b0988703c2d87d12 ;;
    *) echo 'Install Go 1.27.1 and set GO=/absolute/path/to/go for this platform.' >&2; exit 1 ;;
  esac
  curl -fsSL "https://go.dev/dl/$archive" -o .tools/go.tar.gz
  echo "$digest  .tools/go.tar.gz" | shasum -a 256 -c -
  tar -xzf .tools/go.tar.gz -C .tools
fi
export PATH="$PWD/.tools/go/bin:$PATH"
export GOPATH="$PWD/.tools/gopath" GOCACHE="$PWD/.tools/gocache" GOBIN="$PWD/.tools/bin"
# Caddy 2.10.2 identifies encoding/json.RawMessage by its reflected type name.
# Go 1.27's JSON v2 alias breaks that lookup; retain legacy type identity.
export GOEXPERIMENT=nojsonv2
[ -x .tools/bin/xcaddy ] || go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
[ -x .tools/bin/pebble ] || go install github.com/letsencrypt/pebble/v2/cmd/pebble@v2.8.0
