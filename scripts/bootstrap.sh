#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools .local/evidence
if [ ! -x .tools/go/bin/go ]; then
  case "$(uname -s)-$(uname -m)" in
    Darwin-arm64) archive=go1.27.2.darwin-arm64.tar.gz; digest=76812b213b1b2302c978d28fa52fa92d541704b9e7d9d5db8002c50e4018c4c5 ;;
    *) echo 'Install Go 1.27.2 and set GO=/absolute/path/to/go for this platform.' >&2; exit 1 ;;
  esac
  curl -fsSL "https://go.dev/dl/$archive" -o .tools/go.tar.gz
  echo "$digest  .tools/go.tar.gz" | shasum -a 256 -c -
  tar -xzf .tools/go.tar.gz -C .tools
fi
if [ "$(GOTOOLCHAIN=local .tools/go/bin/go env GOVERSION)" != go1.27.2 ]; then
  echo 'Move the existing .tools/go aside and rerun bootstrap for verified Go 1.27.2.' >&2
  exit 1
fi
export PATH="$PWD/.tools/go/bin:$PATH"
export GOPATH="$PWD/.tools/gopath" GOCACHE="$PWD/.tools/gocache" GOBIN="$PWD/.tools/bin"
# Current upstream Google API dependencies require jsontext; Caddy 2.11.7 uses
# reflected type identity and fixes the old JSON alias incompatibility.
export GOEXPERIMENT=jsonv2
[ -x .tools/bin/xcaddy ] || go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
[ -x .tools/bin/pebble ] || go install github.com/letsencrypt/pebble/v2/cmd/pebble@v2.8.0
if [ ! -x .tools/bin/govulncheck ] || ! .tools/bin/govulncheck -version | grep -qx 'Scanner: govulncheck@v1.9.0'; then
  go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
fi
