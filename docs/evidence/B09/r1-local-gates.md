# B09 — R1 local release gates and external certification
**Status:** local release gate implemented; R1 certification BLOCKED  
**Validated code candidate SHA:** `d8c58f19eded6ed787996b5422c96beeebf29e15`  
**Authority:** NFR-001–014; AC-R01–R12 and operational/security acceptance.

## Local results
- Original pre-license `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go release-check`: all configured workstation gates completed successfully on earlier candidate `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`, then returned exit 2 with external prerequisites listed. The latest implementation and MIT-bearing run is recorded below.
- Passed local gates include format, vet/Staticcheck, Go suites, contract/fixture suites, backup drill, ARM64 cross-build, Compose validation, ARM64 image build, asset audit and vulnerability check.
- ARM64 image cross-build succeeded. An AMD64 workstation container smoke passed health/setup/static-asset paths under inspected 440,000,000-byte memory/swap, 64 PID and non-root settings. ARM64 runtime was not executed.
- `make pi-baseline` without `PI_SSH_TARGET`: exit 2, correctly classified BLOCKED.
- At the original scan, first-party packages were Unknown because the project license had not yet been selected; `modernc.org/mathutil` scanner Unknown was manually checked against its module BSD-3-Clause LICENSE. No source requests or production webhook were used.
- The deterministic 100-product/500-listing/4,000-observation workstation benchmark passes; results and limitations are recorded in [B02 SQLite evidence](../B02/sqlite-recovery.md). It is not target Pi capacity evidence.
- `go test -race -count=1 ./...`: PASS on candidate `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`.
- Follow-up `go test -race -count=1 ./internal/storage ./internal/integration`: PASS on implementation candidate `d8c58f19eded6ed787996b5422c96beeebf29e15`, including concurrent episode creation, stale-cache restart, terminal outbox and activity visibility cases.

## Follow-up after MIT selection and AC coverage — 2026-10-19
- `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go license-report`: PASS (exit 0) on the MIT-bearing working tree. First-party packages are recognized as MIT. `modernc.org/mathutil` remains scanner-Unknown and is manually verified as BSD-3-Clause in [third-party notices](../../technical/THIRD-PARTY-NOTICES.md). `go-licenses` also reports that some assembly dependencies cannot be fully inspected and identifies the unversioned local module URL as `HEAD`; these are recorded scanner limitations, not a missing owner license choice.
- `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go release-check`: all configured local gates, including the license report, PASS on the MIT-bearing working tree containing implementation candidate `d8c58f19eded6ed787996b5422c96beeebf29e15`. The command returns exit 2 only for these external prerequisites: target-Pi baseline/cgroup verification; dated source approval and permitted-adapter evidence; independent rendered UI/accessibility review; ARM64 Pi workload/24-hour soak/memory evidence; and independent final security/architecture/operations acceptance. The run used the same implementation tree later committed at this SHA; remaining changes in the working tree were documentation/license records.
- The project-license blocker is closed. A read-only Pi baseline attempt against the owner-supplied target failed at SSH authentication with `Permission denied (publickey,password)`; no host evidence was written. Pi hardware/soak, permitted-source end-to-end evidence, independent reviews and acceptance, search-provider choice, remote-renderer host/policy, webhook target and rendered accessibility review remain open as listed in the execution plan.
- B00 collector follow-up (2026-10-19): `sh -n scripts/pi-baseline.sh` passes. Invalid `PI_SSH_IDENTITY_FILE` and out-of-range `PI_SSH_PORT=65536` inputs are rejected with exit 2 before any network request. A recognized SSH identity is still required for host evidence.

## Release blockers
No accessible target-Pi baseline, 24-hour soak, attributable host memory, OOM evidence, approved-source end-to-end workflow, independent security/architecture/product/UX review or acceptance tester. MIT is selected and recorded; do not publish R1/release pass claims until remaining relevant gates are evidenced.
