# Bring your own 2-QWAC

Supply a **2-QWAC TLS-binding certificate**, its issuer chain and its matching
private key. Ordinary TLS QWACs, PSD2 certificates with incompatible key usages
and electronic seal certificates are not substitutes. These commands require no
remote enrollment, issuer account or paid service. A qualified issuer remains
responsible for issuing your real certificate.

Preparation commands belong to the existing production `qwacamole-agent` CLI
([installation](agent-nginx.md#installation-and-configuration)) and work for
**both Caddy and Nginx**, without a
deployment configuration, Nginx or devtools. Each production binding configuration
owns one literal hostname; use separate state/configuration for multiple sites.

## Local key and CSR

```sh
umask 077
mkdir -m 0700 qwac-input
qwacamole-agent keygen -out qwac-input/signing-key.pem -bits 3072
qwacamole-agent csr -key qwac-input/signing-key.pem -out qwac-input/request.pem \
  -country HU -organization 'Example Kft.' -registration NTRHU-12345678 \
  -common-name example.com -locality Budapest -province Budapest \
  -street 'Example utca 1' -postal-code 1000 \
  -hostname example.com -hostname www.example.com
```

`keygen` supports RSA 2048, 3072 (default) and 4096 bits, writing unencrypted
PKCS#8 PEM. `csr` leaves the existing key unchanged and verifies the generated
PKCS#10 signature before writing. Both create mode 0600 files, refuse existing
paths (including symlinks), and never print private keys. Parent directories must
be private. Required subject fields are country, registered organization, distinct
organizationIdentifier and common name; locality, province, street and postal
code are optional. Repeat `-hostname` for all lowercase literal DNS SANs. Use the
registration syntax your issuer requires.

The CSR requests digitalSignature, QNCP-w-gen and the tls-binding EKU.
Requested fields/extensions **do not guarantee compliant issuance**. QcCompliance
and website QcType are issuer assertions checked on the issued certificate.
Send only the CSR to your issuer. Keep the signing key local and out of Git,
logs, tickets and web roots; protect backups with your secret-storage policy.
Supported key inputs are unencrypted PKCS#1 or PKCS#8 RSA PEM. Encrypted PEM,
PKCS#12, DER keys, EC keys and HSM/remote references are unsupported. Bindings use
RS256.

## Inspect and import

Save the leaf certificate followed by its intermediates in PEM. The root may be
included or omitted; select it **separately** through an authenticated source and
your issuer-verification process:

```sh
qwacamole-agent inspect-qwac \
  -chain qwac-input/issued-chain.pem -key qwac-input/signing-key.pem \
  -trust /etc/qwacamole/trust/qwac-root.pem \
  -hostname example.com -hostname www.example.com
qwacamole-agent import-qwac \
  -chain qwac-input/issued-chain.pem -key qwac-input/signing-key.pem \
  -trust /etc/qwacamole/trust/qwac-root.pem -hostname example.com \
  -out /etc/qwacamole/credentials-v1
```

Repeat `-chain` for separate files in leaf-first order and `-hostname` for every
hostname the integration will bind. Both integrations and these commands share
the core validation path. Public JSON reports subject, DNS names, validated
hostnames, validity, leaf/authority SHA-256 fingerprints, chain size, algorithm
and validation scope. Failure returns nonzero and an actionable error without a
success report or private key output.

Checks cover strict PEM parsing, key matching and minimum RSA 2048-bit size,
QNCP-w-gen `0.4.0.194112.1.6`, exclusively tls-binding EKU
`0.4.0.194115.1.0`, legal-person subject/issuer fields, AIA/CRL locations,
digitalSignature with no encryption/agreement/CA usages (contentCommitment may
also be present), QcCompliance and website-only QcType, statement encoding and
duplicates, every hostname, all chain certificate dates, issuer signatures and
Go X.509 path verification to the explicit authority. Missing intermediates,
wrong order and untrusted roots reject. No AIA fetching or system-root fallback
occurs. Exactly one CA certificate is supported in the separate trust file,
preserving the existing single-authority binding-store policy.

Profile references: [ETSI TS 119 411-5 V2.1.1](https://www.etsi.org/deliver/etsi_ts/119400_119499/11941105/02.01.01_60/ts_11941105v020101p.pdf),
[EN 319 412-4 V1.4.1](https://www.etsi.org/deliver/etsi_en/319400_319499/31941204/01.04.01_60/en_31941204v010401p.pdf),
[EN 319 412-5 V2.6.1](https://www.etsi.org/deliver/etsi_en/319400_319499/31941205/02.06.01_60/en_31941205v020601p.pdf).
This supported local profile check is not a complete issuance audit. Trusting an
operator-supplied root **does not independently establish EU trusted-list
qualification**, revocation status or native browser trust. Production has no
trusted-list/revocation retrieval service. Existing Chrome QWAC metadata and
Firefox compiled-authority checks remain separate from ordinary TLS trust.
Synthetic devtools authorities have no legal qualification and are enabled only
in the explicit isolated test setup. These commands never modify browser/system
trust or the supplied trust file.

Import validates before creating a **new** mode 0700 directory containing a
normalized full `chain.pem` and PKCS#8 `key.pem`, both mode 0600. Its parent must
already exist and be writable only by trusted operators. Existing destinations
reject; a write failure cleans only this invocation's new directory. Source files
stay unchanged. Public output contains absolute-path Caddy and agent field
snippets referencing the separately supplied trust file. Import copies the key
but does not select or activate credentials, change configuration, or expand trust.

## Initial integration configuration

For replacement without changing configured paths, create a stable pointer:

```sh
ln -s /etc/qwacamole/credentials-v1 /etc/qwacamole/current
```

In **both** the Caddy issuer and handler's `binding` configuration, retain the
existing hostname, state directory and lifecycle settings and supply:

```json
{
  "chain_files": ["/etc/qwacamole/current/chain.pem"],
  "key_file": "/etc/qwacamole/current/key.pem",
  "trust_files": ["/etc/qwacamole/trust/qwac-root.pem"]
}
```

The Caddyfile `qwac` block uses `chain_files`, `key_file`, `trust_files`.
Follow the complete [Caddy setup](caddy-plugin.md), keeping QWAC as the sole TLS
issuer, and start Caddy normally.

In the full [dedicated Certbot/Nginx agent configuration](agent-nginx.md), use:

```json
{
  "qwac_chain_files": ["/etc/qwacamole/current/chain.pem"],
  "qwac_key_file": "/etc/qwacamole/current/key.pem",
  "qwac_trust_file": "/etc/qwacamole/trust/qwac-root.pem"
}
```

Render/validate Nginx, start the agent and obtain/deploy ordinary TLS through the
existing Certbot hook. `validate` checks the complete deployment; `inspect-qwac`
checks only the supplied credential. Both integrations accept a chain with the root included or omitted, while
requiring an independently selected trust anchor. Full-chain and separate-file
configurations are supported. Weak RSA keys, incompatible key usages and malformed
or mixed website/seal QCStatements are rejected.

## Replacement and rejected-update recovery

Import v2 into a new `credentials-v2` directory using the same explicit trust
file. Validate every bound hostname. Renewed certificates may use the same key
or a new dedicated key. Keep v1 protected for rollback. Select chain and key
together with an atomic pointer change (Linux):

```sh
ln -s /etc/qwacamole/credentials-v2 /etc/qwacamole/current.next
mv -Tf /etc/qwacamole/current.next /etc/qwacamole/current
```

On macOS use `mv -fh` instead of GNU `mv -Tf`. Both pointer names must be in the
same directory/filesystem. Serialize updates and never edit live chain/key files
individually. The trust file stays outside this pointer. Selecting a pointer
alone does not change the running signer.

Force Caddy to re-read unchanged paths:

```sh
caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile --force
```

Caddy validates/prepares the candidate before activation, then publishes through
the existing shared binding manager. After a publication failure it continues
using the previous activated credential only while the valid binding covers all
stored TLS leaves. Uncovered TLS leaves still fail closed; new TLS issuance
always requires successful publication. Reload success alone is not activation
proof: make a protected request and confirm `QWAC binding configuration activated`
logs and the current document's signer against the inspected fingerprint.

For the running agent, use its private local control operation:

```sh
qwacamole-agent replace-qwac -config /etc/qwacamole/agent.json
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
```

The daemon reads its **startup-configured** paths, validates the candidate, then
signs/verifies/durably publishes retained TLS coverage before switching signer.
QWAC-only replacement does not reload Nginx. Editing the CLI's config file does
not reload the daemon; keep stable paths or use the documented full-stop migration
for deployment changes. After success, keep the selected inputs on disk: restart
loads those credentials and recovers the same binding/journal and TLS history.

A rejected candidate or failed publication preserves the previous in-memory
signer and committed binding. Restore the pointer atomically to v1 **before
restart**, then reload Caddy or retry the agent operation. Diagnose the reported
key/profile/hostname/chain or storage/signing error and re-inspect after repair.
After an uncertain outcome, confirm the active public signer and TLS coverage
before another update. Old bindings expire normally; failure cannot extend signed
validity. If the gate closes on expiry, repair inputs/publication and verify it
reopens. Do not delete retained TLS coverage or reconstruct the deployment journal.

Changing authorities is a separate explicit trust/deployment decision. Online
replacement remains same-authority even if the new chain contains another root.
Import cannot migrate trust. Plan a separately reviewed new deployment/state and
browser trust for an authority migration.
