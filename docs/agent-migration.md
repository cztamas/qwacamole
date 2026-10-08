# Agent configuration versions and offline migration

This procedure upgrades merged `main` at
`4a93ce6c43bf4a728a69625ac7ed567d54db53f1` to the configurable agent. It also
changes compatible `serving` settings in existing versioned deployments without
rebuilding certificate/binding state. See [the agent guide](agent-nginx.md) for
field defaults and [current evidence](evidence-agent-serving-20261008.md) for
executed tests and immutable sources.

## Formats and compatibility

| Artifact | Current format | Supported legacy |
|---|---|---|
| Operator JSON | `config_version: 1` | Omitted/0 accepted as legacy input; new state is still versioned |
| `format.json` | `state_version: 1`, `nginx_format_version: 1` | Absent only for the merged unversioned deployment |
| `deployment.json` | `state_version: 1` | Missing/0 version with the merged journal schema |
| Complete Nginx config and generation `https.conf` | `# qwacamole nginx-format: 1` | Exact merged-main templates retained by the migration verifier |
| Shared binding storage | Existing signed manifest/document schema | Existing verified recovery; no migration rewrite |

Unknown future input, state or journal versions reject without upgrading or
mutating them. Do not remove version fields to force compatibility. A missing
format marker with existing deployment ownership is recognized as legacy and
requires migration, rather than being silently adopted. Unmanaged includes,
modified generated files, malformed journals, unknown fields, mismatched
key/certificate pairs or missing binding coverage reject. Exact template checks
remain an ownership check **within** explicitly identified supported formats.

Certificate deployment/renewal uses the existing controlled validation/reload and
fresh TLS-confirmation path. Same-path supplied QWAC replacement uses
`replace-qwac` and does not reload Nginx. Serving settings affect request behavior,
not certificate identity, retained binding coverage or generation ownership; they
can therefore migrate in place with every artifact retained. This version's
supported setting-change tool is offline: top-level settings and all recoverable
generation templates change together, and old workers must not retain the prior
policy. A manual live edit/reload is unsupported. All new serving fields are
supported by this migration; none requires certificate-state reconstruction.

Hostname, state/runtime ownership, Certbot lineage, listener/probe addresses,
credential/trust paths, application origin, Nginx paths/binary, binding profile
and binding policy must stay identical in this command. Such changes need their
own operator plan and verification; the command rejects them rather than guessing
which old workers, authorities, artifacts or control endpoints remain relevant.
Do not use state reconstruction as a generic workaround. Offline **certificate
retirement**, which deliberately starts new coverage only after complete shutdown,
is a separate operation described in the agent guide.

## Full stop and preparation

The agent cannot prove that all old Nginx workers or authenticated connections
have drained. The CLI flag is an **operator assertion**, not an authentication or
drain detector. Certificate expiry, elapsed grace, a closed listening port,
successful reload and termination of only the newest worker are insufficient.

1. Schedule downtime. Keep the old binary, old JSON and complete top-level Nginx
   configuration. Take a restricted consistent backup of the whole state,
   supplied QWAC credentials/trust, Certbot account/lineage state and configuration.
   The automatic migration backup contains the agent state and generated top-level
   config; it does not copy separately managed QWAC or Certbot directories.
2. Stop renewal timers and other deployment automation, then the agent service.
3. Fully stop the dedicated Nginx master **and every old worker**, including workers
   left by earlier graceful reloads. Use the owning service manager's control
   group and process inventory to confirm exit of every owning process. For the
   guide's services, stop `certbot.timer`, `qwacamole-agent.service` and
   `qwacamole-nginx.service`, then inspect the recorded service control group
   with `systemctl show` / `systemd-cgls`. Confirm the group is empty; inspect any
   manually launched owning processes separately. If graceful shutdown waits on
   streams, apply your approved termination procedure and verify process exit.
   Do not infer this from a listener check. Preserve the private runtime directory
   or recreate it with its original owner and 0700 mode for migration's lock.
4. Install the new immutable binary. Keep all existing paths and non-serving
   settings. Add `config_version: 1` and the desired `serving` object to the
   root-owned configuration. Do **not** render over the old Nginx configuration:
   migration first needs to authenticate that source configuration.

For the example paths, after completing these checks:

```sh
umask 077
install -d -m 0700 /run/qwacamole-example
qwacamole-agent migrate -config /etc/qwacamole/agent.json -full-stop-confirmed
qwacamole-agent validate -config /etc/qwacamole/agent.json
systemctl start qwacamole-nginx.service qwacamole-agent.service
qwacamole-agent reconcile -config /etc/qwacamole/agent.json
qwacamole-agent status -config /etc/qwacamole/agent.json
qwacamole-agent check -config /etc/qwacamole/agent.json
# Restart renewal automation only after confirmed operation:
systemctl start certbot.timer
```

`ready: true`, a reconciled journal and successful `check` are necessary. Native
browser verification is separate. If no retained leaf is currently valid,
migration still retains history; restart remains fail-closed in `needs_deploy`.
Obtain/deploy a fresh valid Certbot candidate through the ordinary binding-first
path. Never erase expired retained leaves to permit restart.

## Staging, interruption and rollback

Migration acquires the exclusive state and private runtime locks, validates
original ownership, journal meaning, keys/chains and signed retained coverage,
and creates a private `.migration-backup-*` directory. Regular backup files are
0600 and directories 0700; the backup retains immutable binding artifacts,
manifest, TLS keys/history, journal and original `top-level.conf`. Socket/lock
resources are excluded. Keep this backup until the new deployment is accepted.
It is not automatically collected as a binding document.

Before activation, `migration.json` durably records the backup and desired
configuration. New templates are staged under `.migration-stage-*` and checked
with actual Nginx `-t`. Only owned configuration files, journal version and format
metadata are replaced. The include pointer, journal phase/IDs/timestamps,
certificate/key bytes and binding bytes stay intact. Expired generation chains
are checked at a historical time for integrity; that check does **not** authorize
TLS activation. Restart/deploy still require current validity and normally
verified fresh TLS confirmation.

`migration.json` prevents startup throughout activation, including if format
metadata has already been written. No migration step starts or reloads Nginx.
After final configuration validation, removing/syncing the intent marker completes
the migration. A failed staged validation or interrupted process leaves a
recoverable backup and blocked startup. Fix the cause and rerun **the same**
command/config after confirming the full stop again. The retry uses the original
backup and refuses changed TLS/binding payloads. Repeated completed execution is
supported, preserving journal meaning and history; it may create another
restricted backup. Do not manually remove intent to bypass recovery.

For rollback before accepting new deployments, fully stop everything again and
restore a consistent original snapshot with its old JSON, old binary and
`top-level.conf`. Restore the backup's complete state at its original path,
excluding transient runtime locks/sockets, and reconcile. The new versioned state
is **not** claimed to be readable by the merged old binary. Once new certificate
or binding generations have been deployed, rolling back to the earlier snapshot
can lose their coverage; use a separately planned consistent restoration and full
stop, not selective copies. HSTS already stored in browsers survives this rollback.
