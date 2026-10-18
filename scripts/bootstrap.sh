#!/bin/sh
set -eu

go_bin=${1:-go}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if ! command -v "$go_bin" >/dev/null 2>&1; then
  "$root/scripts/install-go.sh"
  go_bin="$root/.tools/go1.27.2/bin/go"
fi
actual=$($go_bin version)
case "$actual" in
  *"go1.27.2"*) ;;
  *) printf 'Expected Go 1.27.2, found: %s\n' "$actual" >&2; exit 1 ;;
esac
"$go_bin" mod download
printf '%s\n' "$actual" 'Go module dependencies downloaded.'
