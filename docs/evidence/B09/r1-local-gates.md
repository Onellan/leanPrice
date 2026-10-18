# B09 — R1 local release gates and external certification
**Status:** local release gate implemented; R1 certification BLOCKED  
**Candidate SHA:** `5b4b17ec26d0e42a7ce4158b68ba48c476ce2274`  
**Authority:** NFR-001–014; AC-R01–R12 and operational/security acceptance.

## Local results
- `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go release-check`: all configured workstation gates completed successfully on candidate `5b4b17ec26d0e42a7ce4158b68ba48c476ce2274`, then returned exit 2 with external prerequisites listed.
- Passed local gates include format, vet/Staticcheck, Go suites, contract/fixture suites, backup drill, ARM64 cross-build, Compose validation, ARM64 image build, asset audit and vulnerability check.
- ARM64 image cross-build succeeded. An AMD64 workstation container smoke passed health/setup/static-asset paths under inspected 440,000,000-byte memory/swap, 64 PID and non-root settings. ARM64 runtime was not executed.
- `make pi-baseline` without `PI_SSH_TARGET`: exit 2, correctly classified BLOCKED.
- License scan: first-party packages Unknown because owner project license is not selected; `modernc.org/mathutil` scanner Unknown was manually checked against its module BSD-3-Clause LICENSE. No source requests or production webhook were used.

## Release blockers
No target Pi, 24-hour soak, attributable host memory, OOM evidence, approved source end-to-end workflow, independent security/architecture/product/UX review, acceptance tester or selected project license. Do not publish R1/release pass claims until each relevant gate is evidenced.
