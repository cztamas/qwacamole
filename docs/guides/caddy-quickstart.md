# Caddy quickstart

Bind supplied organization credentials to Caddy's browser-facing TLS. This
quickstart uses the current immutable plugin source, with strict bindings.

## 1. Check prerequisites

You need authorized access to this private repository, Go **1.27.1**, a
reachable application on `127.0.0.1:8080`, and a real DNS hostname pointing
directly to this host. Public ACME needs reachable challenge ports. No upstream
CDN may terminate TLS on a different certificate.

Obtain a valid RSA 2-QWAC certificate/leaf-first chain and matching unencrypted
key. Select one authority PEM separately through your issuer verification
process. If starting with a new key, follow [key/CSR preparation](byo-credentials.md);
qwacamole does not enroll with an issuer.

The reserved `qwac.example.com` and `admin@example.com` below are placeholders.
Replace the hostname consistently with the single SAN you will bind, the email
with your ACME contact, and input paths with your supplied files. Synthetic
credentials are for [isolated demonstrations](demonstration.md) only.

## 2. Build and verify the plugin

Use your existing authorized Git credential helper or SSH setup. Preserve other
GOPRIVATE entries if present.

```sh
export GOPRIVATE=github.com/cztamas/qwacamole
export GOWORK=off GOEXPERIMENT=nojsonv2
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5
xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@9322bd566e825a2c2c645e464a630e09d7aa68d6
./caddy list-modules
go version -m ./caddy
```

Ensure your Go bin directory is on PATH. Expect `tls.issuance.qwac` and
`http.handlers.qwac`, with no devtools modules or replacements. This is an
authenticated install; future `v0.1.0-alpha.1` installs are not available yet.

## 3. Prepare owned paths and configuration

On Linux, from an administrator shell, copy the built binary and supplied inputs
to their deployment paths. This foreground example runs Caddy as root to bind
ports 80/443; choose an appropriately restricted service identity and port
capabilities for a managed deployment. Never make the key web-readable.

```sh
sudo install -m 0755 ./caddy /usr/local/bin/caddy-qwac
sudo install -d -m 0700 /etc/caddy/qwac /var/lib/caddy/qwac
sudo install -m 0600 /path/to/issued-chain.pem /etc/caddy/qwac/chain.pem
sudo install -m 0600 /path/to/signing-key.pem /etc/caddy/qwac/key.pem
sudo install -m 0600 /path/to/selected-authority.pem /etc/caddy/qwac/authority.pem
```

Save this as `/etc/caddy/Caddyfile.qwac`, root-owned and writable only by trusted
operators. The state directory must be exclusively writable by Caddy's owner.
Do not add a fallback issuer, manual TLS leaf, wildcard or a second hostname.

```caddyfile
qwac.example.com {
    qwac {
        hostname qwac.example.com
        directory /var/lib/caddy/qwac
        chain_files /etc/caddy/qwac/chain.pem
        key_file /etc/caddy/qwac/key.pem
        trust_files /etc/caddy/qwac/authority.pem
        lifetime 1h
        acme {
            dir https://acme-v02.api.letsencrypt.org/directory
            email admin@example.com
        }
    }
    reverse_proxy 127.0.0.1:8080
}
```

The sole `qwac` directive configures both issuer and middleware. Full
[reverse proxy](../../config/Caddyfile.reverse-proxy),
[file server](../../config/Caddyfile.file-server) and
[JSON](../../config/plugin.json.example) examples remain available.

## 4. Validate, start and verify

Stop any other owner of this state and any conflicting listener first. A
separate validation process cannot acquire a running owner's state lock.

```sh
sudo /usr/local/bin/caddy-qwac adapt --config /etc/caddy/Caddyfile.qwac --adapter caddyfile --pretty
sudo /usr/local/bin/caddy-qwac validate --config /etc/caddy/Caddyfile.qwac --adapter caddyfile
sudo /usr/local/bin/caddy-qwac run --config /etc/caddy/Caddyfile.qwac --adapter caddyfile
```

Leave the server running. In another terminal, using your real hostname:

```sh
curl --fail --show-error --include https://qwac.example.com/
curl --fail --show-error --include https://qwac.example.com/bindings/current.jose
```

The protected response must advertise the stable binding Link. The binding
endpoint must return `application/jose`, `Cache-Control: no-store`, and no
recursive discovery Link. Use ordinary TLS verification; do not add `-k`.
Check activation logs and match the binding's signer with your inspected
credential. HTTP success is operational evidence, not native browser trust.

## 5. Maintain the credential and binding

[Prepare and inspect replacement inputs](byo-credentials.md#replacement-and-rejected-update-recovery)
before selecting them. An unchanged Caddyfile needs a forced reload:

```sh
sudo /usr/local/bin/caddy-qwac reload --config /etc/caddy/Caddyfile.qwac --adapter caddyfile --force
```

Confirm protected requests and the activated signer after reload. Credential
replacement stays under the same authority; failed validation/publication must
not justify deleting coverage. See [recovery and offline retirement](operations.md)
and the [Caddy reference](caddy-plugin.md#troubleshooting).

Strict bindings remain the default. Firefox compatibility is an explicit
configuration variant, described in [browser limitations](site-support.md#browser-profiles).
Neither mode installs browser trust or proves legal qualification.
