package qwacamole

import (
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddytls"
)

func init() {
	httpcaddyfile.RegisterDirective("qwac", parseSite)
	httpcaddyfile.RegisterDirectiveOrder("qwac", httpcaddyfile.Before, "redir")
	httpcaddyfile.RegisterHandlerDirective("qwac_binding", func(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
		v := new(Handler)
		e := v.UnmarshalCaddyfile(h.Dispenser)
		return v, e
	})
	httpcaddyfile.RegisterDirectiveOrder("qwac_binding", httpcaddyfile.Before, "redir")
}

// qwac { hostname <DNS name>; directory <path>; chain_files <PEM...>;
// key_file <PEM>; trust_files <authority PEM>; [lifetime 45s];
// [firefox_compatibility]; [acme { <ordinary ACME issuer options> }] }
// A site directive installs exactly one gated issuer and the leading middleware.
func parseSite(h httpcaddyfile.Helper) ([]httpcaddyfile.ConfigValue, error) {
	i := new(Issuer)
	if e := i.UnmarshalCaddyfile(h.Dispenser); e != nil {
		return nil, e
	}
	routes := h.NewRoute(nil, &Handler{Binding: i.Binding})
	return append(routes, httpcaddyfile.ConfigValue{Class: "tls.cert_issuer", Value: i}), nil
}

// UnmarshalCaddyfile supports tls { issuer qwac { ... } }. Pair this advanced
// form with qwac_binding and identical settings, or use the site directive.
func (i *Issuer) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	return parseBinding(d, &i.Binding, &i.ACMEIssuer)
}

// UnmarshalCaddyfile supports qwac_binding { ... } without an ACME block.
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	return parseBinding(d, &h.Binding, nil)
}

func parseBinding(d *caddyfile.Dispenser, b *BindingConfig, acme *caddytls.ACMEIssuer) error {
	d.Next()
	if d.NextArg() {
		return d.ArgErr()
	}
	seen := map[string]bool{}
	for d.NextBlock(0) {
		name := d.Val()
		if seen[name] {
			return d.Errf("duplicate %s", name)
		}
		seen[name] = true
		switch name {
		case "hostname":
			if !d.AllArgs(&b.Host) {
				return d.ArgErr()
			}
		case "directory":
			if !d.AllArgs(&b.Dir) {
				return d.ArgErr()
			}
		case "key_file":
			if !d.AllArgs(&b.KeyFile) {
				return d.ArgErr()
			}
		case "chain_files":
			b.ChainFiles = d.RemainingArgs()
			if len(b.ChainFiles) == 0 {
				return d.ArgErr()
			}
		case "trust_files":
			b.TrustFiles = d.RemainingArgs()
			if len(b.TrustFiles) != 1 {
				return d.Err("one explicit authority PEM required")
			}
		case "lifetime":
			var s string
			if !d.AllArgs(&s) {
				return d.ArgErr()
			}
			v, e := caddy.ParseDuration(s)
			if e != nil {
				return d.Errf("invalid binding lifetime: %v", e)
			}
			b.Lifetime = v
		case "firefox_compatibility":
			if d.NextArg() {
				return d.ArgErr()
			}
			b.FirefoxCompatibility = true
		case "acme":
			if acme == nil {
				return d.Err("ACME options belong in the issuer/site qwac directive")
			}
			if e := acme.UnmarshalCaddyfile(d.NewFromNextSegment()); e != nil {
				return e
			}
		default:
			return d.Errf("unrecognized QWAC option: %s", name)
		}
	}
	if _, e := b.normalized(); e != nil {
		return d.Err(fmt.Sprint(e))
	}
	return nil
}

var _ caddyfile.Unmarshaler = (*Issuer)(nil)
var _ caddyfile.Unmarshaler = (*Handler)(nil)
