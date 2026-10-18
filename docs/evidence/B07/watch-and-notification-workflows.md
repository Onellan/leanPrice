# B07 — Watches, alert episodes and notification delivery
**Status:** partial implementation; versioned preferences and preview/edit flow implemented locally; full acceptance and independent review open  
**Candidate SHA:** `5b4b17ec26d0e42a7ce4158b68ba48c476ce2274`  
**Authority:** FR-021–029; NFR-007/011/012; AC-A01–A16; U-03/U-08/U-11–U-14.

## Implemented locally
- Versioned positive ZAR threshold and 1–99% relative-drop criteria, any/all combination, public/member/multibuy controls, pause/resume, same-listing comparable baseline and re-arm logic (`internal/watches/`, `internal/storage/watches.go`).
- Eligibility requires exact identity and fresh valid offers; unknown stock is qualified, and ordinary threshold prices are not described as sales (`internal/watches/`, `internal/alerts/`).
- Alert episode and durable outbox are transactionally linked and deduped; webhook sender uses bounded requests and separate exact notification target rules (`internal/storage/`, `internal/notify/`).
- Each watch stores an in-app or configured-webhook preference; edits append an immutable rule version and stale edits conflict. Legacy rules retain the prior instance-wide delivery behavior. Create/edit shows a read-only snapshot of currently valid eligible offers and never sends a notification. Settings exposes a deliberate test action only when a webhook is configured. POST requires session, origin and CSRF validation; it sends fixed `type=test`, no price claim, uses a fresh idempotency key and returns a generic redacted outcome.

## Local evidence
- `go test -count=1 ./internal/watches ./internal/alerts ./internal/notify ./internal/httpui ./internal/storage`: PASS.
- Tests exercise stale/unknown/mismatch/conditional suppression, missing relative history, same-listing baseline, multi-buy basket criteria, episode dedupe, webhook retry classification and test payload/route redaction.
- `go test -race -count=1 ./internal/integration`: PASS. The synthetic two-listing journey verifies the durable alert/outbox path across process restart, unchanged-price dedupe, recovery/re-arm and two simulated webhook deliveries without network access.
- Focused alert, UI, app and storage suites cover preference persistence, immutable version updates, stale-edit conflicts, eligible-offer preview filtering and destination integrity. No production webhook configured. Remaining AC-A01–A16 coverage, independent alert review and real endpoint test remain open; webhook endpoint configuration remains instance-level configuration.
