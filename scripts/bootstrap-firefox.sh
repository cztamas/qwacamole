#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools/bin
if [ ! -x .tools/bin/geckodriver ]; then
  case "$(uname -s)-$(uname -m)" in
    Darwin-arm64) archive=geckodriver-v0.37.1-macos-aarch64.tar.gz; digest=d02b3f7003f999caf90974a2ef5da0286c05d01cee19112c86846d759fdba4f5 ;;
    *) echo 'Install geckodriver 0.37.1 and pass -geckodriver /absolute/path; the bundled bootstrap supports macOS arm64.' >&2; exit 1 ;;
  esac
  curl -fsSL "https://github.com/mozilla/geckodriver/releases/download/v0.37.1/$archive" -o ".tools/$archive"
  echo "$digest  .tools/$archive" | shasum -a 256 -c -
  tar -xzf ".tools/$archive" -C .tools/bin
fi
