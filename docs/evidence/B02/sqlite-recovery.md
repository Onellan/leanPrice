# B02 — SQLite durability and recovery
**Status:** implemented locally; independent data-integrity review and Pi capacity evidence remain open  
**Candidate SHA:** `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`  
**Authority:** NFR-008/014; AC-O01–O06/O10/O11; ADRs and `DATA-CONTRACTS.md`.

## Implemented scope
- Versioned, checksummed forward migrations, constrained schema, WAL mode, foreign keys, indexes and exclusive database process lock (`migrations/`, `internal/storage/migrate.go`, `store.go`).
- Bounded repository reads/writes for catalog, exact source policy, observations/offers, jobs/leases, versioned watches, alert episodes and outbox state.
- Observation and offers persist atomically; source network access remains outside transactions. Alert episode/outbox insertion is idempotent.
- Online SQLite backup, manifest/checksum, integrity validation, compatibility validation, pre-restore safety snapshot and staged replacement/recovery (`internal/storage/backup.go`, `restore.go`).
- `scripts/backup-drill.sh` and `make backup-drill` execute focused backup/restore evidence.
- `internal/storage/store_benchmark_test.go` builds a deterministic local dataset of 100 products, 500 exact listings, and 4,000 observations with one offer per observation (eight of each per listing), then benchmarks 25-row product and listing-history pages both hot and after `PRAGMA shrink_memory`.

## Local negative coverage and result
- `go test -count=1 ./internal/storage`: PASS, including migration checksum/future-version rejection, source-gated leases, atomic immutable observations, idempotent alert lineage, backup tamper rejection, concurrent writes during online backup, restore staging and bounded query indexes.
- `make backup-drill GO=/tmp/leanprice-toolchains/go1.27.2/bin/go`: PASS.
- `go test -run='^$' -bench='^BenchmarkSyntheticPiCatalogPages$' -benchtime=2s -count=2 ./internal/storage` with Go 1.27.2: PASS on a Linux AMD64 workstation. Product pages measured 546.718–551.463 µs/op hot and 623.978–624.984 µs/op after SQLite cache shrink; history pages measured 251.902–252.480 µs/op hot and 278.135–279.173 µs/op after SQLite cache shrink. Product-page allocations were 17,048 B/op (371 allocs); history-page allocations were 20,712–20,784 B/op (584–588 allocs).
- The parallel mixed-read sub-benchmark performs one product page and one history page per operation against the store's serialized SQLite pool: 931.584–935.923 µs per pair, 38,053–38,054 B/op and 961 allocs/op. This is aggregate workstation throughput timing, not a latency percentile or Pi contention result.
- These measurements are workstation-only. `PRAGMA shrink_memory` releases SQLite's page cache; it does not clear the OS cache or simulate cold disk. No target Pi 500-product/2,500-listing footprint/performance run, target disk-failure exercise, contention/whole-host memory certification, or independent review was run. Those remain BLOCKED/NOT RUN rather than passing.

## Target Pi capacity gate
The workstation benchmark establishes a repeatable query baseline only. B02-S04 still needs execution on the target Pi with the planned 500-product/2,500-listing footprint, memory trace, query plans, contention results and disk/recovery checks under representative load before capacity is accepted.

## Review boundary
Implementer-run evidence. Independent storage/integrity reviewer: not appointed. No schema downgrade is supported; recover with a validated backup and compatible prior application image.
