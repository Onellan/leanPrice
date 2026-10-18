#!/bin/sh
set -eu

version=1.27.2
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
destination="$root/.tools/go$version"
case "$(uname -m)" in
  x86_64) archive="go${version}.linux-amd64.tar.gz"; expected=ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5 ;;
  aarch64|arm64) archive="go${version}.linux-arm64.tar.gz"; expected=94f3e30b8e374bc285e7dadc11e0865726b9bc6e85b841ccceaabc0214c6b7c8 ;;
  *) printf 'Unsupported Linux architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac
mkdir -p "$root/.tools"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
curl -fL "https://go.dev/dl/$archive" -o "$tmp/$archive"
printf '%s  %s\n' "$expected" "$tmp/$archive" | sha256sum -c -
rm -rf "$destination"
mkdir -p "$destination"
tar -xzf "$tmp/$archive" -C "$destination" --strip-components=1
"$destination/bin/go" version
printf '\nPinned Go installed at %s\n' "$destination/bin/go"
printf 'Run make with GO=%s/bin/go or add that directory to PATH.\n' "$destination"
