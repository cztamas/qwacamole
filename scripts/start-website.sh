#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
website_dir="$repo_root/website"
IFS= read -r required_node < "$website_dir/.node-version"
port="${PORT:-4321}"

usage() {
  printf 'Usage: %s [--port PORT]\nDefault: http://127.0.0.1:4321/; PORT also overrides the default.\n' "$0"
}
setup() {
  printf 'Setup commands:\n  fnm install %s\n  fnm use %s\n  npm --prefix %q ci\n' "$required_node" "$required_node" "$website_dir" >&2
}
fail() {
  printf 'Error: %s\n' "$1" >&2
  setup
  exit 1
}
while (($#)); do
  case "$1" in
    --port)
      if (($# < 2)); then usage >&2; exit 2; fi
      port="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; printf 'Error: unknown argument %s\n' "$1" >&2; exit 2 ;;
  esac
done
if [[ ! "$port" =~ ^[0-9]{1,5}$ ]] || ((10#$port < 1 || 10#$port > 65535)); then
  printf 'Error: port must be an integer from 1 to 65535.\n' >&2
  exit 2
fi
port="$((10#$port))"
command -v node >/dev/null 2>&1 || fail "Node $required_node is required but node is not on PATH."
actual_node="$(node --version)"
[[ "$actual_node" == "v$required_node" ]] || fail "Node $required_node is required; found $actual_node."
command -v npm >/dev/null 2>&1 || fail 'npm is missing; install the complete pinned Node distribution.'
cd "$website_dir"
if [[ ! -f node_modules/astro/bin/astro.mjs ]] || ! node --input-type=module -e '
  for (const dependency of ["astro/config", "@astrojs/starlight", "remark-parse", "remark-stringify", "unified", "unist-util-visit"])
    await import(dependency);
' >/dev/null 2>&1; then
  fail 'Website dependencies are missing or incomplete. Run the setup commands below.'
fi
printf 'Starting qwacamole documentation at http://127.0.0.1:%s/\nLive reload is enabled. Stop with Ctrl+C.\n' "$port"
# Stay in the foreground, including when Astro detects an agent environment.
exec npm run dev -- --port "$port" --ignore-lock
