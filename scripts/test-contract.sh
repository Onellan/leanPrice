#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_bin=${GO:-go}
if ! find "$root/internal" -type f \( -path '*/pricing/*_test.go' -o -path '*/match/*_test.go' -o -path '*/watches/*_test.go' \) -print -quit | grep -q .; then
  printf 'No pricing, matching, or watch contract tests exist yet; B04/B07 remain outstanding.\n' >&2
  exit 1
fi
cd "$root"
"$go_bin" test ./internal/pricing ./internal/match ./internal/watches
