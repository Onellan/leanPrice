# B07 — Watches, alert episodes and notification delivery
**Status:** local AC-A01–A16 coverage implemented and tested; source-backed AC-A06, independent review and real-endpoint certification remain open  
**Candidate SHA:** `d8c58f19eded6ed787996b5422c96beeebf29e15`  
**Authority:** FR-021–029; NFR-007/011/012; AC-A01–A16; U-03/U-08/U-11–U-14.

## Implemented locally
- Versioned positive ZAR threshold and 1–99% relative-drop criteria, any/all combination, public/member/multibuy controls, pause/resume, same-listing comparable baseline and re-arm logic (`internal/watches/`, `internal/storage/watches.go`).
- Eligibility requires exact identity and fresh valid offers; unknown stock is qualified, and ordinary threshold prices are not described as sales (`internal/watches/`, `internal/alerts/`).
- Alert episode and durable outbox are transactionally linked and deduped; webhook sender uses bounded requests and separate exact notification target rules (`internal/storage/`, `internal/notify/`).
- Each watch stores an in-app or configured-webhook preference; edits append an immutable rule version and stale edits conflict. Legacy rules retain the prior instance-wide delivery behavior. Create/edit shows a read-only snapshot of currently valid eligible offers and never sends a notification. Settings exposes a deliberate test action only when a webhook is configured. POST requires session, origin and CSRF validation; it sends fixed `type=test`, no price claim, uses a fresh idempotency key and returns a generic redacted outcome.
- Failed notification deliveries use bounded retries and become terminal after five attempts. The activity timeline now shows each delivery's persisted status and redacted reason/HTTP status, including terminal failures.

## Local evidence
- `go test -count=1 ./internal/watches ./internal/alerts ./internal/notify ./internal/httpui ./internal/storage`: PASS.
- Tests exercise stale/unknown/mismatch/conditional suppression, missing relative history, same-listing baseline, multi-buy basket criteria, episode dedupe, webhook retry classification and test payload/route redaction.
- `go test -race -count=1 ./internal/integration`: PASS. The synthetic two-listing journey verifies the durable alert/outbox path across process restart, unchanged-price dedupe, recovery/re-arm and two simulated webhook deliveries without network access.
- Focused alert, UI, app and storage suites cover preference persistence, immutable version updates, stale-edit conflicts, eligible-offer preview filtering and destination integrity. No production webhook is configured; webhook endpoint configuration remains instance-level configuration.
- `go test -count=1 ./internal/storage ./internal/httpui ./internal/integration`: PASS after adding AC-level pause/stale-rule guards, concurrent episode dedupe, five-attempt terminal delivery, activity visibility and cached-price restart cases.
- `go test -race -count=1 ./internal/storage ./internal/integration`: PASS for the concurrency, outbox and restart paths.

## Acceptance coverage
| Case | Local evidence | Remaining boundary |
|---|---|---|
| AC-A01 | Default public single-pack watch and threshold evaluation: `TestProductAndWatchFormsPersistExactLocalState`, `TestEvaluateThresholdDoesNotCallOrdinaryPriceASale` | None locally |
| AC-A02 | Fresh exact qualifying evidence creates a versioned episode: `TestEngineCreatesDurableInAppEpisodeFromFreshExactEvidence` | No approved retailer end-to-end run |
| AC-A03–A04 | Restart dedupe, recovery and re-arm: `TestAlertJourneySurvivesRestartDedupesAndRearms` | No approved retailer end-to-end run |
| AC-A05 | Invalid identity and stale/ineligible lineage are suppressed: `TestEvaluateEnforcesIdentityFreshnessStockAndEligibility`, `TestAlertEpisodeAndOutboxAreIdempotentAndRequireFreshExactLineage` | Real parser/source fixture remains blocked |
| AC-A06 | Disabled source cannot create an alert; fetch Retry-After is bounded: `TestAlertCreationRejectsPausedAndStaleWatchEvidence`, `TestRetryAfterAndErrorMessagesAreBounded` | Source-specific 429-to-activity journey waits for written source approval and an adapter |
| AC-A07 | 429 delivery uses bounded Retry-After and stable idempotency: `TestDeliveryServicePersistsSuccessAndRetryWithoutSensitiveErrors` | Real endpoint test remains open |
| AC-A08 | Fifth failed attempt becomes dead; redacted status appears in Activity: `TestDeliveryOutboxDeadLettersAfterFiveFailedAttempts`, `TestActivityPageShowsRedactedNotificationDeliveryFailure` | Independent UX/security review remains open |
| AC-A09 | Disabled/paused sources stop scheduling; paused or superseded watch evidence is rejected: `TestDailySchedulingOnlyEnqueuesApprovedWatchedListings`, `TestAlertCreationRejectsPausedAndStaleWatchEvidence` | In-flight work requires target/source operational review |
| AC-A10 | Explicit authenticated, CSRF-protected, redacted test action: `TestTestNotificationIsExplicitAuthenticatedCSRFProtectedAndRedacted`, `TestWebhookTestUsesFixedNonAlertPayloadAndSameTargetPolicy` | Real endpoint test remains open |
| AC-A11 | A 25-hour cached offer remains ineligible after database restart: `TestRestartDoesNotTreatOldCachedPriceAsFreshAlertEvidence` | Target-Pi restart/soak evidence remains open |
| AC-A12 | Member-only offers are suppressed unless enabled: `TestEvaluateEnforcesIdentityFreshnessStockAndEligibility` | None locally |
| AC-A13 | Rule versions are immutable; superseded rule evidence cannot create a new episode: `TestWatchRuleUpdatesAppendVersionAndRejectStaleEdits`, `TestAlertCreationRejectsPausedAndStaleWatchEvidence` | Independent alert review remains open |
| AC-A14 | Twelve concurrent evaluations create one logical episode/outbox row: `TestConcurrentAlertEvaluationsCreateOneEpisodeAndOutboxRow` | Target-Pi contention evidence remains open |
| AC-A15 | Unknown stock can trigger a price-only alert only with the explicit disclaimer: `TestEvaluateUnknownStockHasHonestPriceOnlyDisclaimer`, `TestEngineCreatesDurableInAppEpisodeFromFreshExactEvidence` | No approved retailer end-to-end run |
| AC-A16 | Explicit multibuy uses full qualifying basket total/quantity: `TestEvaluateMultibuyUsesCompleteBasketTerms` | Retailer promotion semantics need an approved source fixture |

No production webhook or live retailer request was used. The matrix records deterministic local coverage and does not close independent review, source approval, Pi certification or live endpoint gates.
