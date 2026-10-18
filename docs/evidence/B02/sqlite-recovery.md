# B02 — SQLite durability and recovery
**Status:** implemented locally; independent data-integrity review and Pi capacity evidence remain open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** NFR-008/014; AC-O01–O06/O10/O11; ADRs and `DATA-CONTRACTS.md`.

## Implemented scope
- Versioned, checksummed forward migrations, constrained schema, WAL mode, foreign keys, indexes and exclusive database process lock (`migrations/`, `internal/storage/migrate.go`, `store.go`).
- Bounded repository reads/writes for catalog, exact source policy, observations/offers, jobs/leases, versioned watches, alert episodes and outbox state.
- Observation and offers persist atomically; source network access remains outside transactions. Alert episode/outbox insertion is idempotent.
- Online SQLite backup, manifest/checksum, integrity validation, compatibility validation, pre-restore safety snapshot and staged replacement/recovery (`internal/storage/backup.go`, `restore.go`).
- `scripts/backup-drill.sh` and `make backup-drill` execute focused backup/restore evidence.

## Local negative coverage and result
- `go test -count=1 ./internal/storage`: PASS, including migration checksum/future-version rejection, source-gated leases, atomic immutable observations, idempotent alert lineage, backup tamper rejection, concurrent writes during online backup, restore staging and bounded query indexes.
- `make backup-drill GO=/tmp/leanprice-toolchains/go1.27.2/bin/go`: PASS.
- No Pi 500-product/2,500-listing footprint benchmark, disk-failure exercise on the target, or independent review was run. Those remain BLOCKED/NOT RUN rather than passing.

## Review boundary
Implementer-run evidence. Independent storage/integrity reviewer: not appointed. No schema downgrade is supported; recover with a validated backup and compatible prior application image.
