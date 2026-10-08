# Redacted runtime and browser summaries — 8 October 2026

See [the acceptance report](../../evidence-agent-serving-20261008.md) for scope,
immutable sources, versions, authentication and environment limits.

These summaries come from actual installed-agent Nginx runs and fresh Caddy
runtime/browser runs. `commands.json` records arguments, source commits, exit
statuses and completion times. `installation.json` retains production binary
metadata and its SHA-256. Runtime files retain each executed assertion and held
connection fingerprints. Origin metadata retains the synthetic authority, URI
and trusted forwarding values; cookie values are replaced by a Secure flag.
Migration summaries retain interruption/retry/journal results and artifact
preservation counts, without key contents or private-key hashes.

Native Nginx summaries retain each initial/renewed/replaced identity, unchanged
trust digest and same-profile assertion. Chrome summaries select native binding
and ordinary TLS verifier results and root-store versions. Firefox summaries
select native qualified UI results and confirm that certificate overrides and
`accept_insecure_certs` are disabled. Caddy summaries retain lifecycle/fixture
results, held-connection evidence and stable cache discovery.

No full browser profiles, NetLogs, signed documents, private keys, credentials,
request bodies or raw sensitive browsing logs are committed. The underlying
isolated runtime state remains local under ignored `.local/` directories. A
Chrome expiry incompatibility retains its actual failing fixture and nonzero
command status; it is not converted into a passing result.
