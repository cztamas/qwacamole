#!/bin/sh
# Run from an already checksum-verified extracted package. Never replace files.
set -eu
[ "$#" = 2 ] && [ "$1" = --prefix ] || { echo 'Usage: ./install.sh --prefix /absolute/empty/install/directory' >&2; exit 1; }
case "$2" in /*) ;; *) echo 'Absolute prefix required' >&2; exit 1;; esac
package_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
prefix=$2
[ ! -e "$prefix" ] && [ ! -L "$prefix" ] || { echo 'Prefix already exists; select a new directory. Existing installations are never overwritten.' >&2; exit 1; }
case "$(uname -s)-$(uname -m)" in Linux-x86_64) target=linux-amd64;; Linux-aarch64|Linux-arm64) target=linux-arm64;; Darwin-arm64) target=darwin-arm64;; *) echo 'Unsupported platform' >&2; exit 1;; esac
case "$package_dir" in *-"$target") ;; *) echo 'Package does not match this platform' >&2; exit 1;; esac
umask 022
mkdir -m 0755 "$prefix"
trap 'rm -rf -- "$prefix"' EXIT HUP INT TERM
mkdir -m 0755 "$prefix/bin" "$prefix/share"
if [ -f "$package_dir/qwacamole-agent" ]; then binary=qwacamole-agent; else binary=caddy-qwac; fi
install -m 0755 "$package_dir/$binary" "$prefix/bin/$binary"
for file in "$package_dir"/*; do
 [ "$(basename "$file")" = "$binary" ] || install -m 0644 "$file" "$prefix/share/$(basename "$file")"
done
if [ "$binary" = qwacamole-agent ]; then "$prefix/bin/$binary" version -json; else "$prefix/bin/$binary" qwac-version --json; fi
trap - EXIT HUP INT TERM
printf 'Installed at %s/bin/%s\n' "$prefix" "$binary"
