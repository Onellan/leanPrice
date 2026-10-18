# Implementation candidate read-only audit

**Audited code SHA:** `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`  
**Reviewer:** implementation author in this session; this is **not an independent review or acceptance certification**.  
**Authority reviewed:** product PRD/UX/Acceptance, technical architecture/data/ADRs/validation, detailed roadmap and execution tracker, README/agent guidance, source assessments, notices, code standards and batch evidence.

## Findings

### AUDIT-P0-01 — Pi-hosted memory and release certification are unproven
**Status:** BLOCKED (external prerequisite), not a failed workstation test. **Evidence:** `make release-check` completes local gates and exits 2; `make pi-baseline` exits 2 without `PI_SSH_TARGET`. No target Raspberry Pi or existing workload measurements are available.  
**Impact:** NFR-001, AC-R01–R12, B00-S03/B09/B12 cannot be accepted; no R1/R2/release claim is justified.  
**Boundary:** B00/B09/B12. Obtain the target host, run baseline, actual ARM64 workflow and 24-hour soak, measure the cgroup plus attributable host usage and OOM events, and record exact evidence.

### AUDIT-P0-02 — No retailer source is approved; source-backed workflows remain absent
**Status:** BLOCKED (source-owner permission/policy prerequisite). **Evidence:** five dated assessments under `docs/evidence/sources/`; none is approved. Storage and fetch paths reject non-approved or disabled hosts; no live retailer request was made.  
**Impact:** B08 cannot add a truthful retailer adapter. B06 URL-add/preview/confirm/listing attachment and source-backed R1 acceptance are incomplete.  
**Boundary:** B00/B06/B08. Obtain written approval for exact source host, allowed fields, access path and rate; refresh policy evidence; then implement fixtures and adapter through the centralized fetch path. Do not bypass the gate.

### AUDIT-P1-01 — Product and technical work remains partial beyond the local core
**Status:** OPEN. The local executable implements the foundation, SQLite/recovery, authentication and outbound boundaries, deterministic price/extraction/matching, durable scheduling, product/watch workflows, explicit webhook test delivery and bounded CSV/history reporting. The plan/evidence records identify missing approved-source URL add/confirm, source adapters, discovery provider/review UI, source-backed conditional offers, retention controls and optional account/channel work. Bounded keyset pagination, audited source controls, an app-cgroup resource panel, and per-watch notification preferences with versioned edit and no-send offer preview are implemented locally.  
**Impact:** Several planned B06/B08/B10–B13 acceptance cases are not met. No source or provider is available to validate the required retailer semantics.  
**Boundary:** implement safe local-independent functionality where fully specified; leave source-dependent behavior disabled until approvals and product scope are supplied.

### AUDIT-P1-02 — Independent reviewers and project-license decision are absent
**Status:** BLOCKED. **Evidence:** no independent reviewer/acceptance tester or owner-selected repository license is recorded; `make license-report` reports first-party packages as Unknown. `modernc.org/mathutil` is scanner-Unknown but its module LICENSE was manually reviewed as BSD-3-Clause.  
**Impact:** independent B00/B01–B12 acceptance and redistribution sign-off cannot be self-certified.  
**Boundary:** obtain an owner license choice and independent reviewers for the exact candidate before distribution/release.

### AUDIT-P2-01 — Rendered and manual accessibility review remains unverified
**Status:** NOT RUN. Source-level SSR semantics and route tests pass, but no browser-rendered narrow/wide inspection, keyboard/screen-reader walkthrough or independent UX review is recorded.  
**Impact:** UX U-01–U-14 visual/accessibility evidence remains incomplete.  
**Boundary:** browser QA and independent UX review after the source-backed screens exist.

### AUDIT-NOTE-01 — Vulnerability database reports an uncalled module package
`govulncheck` found zero vulnerabilities in imported symbols and one advisory in a required module package not called by this code (`golang.org/x/crypto/openpgp`, `GO-2026-5932`, no fixed version listed at scan time). Record this as an uncalled-module warning; it is not an imported-symbol vulnerability result. Re-scan at release because advisory data changes.

## Local evidence reviewed
- `go test -count=1 ./...`: PASS.
- `go test -race -count=1 ./...`: PASS, including the synthetic alert restart/re-arm integration journey.
- `go vet ./...` and pinned Staticcheck: PASS.
- `make test-contract`, `make test-fixtures`, `make backup-drill`: PASS.
- ARM64 binary/image cross-build and Compose validation: PASS. ARM64 image not run on Pi.
- Deterministic 100-product/500-listing/4,000-observation storage page benchmark: PASS on workstation; not Pi capacity evidence.
- `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go release-check`: local gates pass on audited candidate `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`; exit 2 with external certification requirements listed.
- Local Markdown-link validation and `bash -n scripts/*.sh`: PASS.

No new reproducible code defect was established by this implementer-run audit. The open findings are incomplete product scope or absent external acceptance evidence, not independent passes.
