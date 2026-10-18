#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_bin=${GO:-go}
if [ ! -d "$root/testdata/fixtures" ] || ! find "$root/testdata/fixtures" -type f -print -quit | grep -q .; then
  printf 'No fixture files exist yet; source extraction fixtures are part of B04.\n' >&2
  exit 1
fi
if ! find "$root/internal" -type f -name '*_test.go' -print -quit | grep -q .; then
  printf 'No fixture-backed tests exist yet.\n' >&2
  exit 1
fi
cd "$root"
"$go_bin" test ./...
