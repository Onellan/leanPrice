# leanPrice — Technical Plan Execution Plan
**Version:** 1.3 | **Status:** implementation tracker; local work is progressing, release certification is blocked on external evidence | **Updated:** 2026-10-19
**Authority:** [PRD](../product/PRD.md), [UX](../product/UX-SPEC.md), [Acceptance](../product/ACCEPTANCE.md), [Technical implementation plan](IMPLEMENTATION-PLAN.md), [Architecture](../technical/ARCHITECTURE.md), [Data contracts](../technical/DATA-CONTRACTS.md), [ADRs](../technical/ADRS.md), [Validation](../technical/VALIDATION.md).

## 1. Objective and completion rule
Execute the technical roadmap from B00 through B14 in dependency order while preserving product authority and hard source, security, data and resource gates. This tracker records implementation and evidence; it does not replace source-owner approval, independent review, hardware certification or a product-scope decision.

A slice can be **implemented** with local code and deterministic evidence, but a batch is **accepted** only after its specified independent and/or external gates pass. `BLOCKED` means a real prerequisite is absent. It is never converted to pass by cross-compilation, workstation smoke, implementer review or a plan change. Conditional features require explicit product-scope approval before exclusion.

## 2. Starting point and constraints
- Starting commit on `main`: `03356fdc22cd8be40006dddc5ab51012080a5796` (`docs: align product and technical contracts v1.1`). The initial checkout contained product, technical and agent-guidance documents only.
- Technical roadmap relocated to `docs/plans/IMPLEMENTATION-PLAN.md`; this file is the execution plan. Supporting product and technical authorities remain in their existing folders.
- Go 1.27.2 was installed in `/tmp/leanprice-toolchains/` after checksum verification. Docker/Buildx, Make and a Docker daemon are available. Node is not installed on the host; the pinned Tailwind build runs in the Docker builder.
- Do not fetch retailer pages until a dated assessment and separate source-owner permission approve the exact source. Do not invent adapters or credentials. The five assessed candidates all remain disabled; Takealot is disallowed, Clicks/Makro/Checkers require further review and Dis-Chem requires written consent.
- The owner selected MIT for the project license; the repository records it in [`LICENSE`](../../LICENSE). The owner supplied a target Pi SSH address, but available local keys were rejected and no host baseline has been captured. No production webhook, appointed independent reviewers or external renderer host is available in this workspace.
- Preserve existing CRLF-only edits in product/technical source documents. Do not add GitHub workflows. Network and webhook calls stay outside SQLite transactions.

## 3. Evidence and authority rules
1. Load only the skills triggered by a slice. Source acquisition requires source feasibility and safe HTTP; domain changes require identity/pricing/domain guidance; SQLite/outbox work requires the integrity skill; UI work requires frontend design; release claims require Pi certification and local test-execution guidance.
2. Evidence states are `PASS`, `FAIL`, `BLOCKED`, or `NOT RUN`. A passing unit test proves only its tested contract. Workstation containers do not certify ARM64 hardware or total Pi memory.
3. Each batch record lists authority IDs, files/interfaces, negative cases, exact commands actually run, result, implementer/reviewer boundary, and remaining risk. The earlier full benchmark/audit base is code SHA `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`; the latest implementation candidate is recorded in the validation section. Later documentation/evidence commits must not be mistaken for the code candidate.
4. Independent review and acceptance evidence must come from people other than the implementer. This session has no delegated reviewer; it records implementer checks honestly and leaves independent gates open.
5. Never change source approval, project license, deployment, household-account scope or remote-renderer policy by inference.

## 4. Dependency order and remaining delivery
1. Close B00 owner/reviewer/Pi decisions; keep live retrieval disabled.
2. Complete local foundation and domain batches B01–B07, with deterministic negative coverage and operational artifacts.
3. Implement B08 adapters only after source permission; complete B09 on the actual Pi with independent security, UX and acceptance review.
4. Implement provider-backed B10 discovery and complete B11 promotion/history/channel work only after relevant source/provider approvals and product decisions; certify together in B12.
5. Deliver B13 approved-source expansion and any explicitly authorized account scope only after privacy/security review. B14 remains conditional on an operator-owned separate host and policy review.

## 5. Batch ledger
| Batch | Current state | Implemented locally | Acceptance blocker / next action |
|---|---|---|---|
| B00 feasibility | Partial; independent sign-off open | Requirement map and five dated candidate assessments | No approved source, no target Pi baseline, no independent product/source-policy sign-off. Preserve stop gates. |
| B01 foundation | Implemented; local gates pass | Go/Chi lifecycle, SSR/HTMX, Tailwind build, ARM64 image/Compose, quality scripts | Target-Pi run and rendered browser/accessibility review remain open. |
| B02 SQLite/recovery | Implemented locally; workstation synthetic benchmark passes | Versioned schema, bounded stores, transactions, WAL backup/restore and recovery; 100-product/500-listing/4,000-observation benchmark | Independent integrity review and target Pi 500-product/2,500-listing footprint, performance and recovery execution remain open. |
| B03 authentication/security | Implemented; local tests pass | Setup/login/session/password reset, CSRF/CSP, trusted proxy handling, SSRF-safe fetch and separate webhook policy | Independent security review and full adversarial release review remain open. |
| B04 pricing/extraction/matching | Implemented for deterministic contracts; approved-source flow absent | Cents/rational units, offer eligibility, bounded parser/provenance, identity decisions and refresh service | Approved real source fixtures and independent parser/domain review remain blocked. |
| B05 jobs/source governance | Implemented locally; operational certification open | Registry default-off, durable leases/queue, scheduler and memory governor | Source-enabled long run, load/performance study and Pi memory evidence remain blocked. |
| B06 product workflows | Partial | Catalog/search, identity edit/audit, product detail, current-offer rules, observed range, watches, source/activity views, bounded pagination, audited source controls and app cgroup status | Approved-source URL add/preview/confirm and final responsive/browser/accessibility review remain open. |
| B07 alerting | Local AC-A01–A16 deterministic coverage implemented; independent and external acceptance open | Versioned absolute/relative rules, dedupe/outbox/retry, pause/resume, versioned edit, no-send offer preview, explicit test payload, delivery status in Activity, stale-cache restart guard, concurrent episode dedupe and deterministic drop/recovery/re-arm flow | Source-specific AC-A06, independent alert/security/UX review and real webhook endpoint test remain open. |
| B08 source adapters | Blocked | Source registry and assessment evidence only | No candidate is approved. Obtain written permission and dated technical approval before implementing or fetching. |
| B09 R1 release | Local gate implemented; certification blocked | Release script runs local gates; ARM64 build/image and workstation smoke pass; owner-selected MIT license recorded | Actual ARM64 Pi test/soak, approved-source E2E, independent reviews, complete redistribution evidence and release sign-off. |
| B10 assisted discovery | Partial | Bounded provider registry, candidate cap/dedupe and unverified candidate semantics | No approved provider, opt-in settings, provider implementation or candidate-review UI. |
| B11 promotions/history | Partial | Rational quantity/unit-cost domain, conditional offer fields, sampled 30-day range, bounded server-rendered chart and keyset-paginated history | Source-backed member/multibuy/coupon eligibility and added channels need approved evidence/product scope. |
| B12 R2 certification | Blocked | No integrated R2 certification claim | Depends on complete B10/B11, Pi workload, source refresh and independent review. |
| B13 expansion/reporting | Partial | Bounded formula-safe validated-history CSV and price sample summary | Additional approved sources, owner-defined retention/deletion policy and any household-account authorization/privacy review remain outstanding. |
| B14 remote renderer | Blocked/conditional | No remote rendering code or service | Requires operator-owned separate host, explicit retailer policy, privacy/security protocol and a scope decision. |

Batch evidence records are in [`docs/evidence/`](../evidence/). The implementer-run read-only review for code SHA `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c` is [here](../evidence/reviews/implementation-candidate-audit.md); it is not an independent review. Evidence status is authoritative at the slice level; the summary above does not upgrade a partial batch into an accepted one.

## 6. Implemented slice register and local evidence

### B00 — Feasibility and product traceability
- `B00-S01`: requirement/acceptance mapping implemented in `docs/evidence/B00/requirements-map.md`; product/design sign-off is still independent and open.
- `B00-S02`: candidate assessment documents exist for Takealot, Clicks, Makro, Checkers and Dis-Chem. They do not grant permission. All live fetches and adapters remain blocked.
- `B00-S03`: the target host was supplied, but its existing workload has not been measured. A batch-mode SSH attempt failed with `Permission denied (publickey,password)` and wrote no baseline. The read-only `scripts/pi-baseline.sh` accepts optional validated `PI_SSH_PORT` (1–65535) and `PI_SSH_IDENTITY_FILE` settings; baseline and workload gates remain blocked until an authorized SSH identity is available.

### B01 — Foundation
- `B01-S01–S04`: implemented in `cmd/`, `internal/app`, `internal/httpui`, `web/`, `Dockerfile`, `compose.yaml`, `Makefile` and `scripts/`.
- Local gate commands and their historical checkpoint detail: [`B01 evidence`](../evidence/B01/toolchain-and-foundation.md). B01 evidence is a dated foundation checkpoint, not the final release review.

### B02 — SQLite and recovery
- `B02-S01–S03`: migrations, schema constraints, store operations, WAL snapshot, manifest validation and restore safety-copy/recovery are implemented in `migrations/` and `internal/storage/`.
- `scripts/backup-drill.sh` runs focused backup/restore integrity tests; `go test ./internal/storage` covers repository, migration and recovery behavior. Local PASS; independent review and Pi disk/footprint gates open.
- `B02-S04`: `internal/storage/store_benchmark_test.go` passes a deterministic workstation benchmark at 100 products, 500 listings, 4,000 observations and 4,000 offers. Product page observed ranges were 546.718–551.463 µs/op hot and 623.978–624.984 µs/op after SQLite cache shrink; history pages were 251.902–252.480 and 278.135–279.173 µs/op. Parallel mixed reads measured 931.584–935.923 µs per product/history pair. These are not cold-disk or Pi results. The planned 500-product/2,500-listing Pi workload, memory trace, representative contention, recovery and independent integrity review remain open.

### B03 — Authentication and security boundaries
- `B03-S01–S04`: setup, Argon2id admin credentials, sessions, login throttling, reset/audit, mutation CSRF, response security headers, trusted proxies, hardened fetch client and separate exact webhook policy are implemented in `internal/auth`, `internal/fetch`, `internal/notify` and `internal/httpui`.
- Local adversarial tests cover session/CSRF, SSRF, private address and redirect handling. Implementer tests pass; independent security review is open.

### B04 — Price, extraction, identity and observations
- `B04-S01–S05`: bounded HTTP policy, exact money/quantity, offers, tri-state stock, JSON-LD/HTML extraction provenance, GTIN/variant conflict decisions and append-only observations are implemented in `internal/fetch`, `internal/pricing`, `internal/extract`, `internal/match`, `internal/catalog` and `internal/storage`.
- Local package/contract/fixture suites pass. No retailer-specific fixtures or approved-source refresh have been added; source-backed coverage is BLOCKED.

### B05 — Queue, schedule and resource governance
- `B05-S01–S04`: registry approval gates, SQLite job leases/recovery, single-worker scheduler and cgroup/Go memory governor are implemented in `internal/storage`, `internal/jobs`, `internal/ops` and app wiring.
- Local tests pass. Pi pressure/soak, enabled-source quotas and attributable host-memory tests remain NOT RUN/BLOCKED.

### B06 — Product/operator UX
- `B06-S01`: SSR shell, local assets, semantic forms/tables and full-page/HTMX response paths implemented.
- `B06-S02`: local product create/search/edit are implemented, including bounded keyset pagination. URL-add preview/confirm and listing attach are NOT IMPLEMENTED because no source is approved; local product entry is not retailer verification.
- `B06-S03`: exact identity edit/audit, current validated offers, safe retailer links, history and observed price-range views are implemented and locally tested.
- `B06-S04`: dashboard, source status, activity and CSV navigation exist. Source pause and approval-gated enable record source/admin events. Settings shows app cgroup current/threshold/hysteresis. Product, activity and history views use bounded keyset pagination. The memory panel is not whole-Pi evidence.
- Frontend source review and route tests pass; browser-rendered keyboard/mobile/accessibility evidence and independent UX review are open.

### B07 — Watches, episodes and delivery
- `B07-S01`: immutable versioned watch rules support positive ZAR target, 1–99% relative drop, any/all combination, eligibility flags, exact/fresh offer checks and pause/resume.
- `B07-S02`: durable alert episode/outbox, idempotency, bounded retries and a fixed test webhook payload use the separate notification target policy.
- `B07-S03`: create/pause, immutable versioned edit, per-watch in-app/configured-webhook preference, no-send validated-offer preview, offers/deals/settings and test-notification action are implemented. Test delivery requires an explicit authenticated CSRF-protected POST; success uses a fixed `type=test` payload; errors do not reveal raw transport details. Legacy watches preserve the prior instance-wide destination. The Activity page now exposes delivery status and redacted failure reasons.
- `B07-S04`: `internal/integration/alert_journey_test.go` uses two synthetic exact listings to verify one qualifying drop, durable webhook outbox delivery, restart, ten unchanged-price rechecks without duplicates, recovery above target, a second drop/re-arm and a second delivery. A separate restart case rejects a 25-hour cached offer. No network call is made.
- Deterministic AC-A01–A16 coverage is recorded in [B07 evidence](../evidence/B07/watch-and-notification-workflows.md). Focused storage/UI/integration and race suites pass. Source-specific AC-A06, independent review, target-Pi contention/restart evidence and real endpoint test remain open.

### B08–B14 — Conditional and external batches
- B08 is blocked by written source authorization and feasibility. No adapter should be fabricated to create superficial coverage.
- B09 local `make release-check` passes its workstation/build/container gates and then returns status 2 with the missing external evidence list. It is not a release certification.
- B10's core candidate registry is bounded and treats all results as unverified; approved provider implementations and review UX remain absent.
- B11 has exact rational unit costs, conditional offer data, a sampled observed range, bounded history scatter chart and keyset-paginated history. Retailer-specific promotion proof and extra channels remain absent.
- B12 is blocked on B10/B11 completion plus target hardware and independent review.
- B13 CSV export is implemented and tested for exact validated public history, row/field caps, formula injection, secret/URL exclusion and streaming; additional coverage/account work is not complete. Retention controls require an owner-defined retention window and deletion policy that preserves active-alert and audit evidence.
- B14 has no renderer implementation. Feasibility beyond architecture requires a user-supplied host and policy/privacy/security approvals.

## 7. Validation record
Latest implementation candidate: `d8c58f19eded6ed787996b5422c96beeebf29e15`. The release-check local gates and focused storage/integration race suite pass on this candidate; its release-check returns status 2 for external prerequisites. The earlier candidate `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c` has the full-repository race result and synthetic storage benchmark. Results and limits are recorded in [B09 evidence](../evidence/B09/r1-local-gates.md) and [B02 SQLite evidence](../evidence/B02/sqlite-recovery.md).

Local workstation gates actually executed during this work:
- `go test -count=1 ./...` using `/tmp/leanprice-toolchains/go1.27.2/bin/go`: PASS (Go package suites, including UI, notify, storage, jobs, pricing, parser and identity).
- `go vet ./...` and pinned Staticcheck: PASS.
- `make test-contract`, `make test-fixtures`, and `make backup-drill`: PASS.
- `make build-arm64`: PASS; generated ELF for Linux/AArch64.
- `docker compose config --quiet`: PASS.
- `docker buildx build --platform linux/arm64 --load -t leanprice:local .`: PASS. ARM64 image is cross-built, not run on Pi.
- Local AMD64 container smoke (separately built for workstation): liveness/readiness/setup and local asset responses pass under inspected Docker settings; not Pi or ARM64 runtime evidence.
- `go test -run='^$' -bench='^BenchmarkSyntheticPiCatalogPages$' -benchtime=2s -count=2 ./internal/storage` with Go 1.27.2: PASS on workstation (exact workload and observed timing/allocation ranges are in [B02 evidence](../evidence/B02/sqlite-recovery.md)); this does not certify target capacity.
- `go test -race -count=1 ./...`: PASS on earlier code candidate `eece378259fe7e0f3f5cf0be8a9289ea2f7d090c`, including the synthetic alert restart/re-arm journey. Focused storage/integration race suites also PASS on `d8c58f19eded6ed787996b5422c96beeebf29e15`.
- `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go release-check`: all configured local gates PASS on implementation candidate `d8c58f19eded6ed787996b5422c96beeebf29e15` with the MIT-bearing working tree; script exits status 2 for the external gates listed in B09 evidence.
- `make pi-baseline`: exits status 2 / BLOCKED with no `PI_SSH_TARGET`. A later attempt against the owner-supplied target reached SSH but failed batch-mode authentication (`Permission denied (publickey,password)`); no baseline evidence was written.
- At the earlier release-check, before the owner selected the project license, the report marked first-party packages and `modernc.org/mathutil` Unknown. The dependency module license was manually reviewed as BSD-3-Clause; the follow-up scan after adding MIT is recorded in B09 evidence.
- After MIT was added, `make GO=/tmp/leanprice-toolchains/go1.27.2/bin/go license-report` passed and identified first-party packages as MIT. `modernc.org/mathutil` remains scanner-Unknown and manually verified as BSD-3-Clause; the remaining scanner limitations and release-check outcome are recorded in [B09 evidence](../evidence/B09/r1-local-gates.md).
- `govulncheck`: zero vulnerabilities in imported symbols; one advisory is for an uncalled `golang.org/x/crypto/openpgp` package. This is recorded as an uncalled-module warning, not an imported-symbol finding.
- No live retailer page was requested and no production webhook was configured.

## 8. External blocker register
| Gate | Needed evidence / owner | Current result |
|---|---|---|
| Product/source approval | Product owner/legal approval for exact retailer/source/fields/rate and scope | Five assessments, zero approved sources; B08 blocked |
| Target Pi | Access to target Pi and other workloads; OS/cgroup/Docker facts; 24h soak | Host target supplied; batch-mode SSH key authentication is unavailable; B00-S03/B09/B12 blocked |
| Independent review | Independent product, security, architecture, UX and acceptance reviewers on exact candidate SHA | Not appointed in this session; acceptance not self-certified |
| Repository license | Record owner-selected project license and verify dependency notices/report | MIT selected in `LICENSE`; `make license-report` recognizes first-party packages as MIT; `modernc.org/mathutil` scanner-Unknown is manually verified as BSD-3-Clause |
| Search provider | Explicitly selected/authorized provider and quota/privacy decision | None configured; B10 provider path incomplete |
| Remote renderer | Separate operator-owned host, allowed use, protocol and privacy approval | None supplied; B14 blocked/conditional |
| Webhook | Operator destination and secret provisioning | No production target; local mock tests only |
| Visual/manual accessibility | Browser available for narrow/wide rendering, keyboard and assistive review | No browser-based independent review recorded |

## 9. Remaining implementation and acceptance work
These are explicit roadmap items, not implied passes:
- Complete approved-source URL add/preview/confirm after an eligible source exists. Pagination, source controls, app cgroup status and watch preview/preferences are implemented locally; browser/accessibility review remains open. Complete conditional-promotion handling only after real source semantics are approved.
- Add assessed retailer adapters only for written-approved sources; refresh policy/fixture evidence before every release.
- Complete assisted-discovery provider configuration/review, history graph and any channel extensions only with explicit provider/product approval.
- Run independent batch/final code, product, security, UX and acceptance reviews against a fixed SHA.
- Run ARM64 Pi memory, recovery, performance, existing-workload and 24-hour soak gates; publish no R1/R2 claim before results.
- Re-run the MIT license report when dependencies change and include the current notices before publishing an image/source release.

## 10. Slice record template
Use one record per slice in the corresponding batch evidence file:
```markdown
### LP-Px-Byy-Szz — <name>
Status: planned | implemented | blocked | independently reviewed | accepted
Candidate SHA:
Authority: FR/NFR, AC, U, ADR
Files and interfaces:
Migration / rollback:
Security, privacy, source-policy and resource effect:
Negative cases and local commands actually run:
Results and evidence paths:
Independent reviewer / acceptance tester:
Open blockers:
```

## 11. Current checkpoint
All repository plans are under `docs/plans/`; product authority remains under `docs/product/` and technical contracts under `docs/technical/`. Local implementation spans foundation, storage, security, pricing, scheduling, catalog/watch workflows and reporting. Workstation code gates and the synthetic SQLite benchmark pass, and the owner-selected MIT license is recorded and recognized by the license report. The current checkpoint is **not a completed product plan or release**: approved-source workflows, source/provider-backed promotion and discovery work, retention/account/channel scope, independent reviews and Pi evidence remain outstanding. Keep all retailer sources disabled and continue only within those boundaries.
