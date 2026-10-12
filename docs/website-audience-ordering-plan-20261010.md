# Website ordering and audience correction — 10 October 2026

Scope: update open PR #8 from 6525655. Preserve Astro/Starlight design, canonical
manuals, quickstarts and start script. No Go behavior, deployment or account changes.

## Implementation decisions and engineering review

- Keep the deployment job serialized, without cancellation during uploads. Read
  Pages production history inside that job and compare actual Git ancestry through
  GitHub's contents-read API before uploading the checked artifact. Reject unknown
  identities/comparisons. Skip older or already-deployed sources explicitly.
  Compare sources rather than current HEAD equality so Go-only commits do not
  suppress path-filtered website changes. Retained successful production history
  also protects against stale reruns after a dashboard rollback.
- Store canonical product manuals in docs/guides; root docs remain engineering
  plans/reports and maintainer workflows. Publish only explicit manuals plus the
  real SECURITY policy. Keep licence assets allowlisted. Retain detailed browser
  investigations/acceptance procedures in the repository; publish a task-oriented
  demo and concise version/compatibility support guide.
- Review every published page, remove preparation/history/approval prose, preserve
  current private installation constraints, strict defaults and Chromium expiry
  limitation. User upgrades/recovery remain in guides; release/website processes
  stay repository-only.

## Verification gates

Real mocked upload sequence B then delayed A, rerun/manual/first/unknown/API cases;
existing upload/identity/no-retry checks; locked install/check/build, exact route
boundary and search; desktop/mobile quickstarts/navigation/keyboard/code/axe;
script startup/other-directory/ports/shutdown, canonical live reload, production
preview. Dated excluded report distinguishes new proof from inherited native Go
proof. Final diff and PR description reviewed, fixes committed/pushed, PR open.
