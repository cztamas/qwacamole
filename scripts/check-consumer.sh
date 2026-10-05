#!/bin/sh
# Build outside go.work, using only explicit local module replacements.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
consumer=$(mktemp -d "${TMPDIR:-/tmp}/qwacamole-consumer.XXXXXX")
trap 'rm -rf "$consumer"' EXIT HUP INT TERM
cat > "$consumer/go.mod" <<EOM
module example.invalid/qwacamole-consumer

go 1.27.1

require (
 github.com/cztamas/qwacamole v0.0.0
 github.com/cztamas/qwacamole/caddy v0.0.0
)
replace github.com/cztamas/qwacamole => $root
replace github.com/cztamas/qwacamole/caddy => $root/caddy
EOM
cat > "$consumer/main.go" <<'EOM'
package main
import (
 "fmt"
 "github.com/caddyserver/caddy/v2"
 "github.com/cztamas/qwacamole/binding"
 plugin "github.com/cztamas/qwacamole/caddy"
 "github.com/cztamas/qwacamole/credentials"
 "github.com/cztamas/qwacamole/profile"
 "github.com/cztamas/qwacamole/store"
)
var _ = binding.Sign
var _ = credentials.Load
var _ = profile.Validate
var _ = store.Open
var _ = plugin.BindingConfig{}
func main(){
 for _,id:=range []string{"tls.issuance.qwac","http.handlers.qwac"}{if _,e:=caddy.GetModule(id);e!=nil{panic(e)}}
 for _,id:=range []string{"tls.issuance.qwac_demo","http.handlers.qwac_demo"}{if _,e:=caddy.GetModule(id);e==nil{panic("development module leaked")}}
 fmt.Println("PASS consumer outside workspace and production-only registrations")
}
EOM
export GOWORK=off GOEXPERIMENT=nojsonv2 GOPATH="$root/.tools/gopath" GOCACHE="$root/.tools/gocache"
cd "$consumer"
"$root/.tools/go/bin/go" mod tidy
"$root/.tools/go/bin/go" build -o consumer .
./consumer
