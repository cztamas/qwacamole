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
current_tool() {
  [ -x "$1" ] || return 1
  tool_metadata=$(go version -m "$1")
  printf '%s\n' "$tool_metadata" | head -n 1 | grep -q ': go1.27.2$' || return 1
  printf '%s\n' "$tool_metadata" | grep -Fq "$(printf '\tmod\t%s\t%s\t' "$2" "$3")"
}
current_tool .tools/bin/xcaddy github.com/caddyserver/xcaddy v0.4.5 || go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
current_tool .tools/bin/pebble github.com/letsencrypt/pebble/v2 v2.8.0 || go install github.com/letsencrypt/pebble/v2/cmd/pebble@v2.8.0
if ! current_tool .tools/bin/govulncheck golang.org/x/vuln v1.9.0; then
  go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
fi
