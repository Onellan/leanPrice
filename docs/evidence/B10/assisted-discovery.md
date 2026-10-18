# B10 — Assisted discovery
**Status:** bounded core contract implemented; user workflow/provider absent  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-006–007; AC-M08/M09; U-01/U-02; ADR-012.

## Implemented locally
`internal/discovery` bounds provider count/results/page size, validates candidate URLs, deduplicates normalized candidates and marks every result unverified. Provider errors do not become local catalog errors. Tests in `internal/discovery/service_test.go` pass with `go test -count=1 ./internal/discovery`.

## Not implemented / blocker
No assessed provider, opt-in settings/quota UI, search provider, or candidate-review/confirm UI exists. Provider selection and privacy/spend policy require an explicit operator/product decision. Search candidate data cannot create an offer or alert. B10 is partial, not accepted.
