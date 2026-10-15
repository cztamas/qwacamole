// This entry point builds the candidate distribution from the exact source.
package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/cztamas/qwacamole/caddy"
)

func main() { caddycmd.Main() }
