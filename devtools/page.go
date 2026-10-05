package devtools

import "html/template"

var page = template.Must(template.New("demo").Parse(`<!doctype html>
<html><head><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Qwacamole — TEST PKI</title>
<style>body{font:18px system-ui;max-width:850px;margin:60px auto;padding:24px;background:#f5f5ef;color:#183b2d}code,a{overflow-wrap:anywhere}li{margin:12px 0}</style>
</head><body><h1>Qwacamole</h1>
<p>Synthetic organization: {{.Owner.Organization}}, {{.Owner.Locality}}, {{.Owner.Country}}, {{.Owner.Registration}}.</p>
<p><strong>TEST PKI. No legal qualification.</strong></p>
{{if .BindingProfile}}<h2>Check Firefox's verified status</h2>
<p>Click the padlock beside the address, then <strong>Connection secure</strong>.
Firefox's native panel should show <strong>{{.Owner.Organization}}</strong> and
<strong>Qualified as specified in Regulation (EU) 2024/1183.</strong></p>
<p>This Firefox compatibility binding omits the critical-header declaration and uses Mozilla's public test QWAC authority.</p>
<p>This page reports server state. Browser verification comes from Firefox's native identity panel.</p>
{{else}}<h2>Check Chrome's verified status</h2>
<p>Click the site controls icon beside the address, then <strong>Connection is secure</strong>.
Chrome's own Security panel should show <strong>EU Qualified Status</strong> and
<strong>Issued to: {{.Owner.Organization}} [{{.Owner.Country}}]</strong>.</p>
<p>This page reports server state. It cannot independently attest browser QWAC verification;
that evidence comes from Chrome's native Security panel and Chromium NetLog.</p>{{end}}
<h2>Published TLS identifiers</h2><ul>{{range .IDs}}<li><code>{{.}}</code></li>{{end}}</ul>
<p>Binding: <a href="{{.URL}}">{{.URL}}</a></p><p>Expiry (Unix): {{.Expires}}</p></body></html>`))
