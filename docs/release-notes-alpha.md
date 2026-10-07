# Public alpha — draft, unpublished

Version and publication date await owner approval. Engineering acceptance is
tracked in the [hardening ledger](public-alpha-plan.md); this draft is not a
release announcement.

The Caddy plugin accepts supplied RSA 2-QWAC credentials and gates ordinary ACME
TLS activation on independently verified, durable binding publication. It now
advertises stable uncached discovery so cached application responses can resolve
fresh bindings after refresh, renewal and same-authority credential replacement.
Immutable artifacts remain separate for durable recovery and temporary history.

Activation commits candidate credentials and complete retained/stored/new TLS
coverage together. Expiry-driven refresh adds bounded retry backoff/jitter;
authenticated immutable-document GC preserves current/staged/grace-protected
artifacts. Enforced certificate-count/document-size capacities refuse publication
rather than drop coverage for held connections. Structured diagnostics expose
expiry, failures, coverage and storage growth.

Migration: legacy numeric durations and immutable manifests remain supported,
with additive `discovery_url`. Omitted lifetime changes from 45s to 1h; lifetimes
below 3s now reject. Purge old cached immutable discovery Links; grace cannot
extend signed expiry. Policy changes require full stop/restart. See
[operations](operations.md) for defaults, failures and full drain/offline retirement.

Scope remains one literal hostname per independent owner, RS256, supplied
credentials and local exclusive storage. Multiple independent owners can share
Caddy. The browser-facing TLS certificate must be covered; arbitrary CDN/TLS
termination is unsupported. No automatic QWAC enrollment, distributed state,
HSM support or authority migration is provided. Synthetic demo authorities do
not confer legal qualification. Strict standard bindings remain default, with
an explicit Firefox compatibility mode. Chromium's known ignored-exp assertion
must remain visible; final dated browser evidence will list exact versions.

Visibility, license, release tags/publication, anonymous availability and Caddy
registration/hosted-builder compatibility remain owner gates in the
[release checklist](alpha-release.md).
