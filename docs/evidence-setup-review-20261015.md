# Setup review fixes — 2026-10-15

Both review findings were reproduced against PR #9 head
`3014c52df805dcc6f196be35f7f96b80658f0da2`. The fixes are in production commit
`551959d481845acc7f1f5a6afcf041b62d1286b9`. Earlier acceptance reports remain
historical evidence for their recorded sources and packages.

## Findings and changes

On a fresh Ubuntu 24.04 amd64 container, both unchanged setup commands returned
success under `umask 077`, but created their systemd service files as `0600`.
The agent also created the Nginx prefix and HTTP-01 webroot as `0700`. Identical
repeat setup failed for both integrations, and a real challenge request handled
by Nginx's `www-data` workers returned HTTP 403. See the
[before-fix result](evidence/setup-review-20261015/before.json).

Both commands now explicitly apply the intended permissions after exclusive
creation, then verify the resulting deployment before reporting success. The
agent also handles newly created parent directories, including `/var/lib/nginx`.
Only paths created by that invocation are chmodded; existing operator files,
directories and state retain the existing refusal/preservation behavior. The
process-wide umask is never changed.

The generated Nginx setup binds `0.0.0.0:80` and `0.0.0.0:443`. Its quickstart
now requires a direct public IPv4 A record, IPv4 reachability and no AAAA record.
IPv6 requires a separately configured and tested deployment.

## Restrictive-umask regression

Built Linux amd64 agent and Caddy binaries using Go 1.27.1,
`GOEXPERIMENT=nojsonv2`, and the independently pinned production module graph.
The shared-core pin remains `b3fb517849451b5f201a5ad6682ed7516d9ab858`.
Generated fresh synthetic credentials through the existing development CLI;
authority selection stayed explicit. No generated keys or profiles are committed.

```sh
docker build --platform linux/amd64 -t qwacamole-setup-review:20261015 \
  -f devtools/operator-acceptance/Dockerfile .
python3 devtools/operator-acceptance/run_setup_umask.py \
  --image qwacamole-setup-review:20261015 \
  --agent .local/setup-review/fixed-agent --caddy .local/setup-review/fixed-caddy \
  --fixtures .local/setup-review/pki --output .local/setup-review-ci
```

All four combinations passed: agent and Caddy, each under `077` and `0777`.
Each mask uses a fresh disposable Ubuntu 24.04 amd64 container, with the Nginx
parent absent initially. Assertions cover every planned mode, identical repeat
setup preserving contents/modes/ownership/timestamps and retained state, refusal
of operator content edits, and refusal of altered existing service permissions.
The agent additionally starts the generated Nginx configuration and requires a
real HTTP 200 with the complete challenge body through `www-data` workers.
The fixed results are recorded in
[agent 077](evidence/setup-review-20261015/agent-077.json),
[agent 0777](evidence/setup-review-20261015/agent-0777.json),
[Caddy 077](evidence/setup-review-20261015/caddy-077.json), and
[Caddy 0777](evidence/setup-review-20261015/caddy-0777.json).

The portable integration workflow now runs this same regression using its
production agent and xcaddy-built plugin-only binary, and uploads the sanitized
JSON results. The container has real Nginx, systemd validators and Certbot 5.8.0;
it does not emulate a full systemd boot. Service boot, renewal and reboot evidence
in the earlier report is not claimed as rerun by this focused regression.

## Other validation

- `make agent-build plugin-build build`: passed; plugin module/build metadata
  inspected, production dependency boundaries preserved.
- `make test` and `scripts/check-independent-modules.sh`: four-module race tests
  and dependency boundaries passed, including independently pinned modules with
  workspace/replacements disabled.
- `python3 -m unittest scripts/test_release_agent.py scripts/test_release_publication.py`:
  all 28 tests passed.
- Website check, production build, search/link/publication-boundary validation
  and all 12 Playwright tests passed.
- Sequential native Nginx Chrome 155.0.8059.40 and Firefox 158.0 integration runs
  passed, including real Certbot issuance/renewal, retained TLS streams, cached
  discovery, same/new-key credential replacement and agent restart. Each browser
  used fresh isolated state through the existing harness. The first Firefox
  157.0.1 attempt timed out creating its WebDriver session while the installed
  browser launched its pending updater. The rerun used the already staged stock
  Firefox 158.0 executable directly, without applying that update or changing
  browser assertions. Both attempt logs are retained locally.
- `git diff --check`: passed.

The immutable remote agent and plugin installations also passed:

```sh
scripts/check-remote-agent.sh 551959d481845acc7f1f5a6afcf041b62d1286b9
scripts/check-remote-install.sh 551959d481845acc7f1f5a6afcf041b62d1286b9
```

Both resolve `v0.0.0-20261015112725-551959d48184` for their respective nested
module, with the unchanged core pin above. The scripts used fresh module caches
and existing authenticated Git access; binary metadata contains no replacements
or development dependencies.

The installed remote agent also passed the complete real Nginx/Certbot portable
smoke, including renewal, WebSocket/stream/upload behavior, replacement,
signing/publication/reload failures, signed expiry and recovery:

```sh
bin/qwacamole nginx-smoke -agent-binary bin/qwacamole-agent-remote \
  -state .local/setup-review-remote-nginx
```

The installed remote plugin passed its complete real ACME smoke as well:

```sh
bin/qwacamole plugin-smoke -plugin-binary bin/caddy-remote \
  -state .local/setup-review-remote-plugin
```

This covered two automatic TLS renewals with retained connection coverage,
WebSocket/streaming/header preservation, cached discovery after signed expiry,
same/new-key replacement, rejected reloads preserving service, actual expiry
rejection/recovery, process restart, multiple sites and offline retirement.

At production commit `551959d481845acc7f1f5a6afcf041b62d1286b9`, the hosted
[candidate package workflow](https://github.com/cztamas/qwacamole/actions/runs/38727480156)
and [documentation workflow](https://github.com/cztamas/qwacamole/actions/runs/38727480144)
passed. The new restrictive-umask step in the
[portable integration workflow](https://github.com/cztamas/qwacamole/actions/runs/38727480153)
also passed with its xcaddy-built plugin-only binary; the remaining portable
integration steps were still running when this result was recorded. Checks for
subsequent evidence-only commits are reported separately on the PR.

Raw local evidence is under `.local/setup-review-*.log`,
`.local/setup-review-ci/`, and `.local/setup-review-nginx-*/`. These are local
execution records, not published release artifacts.
