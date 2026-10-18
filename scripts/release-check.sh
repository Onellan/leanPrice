#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_bin=${GO:-go}
if [ -n "${GOFMT:-}" ]; then
  gofmt_bin=$GOFMT
elif [ "${go_bin%/go}" != "$go_bin" ]; then
  gofmt_bin="${go_bin%/go}/gofmt"
else
  gofmt_bin=$(command -v gofmt || true)
fi
if [ -z "$gofmt_bin" ] || [ ! -x "$gofmt_bin" ]; then
  echo 'gofmt is unavailable for the selected Go toolchain.' >&2
  exit 1
fi
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
cd "$root"

run() {
  label=$1
  shift
  printf '\n== %s ==\n' "$label"
  "$@"
}

printf '%s\n' 'Running local release gates. External approvals and hardware are reported separately.'
unformatted=$($gofmt_bin -l $(find cmd internal migrations web -type f -name '*.go' -print))
if [ -n "$unformatted" ]; then
  printf 'Go files require gofmt:\n%s\n' "$unformatted" >&2
  exit 1
fi
printf '\n== gofmt check ==\n%s\n' 'PASS'
run 'go vet' "$go_bin" vet ./...
run 'staticcheck' "$go_bin" run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
run 'all Go tests' "$go_bin" test -count=1 ./...
run 'domain contracts' env GO="$go_bin" ./scripts/test-contract.sh
run 'fixture suite' env GO="$go_bin" ./scripts/test-fixtures.sh
run 'SQLite backup and restore drill' env GO="$go_bin" ./scripts/backup-drill.sh
run 'ARM64 binary build' env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$go_bin" build -trimpath -o "$temporary/leanprice-linux-arm64" ./cmd/leanprice
run 'Compose validation' docker compose config --quiet
run 'ARM64 image build and locked frontend audit' docker buildx build --platform linux/arm64 --load -t leanprice:release-check .
run 'Go vulnerability scan' "$go_bin" run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
run 'Go dependency license report' "$go_bin" run github.com/google/go-licenses@v1.6.0 report ./...

blockers=''
if [ ! -f LICENSE ]; then blockers="$blockers\n- Owner has not selected the project license (LICENSE is absent)."; fi
if [ ! -f docs/evidence/B00/pi-baseline.md ]; then blockers="$blockers\n- Target Raspberry Pi baseline and cgroup verification are absent."; fi
if [ ! -f docs/evidence/B08/approvals-and-adapters.md ]; then blockers="$blockers\n- Dated source approvals and permitted adapter evidence are absent."; fi
if [ ! -f docs/evidence/B06/independent-ux-review.md ]; then blockers="$blockers\n- Independent rendered UI and accessibility review is absent."; fi
if [ ! -f docs/evidence/B09/pi-soak.md ]; then blockers="$blockers\n- Actual ARM64 Pi workload, 24-hour soak and memory evidence are absent."; fi
if [ ! -f docs/evidence/B09/independent-final-review.md ]; then blockers="$blockers\n- Independent final security, architecture and operations acceptance is absent."; fi
if [ -n "$blockers" ]; then
  printf '\nCERTIFICATION BLOCKED: local gates passed, but the following evidence is missing:'
  printf '%b\n' "$blockers"
  exit 2
fi
printf '\nAll local and external release evidence is present. Independent review of the evidence is still required.\n'
