# A supplied identity, bound to the TLS you serve

Qwacamole connects an operator-supplied **2-QWAC** credential to the ordinary TLS
certificates served by Caddy or a dedicated Certbot/Nginx installation. It signs
a binding, verifies and publishes it durably, and gates new TLS activation and
protected responses on usable binding coverage.

## Two certificates, different jobs

The ordinary TLS certificate authenticates the HTTPS connection. The separate
2-QWAC certificate carries the supplied organization's identity and authorizes
TLS-binding signatures. Qwacamole does not obtain that qualified certificate.
An issuer must supply a suitable certificate and chain for the key you control.

Responses advertise `/bindings/current.jose` through the
`tls-certificate-binding` Link relation. The browser can retrieve the signed
document and compare its covered TLS certificates with the certificate it sees.
The stable discovery endpoint is uncached; immutable documents support recovery.

## Choose your integration

- [Caddy quickstart](caddy-quickstart.md): install Caddy with the plugin already included. One
  site-level directive configures the gated ACME issuer and HTTP middleware.
- [Certbot/Nginx quickstart](nginx-quickstart.md): install the production agent.
  It owns one dedicated Nginx master and deploys Certbot lineages through a
  publication, reload and fresh TLS-confirmation gate.

Both share [credential preparation and validation](byo-credentials.md),
[maintenance and recovery](operations.md), and the same explicit authority trust.
Renewal keeps old TLS leaves covered for authenticated held connections.
Binding refresh runs independently of ordinary TLS renewal. A rejected
same-authority credential replacement preserves the prior valid binding.

## Experimental status and trust boundaries

This is an unpublished experimental alpha. The repository is currently private;
source access requires authorized Git credentials; packaged candidates require authenticated repository access.
No proposed alpha tag or public package download exists yet. Original code is
[MIT licensed](licensing.md); third-party notices remain separate.

Operational validation does not establish EU trusted-list qualification,
revocation freshness or browser trust. Native browsers maintain their own QWAC
authority and compatibility rules. Synthetic demonstrations test behavior and
make no legal identity claim. See [support and browser limitations](site-support.md).

## Learn and operate

[Keys and CSRs](byo-credentials.md#local-key-and-csr) →
[inspection and import](byo-credentials.md#inspect-and-import) →
[configuration and verification](caddy-quickstart.md) →
[replacement and recovery](byo-credentials.md#replacement-and-rejected-update-recovery).

Try the [isolated demonstration](demonstration.md) before selecting production
credentials. Check [supported versions and browser compatibility](site-support.md)
before installing or upgrading.
