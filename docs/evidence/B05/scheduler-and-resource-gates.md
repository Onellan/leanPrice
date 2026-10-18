# B05 — Source governance, durable jobs and resource backpressure
**Status:** implemented locally; enabled-source and Pi acceptance open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-026/029/031–033/038; NFR-001/005/006/010/012; AC-S/O/R families.

## Implemented scope
- Sources default disabled and must be assessment-approved and explicitly enabled before reservation, job claim or fetch (`internal/storage/sources.go`, `internal/fetch/`).
- SQLite-backed bounded queue with dedupe, leases, retry/dead states and stale lease recovery (`internal/storage/jobs.go`, `internal/jobs/`).
- One in-process worker/scheduler and persisted per-source rate/backoff state; job processing is deferred under resource pressure (`internal/jobs/scheduler.go`).
- cgroup v2 and Go runtime memory readings, bounded resource governor with hysteresis, and deferral callbacks (`internal/ops/memory.go`, app wiring).

## Local negative coverage and result
- `go test -count=1 ./internal/jobs ./internal/ops ./internal/storage`: PASS.
- Tests cover unapproved/disabled source rejection, duplicate jobs, bounded attempts/dead lettering, restart lease behavior, source interval persistence and memory-gate hysteresis/fail-closed reads.
- No approved source was enabled. No actual Pi pressure experiment, multi-workload host attribution or long-run queue profile was run; certification remains BLOCKED.
