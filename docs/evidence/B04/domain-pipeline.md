# B04 — Bounded fetch, extraction, pricing and identity
**Status:** local deterministic pipeline implemented; retailer-backed acceptance blocked  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-002–005/008/010–019; NFR-006/007; AC-M/P/S families; ADR source and domain decisions.

## Implemented scope
- Bounded HTTP client with exact approved-host policy, pinned DNS connection, redirect revalidation, timeout, body/decompression caps, retry classification and source rate reservations (`internal/fetch/`).
- Exact integer minor-unit money; rational dimension-safe quantity/pack count; public, member/conditional, multibuy, coupon and stock-state domain representation (`internal/pricing/`).
- JSON-LD/HTML parser with field provenance and digest, bounded inputs and conflict/missing-offer semantics (`internal/extract/`).
- GTIN sanity plus brand/line/variant/quantity checks, hard physical mismatch and auditable decisions (`internal/match/`).
- Refresh service stores each attempt and leaves prior history immutable; only valid exact identity permits observed offers (`internal/catalog/`, `internal/storage/`).

## Local negative coverage and result
- `go test -count=1 ./internal/fetch ./internal/extract ./internal/match ./internal/pricing ./internal/catalog`: PASS.
- `make test-contract GO=/tmp/leanprice-toolchains/go1.27.2/bin/go`: PASS.
- `make test-fixtures GO=/tmp/leanprice-toolchains/go1.27.2/bin/go`: PASS.
- Fixtures cover ambiguous/invalid money, rational quantity mismatch, wrong variant despite GTIN, structured/HTML conflict, missing or oversized data, unknown offer eligibility, rejected host and failed refresh preserving history.
- These are synthetic/sanitized deterministic fixtures. No retailer adapter or genuine retailer fixture is approved; real-source parser coverage and independent review remain blocked.
