# leanPrice — Detailed Technical Implementation Plan
**Version:** 1.0 | **Status:** implementation-ready planning baseline, **not executed**  
**Planning basis:** Product PR #1 was merged into `main` at commit `15c53baff00a7eaffa028ceb5e9d73938fb6ff28` **before this technical plan was authored**.  
**Product contract:** [PRD](../product/PRD.md) · [UX](../product/UX-SPEC.md) · [Acceptance](../product/ACCEPTANCE.md)  
**Technical contracts:** [Architecture](ARCHITECTURE.md) · [Data/API](DATA-CONTRACTS.md) · [ADRs](ADRS.md) · [Local Validation](VALIDATION.md)  
**Implementation environment:** Go + Chi + `html/template` + HTMX + Tailwind CSS + SQLite, one Docker/ARM64 container on Raspberry Pi 4B. No local browser, separate database or local SearXNG.

# A. Executive delivery strategy

## A1. Mandate and design guardrails
Build an **original leanPrice application**, not a PriceBuddy port. The product watches specific comparable products across assessed South African retailers, saves durable history and notifies users only when reliable fresh pricing qualifies. Product correctness, ethics and resource safety are release blockers.

**Non-negotiable NFR-001:** at every supported workflow (including search, scrape, notification, backup, startup, error retry and upgrade), total attributable Pi-hosted RAM must stay below **500,000,000 bytes**. Default technical implementation begins with a **440 MB decimal Docker container hard limit**, with no additional local service and separately measured host overhead. Any failure/OOM is a release blocker, not permission to raise limits. Normal-operation engineering target <=350 MB peak, subject to evidence.

**No GitHub CI/CD:** all builds, static analysis, security, integration, accessibility, performance, Pi certification and deployment validation execute locally. Do not add/edit `.github/workflows/*`. Do not claim checks ran until scripts are implemented and actual result recorded.

**No unverified retail claims:** Checkers, Pick n Pay, Shoprite, Clicks, Dis-Chem, Makro, Game and Takealot are candidate integrations only. Source discovery and legal/technical feasibility decide the first two adapters; no bypassing CAPTCHA/rate limits/login.

**No architecture debt as a speed shortcut:** price parsing and matching errors can create misleading financial decisions, so fail-closed behavior is mandatory. Every slice contains tests, rollback and relevant observability. Favor readable domain-specific packages over generic heavy frameworks.

## A2. Release map and batch dependency DAG
```text
PRD/UX/Acceptance merged to main [DONE: docs only]
  |
  +-- P0-B00 Source / design / Pi feasibility evidence
  +-- P0-B01 Build + runtime scaffold + local gates
          |
          +-- P0-B02 SQLite schema/store/backup
          +-- P0-B03 Security/session/SSRF
          |        |
          +--------+--> P0-B04 Fetch/extraction/matching
                            |
                       P0-B05 Durable jobs/resource governor
                            |
                       P0-B06 Product/offer/health UI
                            |
                       P0-B07 Watches/alerts/outbox
                            |
                       P0-B08 Approved retailer integrations
                            |
                       P0-B09 MVP certification/release
                            |
                +-----------+---------------+
                |                           |
          P1-B10 Discovery              P1-B11 Promotions
                |                           |
                +-----------+---------------+
                            |
                       P1-B12 R2 certification
                            |
                       P2-B13 Expanded sources/analytics
                            |
                       P3-B14 Optional remote renderer/evolution
```
B00 and B01 may run concurrently with clear contracts; B02 and B03 can follow B01 in parallel; remaining batches depend on the specified verified interface/schemas. Retailer feasibility may force a documented MVP scope adjustment before B08.

## A3. Execution model — Priority → Batch → Slice
- **Priority (P0 / P1 / P2 / P3)** groups user value and risk. P0 is MVP, P1 discovery and richer comparisons, P2 broader source and analysis capabilities, P3 optional extensions.
- **Batch** is a review/certification boundary containing multiple coherent slices.
- **Slice** is an independently testable vertical or architectural change with concrete files, behaviors, acceptance IDs and local validation. Name format `LP-P0-B02-S03`.
- **Multiple slices per coding-agent run are allowed** when dependencies are met and validation is individually recorded; do not create one artificial run per slice.
- **Three lanes:** (A) implement independent slices, (B) continuous lightweight architecture/security/quality checks on each slice, (C) independent integrated review only at batch close and full final certification. Blocking findings become remediation slices in that batch.
- **Continuous guardrail PC-B10 analogue:** every slice checks data correctness and memory implications; any new fetching/parser/queue/UI path must prove bounded allocation; any new price/offer field must preserve eligibility/freshness and provenance.
- **Context efficiency:** agent tasks consume this plan's relevant batch, current code contracts, targeted files, current tests and precise acceptance IDs. Avoid re-reading all docs in every small slice. Batch reviewer loads wider architecture and cross-cutting risks.
- **Branch workflow:** feature branches to PRs; prefer small reviewed commits; no push to `main` that bypasses current local gates. Documentation is not evidence of runnable software.

## A4. Ownership / review roles
Product owner: scope/claim decisions; Architect: interface and resource enforcement; Go/backend lead: services/schema; Scraping lead: source approvals and extraction fixtures; UX/design lead: screen/state/accessibility; Security reviewer: auth/SSRF/dependencies; Test/SRE: local gates, ARM64 profiling, backup and release. Roles may be staffed by fewer people, but batch/final independent review must be performed by someone other than the primary implementer.

## A5. Core initial module map
```text
cmd/leanprice
internal/app,auth,httpui,catalog,sources,discovery,fetch,extract,
internal/retailers,match,pricing,watches,notify,jobs,storage,ops
web/templates/{layouts,partials,pages}
web/static/{css,js}
migrations
testdata/fixtures
scripts
docs/evidence
```
App dependencies flow *handlers → services → repository interfaces → SQLite adapters*; source fetcher never calls notifications; rule evaluator never fetches network. Refer to ARCHITECTURE.md for contracts.

# B. P0 — MVP foundation, trust and on-device certification

## LP-P0-B00 — Risk and source feasibility (G0)
**Objective:** prevent building retailer promises on untested assumptions, baseline real Pi footprint and lock product semantics.  
**Dependencies:** merged PRD/UX/Acceptance on main.  
**Exit:** feasibility register, sanitized fixtures, CPU/RAM baseline, ADR deltas and release-compatible scope recorded.

**S01 — Traceability and architecture feasibility**
- Deliver: `docs/evidence/B00/requirements-map.md`, architecture dependency map, requirement/acceptance matrix with priority owner/target batch; confirm decisions in ADR-001–014.
- Review conflicting semantics: “on sale” vs “below threshold,” manual URLs and approved domains, stock unknown, relative drop, exact match; reconcile before coding.
- Tests: desk-check every M/AC critical requirement has a batch and measurable gate; every domain/route represented.
- Done: unresolved decisions registered with stop/go owner and no hidden product claims.

**S02 — Candidate-retailer compliance and technical spike**
- Target candidate sources selected by objective likelihood, not a promise of specific retailers. Inspect published terms/robots/policy, public endpoint/JSON-LD/HTML, geographic prices, login/membership, paging and rate limits.
- Create two distinct sanitized fixture suites if permitted; record all attempted source capabilities, negative cases and parser evidence. Output `docs/evidence/sources/<key>-assessment.md` with approved/unsupported/disallowed.
- Explore representative household products incl Domestos exact variant, different sizes and promotion terms.
- Stop criteria: if no compliant HTTP extraction for two sources, choose transparent manual-URL/structured-data MVP and submit scope change to PRD; **do not** add stealth browser bypass.

**S03 — Pi baseline / memory capacity plan**
- Capture actual Pi OS/cgroup mode/available RAM/Docker versions, existing workload memory, swap behavior and storage. No code release claim.
- Model cgroup 440 MB limit and 60 MB host-overhead reserve; identify other services that might contend.
- Output `docs/evidence/B00/pi-baseline.md`, measurement script outline and config assumptions; verify how Pi's Docker parses memory units.
- Gates: NFR-001, AC-R01/R10 measurement methodology approved; resource budget is provisional until runnable code.

**B00 certification:** product, source-policy and architecture reviewers approve feasible MVP assertions; record unknown coverage explicitly.

## LP-P0-B01 — Repository scaffold and reproducible local toolchain (G2 part 1)
**Dependencies:** B00 architecture inputs; no retailer fetching yet.  
**Exit:** ARM64 single Go web service, compiled static assets, no Node.js runtime, local quality gate scripts.

**S01 — Go module, Chi bootstrap and lifecycle**
- Create `go.mod`, `cmd/leanprice/main.go`, `internal/app/config.go`, `internal/app/lifecycle.go`, `internal/httpui/router.go`.
- Structured config schema, safe defaults, `log/slog`, graceful `SIGTERM`, request IDs, context deadlines, `/health/live` and `/health/ready` states.
- Gate: unit config tests, graceful shutdown/health integration, `go vet`; no unbounded goroutines or ad hoc globals.

**S02 — SSR layout, HTMX, Tailwind, initial design tokens**
- Create `web/templates/layouts`, `web/templates/partials`, `web/static`, pinned HTMX asset and compiled Tailwind CSS using **builder-only** tooling, accessible layout and functional no-JS navigation.
- Implement placeholder pages for dashboard/products/sources/settings that state “not implemented” accurately.
- Gate: keyboard/mobile smoke, CSP, no CDN/network frontend requirement, SSR/HTMX full-page compatibility. UX U-06.

**S03 — Secure ARM64 image and Compose baseline**
- Multi-stage Dockerfile and `compose.yaml`; CGO-free build, unprivileged user, writable `/data`, CA certs, memory cap **440m decimal**, matching swap max, pids cap, stop-old-first update script.
- `.env.example`, healthcheck, persistent volume, clean permissions, no privileged/docker-socket mounts. No second runtime service.
- Gate: `docker inspect` cgroup enforcement, ARM64 build/run on Pi, no OOM startup, offline local SSR smoke. AC-O01/R01.

**S04 — Local quality scripts and docs standard**
- `Makefile` + `scripts/local-gate.sh`, static/vuln/license/fixture targets, evidence template; fail if any executable target missing; document tool install/pins.
- `docs/CONTRIBUTING.md`, code standards (interfaces, error codes, migrations, no network in handlers, bounded reads), PR template outside GitHub workflow tree.
- Gate: actual local `make fmt lint test build-arm64` and evidence; absence of GitHub Actions changes. NFR-013.

**B01 certification:** new contributor can clone, build locally and run a single-container placeholder on Pi under bound; review SSR and security defaults.

## LP-P0-B02 — SQLite, durable model and recovery (G2 part 2)
**Dependencies:** B01.  
**Exit:** real repositories, schema migrations, backups/restore and DB fault tests.

**S01 — Versioned schema/migrations**
- `migrations/0001_core.sql`, `internal/storage/migrate.go` and explicit migration table. Implement retailers, sources, canonical products, listings, observations, offers, watches, rules, episodes, outbox, jobs, admin/session/event tables as reviewed from DATA-CONTRACTS.
- Enforce FK/CHECK/UNIQUE, WAL, busy timeout, safe time representation, integer money; reject unsupported schema upgrade.
- Gate: fresh schema test, FK violations, index/query-plan baseline, checksum/migration idempotency. AC-O01/O02.

**S02 — Store interfaces, queries and transactions**
- `internal/storage/{catalog,observation,queue,watch,source,outbox}_repo.go` plus domain mocks; start `MaxOpenConns(1)`, prepared queries, keyset pagination, small result sets, UTC times.
- Implement atomic observation persist, job claim/lease and episode+outbox transaction; keep network outside transaction. Status transitions validated.
- Gate: transaction cancel/rollback, concurrent claims, no duplicate key races, check constraints, DB_BUSY bounded retry. AC-O04/A14.

**S03 — Backup, retention and restore**
- `internal/storage/backup.go`, `scripts/backup-drill.sh`, documented online snapshot method, checksum manifest, strict permissions, integrity check and restore compatibility/stop-the-world protocol.
- Retention of diagnostics/candidates without losing active alert-linked evidence. Never copy SQLite main DB alone in WAL mode.
- Gate: backup concurrent with write, clean restore and count checks, disk-full/read-only tests, bad schema rollback. AC-O03/O05/O06/O10.

**S04 — SQLite stress/footprint and migrations review**
- Synthetic 100-product/500-listing data seeder, query benchmark, hot/cold cache experiments, FKs and WAL checkpoint. Optimize indexes before changing connection pool size.
- Gate: query plans and memory trace; document actual allocation plus contention results. AC-R07/R08.

**B02 certification:** independent database/data-integrity review, proven restore on local machine; no live retailer integration dependency.

## LP-P0-B03 — Authentication, session and outbound security
**Dependencies:** B01/B02 schema/session support.  
**Exit:** safe admin authentication and hardened fetch policy shared by all adapters.

**S01 — Admin setup and session lifecycle**
- `internal/auth/{setup,password,session}.go`; explicit one-time local setup token, Argon2id with measured bounded memory and single concurrent hashing slot, no default creds.
- HTTP-only/SameSite/Secure-over-HTTPS cookies, login throttling, session rotation/logout, change password and recovery guidance. Audit admin changes.
- Gate: setup replay, rate limiter, session fixation, password redaction, recovery exercise. FR-030, AC-X01/X02/X09.

**S02 — CSRF, CSP, headers, trusted proxy**
- `internal/httpui/middleware.go`, CSRF enforcement on all mutations incl HTMX, output escaping, MIME sniff prevention, CSP, trusted reverse proxy policy; no external JS CDN.
- Gate: blocked unauthenticated/forged HTMX, stored-XSS fixture, host-header attacks, full vs fragment navigation. AC-X03/X04/X07/X08.

**S03 — SSRF-safe outbound HTTP**
- `internal/fetch/{policy,dns,transport,redirect}.go`; source-host allowlist, HTTPS, DNS public IP validation (IPv4/IPv6), redirect rechecks, connector pinned IP approach as appropriate, blocked metadata/CNAME/private and dangerous ports.
- No adapter may instantiate raw arbitrary HTTP client; enforce centralized hardened transport.
- Gate: httptest malicious redirects, DNS rebinding, encoded hosts, scheme violations; never dispatch request to private addresses. AC-M02, AC-X05.

**S04 — Secrets/logging and threat review**
- `internal/app/secrets.go`, redaction helpers, safe debug flags, local security script; document trust boundaries and threat scenarios involving a home LAN.
- Gate: fixture scans for leaked keys, role check, source-data output escaping; independent security review before allowing live fetches. AC-X06/X10–X12.

**B03 certification:** no exposed setup/SSRF path; app remains LAN-safe behind approved proxy and rejects unsafe URLs.

## LP-P0-B04 — Bounded fetch, price extraction, matching (G3 core)
**Dependencies:** B02 repositories and B03 safe transport, B00 source feasibility fixtures.  
**Exit:** deterministic fetch→extract→validate→match→persist pipeline on fixture and approved site.

**S01 — Fetch policy and resource-capped transport**
- `internal/fetch/{client,budget,ratelimit}.go`; context timeout, gzip/deflate safe decompression, **2MiB decompressed** body cap, redirect max 3, controlled cookies, origin-specific rate and backoff, one global fetch semaphore.
- Integrate Colly only if its handlers delegate to same safe transport. Separate source status codes for 403,429,timeout,oversized and unsupported.
- Gate: slow/oversize/decompression/429 fixtures with bounded allocations, no override of robots/terms, no repeated retry loop. AC-S01–S06.

**S02 — Price, unit and offer domain types**
- `internal/pricing/{money,quantity,offer,availability}.go`; integer cents parser for ZAR and locale, rational quantity, expiry, promo classification and in-stock/unknown handling.
- No float money; reject missing/conflicting, zero, negative, cross-currency price; unit-price only for compatible dimensions/pack counts.
- Gate: golden AC-P01–P11/P14 + malformed locale input + overflow/precision checks.

**S03 — Extractor priority and parser contracts**
- `internal/extract/{jsonld,embedded,html,normalize,provenance}.go`; prefer Product/Offer JSON-LD, then assessed embedded structured JSON, then approved CSS selectors via goquery.
- Capture field-level origin, content digest, parser version and conflicts, without retaining unsafe full HTML; return `Extraction` typed statuses.
- Gate: sanitized genuine-like fixtures for valid, missing, contradictory, script-containing, multi-offer, member and oversized markup. AC-P06/P09/P13, AC-S07.

**S04 — Canonical matching and review decisions**
- `internal/match/{normalize,rules,decisions}.go`; exact GTIN subject to sanity, brand, variant and quantity/pack rules; explicit hard mismatch beats fuzzy confidence; manual review.
- Output ranked exact/possible/rejected with rationale; store manual override/audit and preserve pre-decision observation provenance.
- Gate: Domestos 750ml vs 1L/Original/twin pack, ambiguous GTIN and Unicode cases. AC-M04–M07.

**S05 — Observation persistence and failure status**
- Implement integration service `internal/catalog/refresh.go` that makes one bounded request, parser selection, validation, storage and source health updates; failures do not replace last good history.
- Gate: old valid price retained stale, new failure event classified, no invalid alert input. AC-P03/P05, AC-S02/S07.

**B04 certification:** independent fixture review, no source policy bypass, zero false prices in negative suite, bounded response size/parse memory.

## LP-P0-B05 — Job orchestration, source governance and resource backpressure
**Dependencies:** B02/B04.  
**Exit:** automatic durable refresh with no unbounded memory growth.

**S01 — Retailer registry and governance**
- `internal/sources/{registry,assessment,health}.go` models manual/search/special capabilities, assessment state, enabled flags, source rate policy and canonical host allowlist. Default unassessed+disabled.
- Gate: unsupported source cannot fetch or search; disabled source cancels new leases; 403/429 classify accurately. AC-S01/S11/S12.

**S02 — SQLite job queue and restart leases**
- `internal/jobs/{queue,lease,worker}.go`; `queued, leased, retry_at, blocked, dead, completed`, unique dedupe key per schedule bucket, bounded pending queue, atomic lease claim and reclaim.
- Gate: simultaneous “check now” coalescing, SIGKILL/restart, lease expiry and at-most-one active fetch worker. AC-S08–S10/O04.

**S03 — In-process scheduler**
- `internal/jobs/scheduler.go`; daily default per watch with jitter, per-domain tokens, retry-after, administrative pause/force-check, next-run hints and clear source errors.
- Gate: 100-product seeded schedule, hourly/daily time-bound checks, daylight-saving UTC correctness, no per-product goroutine or cron process. FR-026/032–033.

**S04 — Memory resource governor**
- `internal/ops/{memory,limits,metrics}.go`; read cgroup memory.current/events and runtime heap; application soft thresholds/hysteresis pause new discovery/fetch jobs; `GOMEMLIMIT` and bounded pools.
- Gate: pressure simulation defers jobs with `RESOURCE_DEFERRED`, UI still works, no OOM during oversize workload; Pi cgroup cap measured. AC-R05/R06/R11.

**B05 certification:** long-run queue and memory profiles, crash recovery, source-policy limits, no hidden in-memory backlog.

## LP-P0-B06 — Product, comparison and operator UX (G3 UI)
**Dependencies:** B01–B05.  
**Exit:** complete manual URL → confirm → compare/history flow.

**S01 — Design system and page shell**
- `web/templates/{layouts,partials,components}` and compiled CSS tokens; navigation Dashboard/Search/Products/Watches/Deals/Activity/Sources/Settings, mobile/desktop breakpoints, semantic tables, badges and accessible feedback.
- Detailed UI states from UX-SPEC; use “verified”, “stale”, “unavailable”, “unknown” correctly; no colored discounts without evidence.
- Gate: mobile 320/375 px, keyboard, WCAG AA review, HTMX/full-page fallback. U-06/U-07.

**S02 — Product catalog and add URL journey**
- `internal/httpui/{catalog,add_url}.go`, domain use cases, route validation, extraction preview/confirmation, exact-match selection, no partial on cancel, searchable/paginated catalog.
- Gate: <=4 primary actions on valid URL after setup, duplicate URL idempotency, unsafe input handled with focus/error and no outbound fetch. FR-001–009, AC-M01–M07, U-01.

**S03 — Product detail, evidence and history**
- `internal/httpui/product_detail.go`, offer comparison VM: fresh exact public/in-stock default, separate loyalty/multibuy/unknown/stale, source deep link and timestamps. Paginated history table/chart deferred to R2 as per product.
- Gate: AC-P04–P12, U-02–U-05/U-09; click from offer to retailer is explicit and external.

**S04 — Dashboard, source health and activity**
- `internal/httpui/{dashboard,sources,activity}.go`; source capability and last success, job status, error categories, operator pause, settings and resource panel. HTMX status only for bounded check requests.
- Gate: stale and rate-limit messages truthful, source coverage visible; activity has redacted causes. FR-031/033/035/038, U-05/U-10.

**B06 certification:** independent UX/accessibility review, evidence/link truthfulness, app usable with JavaScript disabled.

## LP-P0-B07 — Watches, reliable alerts and notification outbox (G4)
**Dependencies:** B02 offers/jobs plus B06 product detail.  
**Exit:** exactly-once logical alert episodes, bounded delivery attempts and watch UX.

**S01 — Versioned watch rule engine**
- `internal/watches/{rules,evaluate,episode}.go`; threshold cents/percentage, any/all, public-only default, freshness, stock, exact-match and rule version; re-arm after recovery policy.
- Gate: never alert on stale/unknown member/price parse failure, historical baseline absence or mismatch. AC-A01–A05/A11–A14.

**S02 — Alert outbox and webhook transports**
- `internal/notify/{outbox,webhook,delivery}.go`; one outbound HTTPS webhook protocol with idempotency key, safe host allowlist and redaction, retry/backoff, delivery state.
- Gate: webhook 2xx/429/5xx/drop, dedupe, bounded retries and recovery. AC-A06–A08/A14. Home Assistant/ntfy can be mapped to webhook shape; richer native templates later.

**S03 — Watches, Deals and Settings UI**
- `internal/httpui/{watches,deals,notification_settings}.go` plus UX partials: create/pause rules, test notification, see verified price/offer conditions, recent deal episodes.
- Gate: public-only chosen by default, eligible member offers separate, preview vs actual alert explicit, understandable disabled/error states. AC-A09/A10, U-03/U-08.

**S04 — Combined price-to-alert end-to-end and durability**
- Deterministic scenario: two exact listings, fresh threshold drop, history records, one alert, recheck no duplicate, recover above target, fall again, send bounded notifications; restart between stages.
- Gate: AC-A* complete, no phantom sale claim, rule-version trace, SQLite replay safety and Pi peak under cap.

**B07 certification:** independently review alert correctness + notification privacy + safe retry; no false-positive negative fixture.

## LP-P0-B08 — Production-capable source adapters (conditional G0 decision)
**Dependencies:** B00 permission/fixture proof and B04/B05 pipeline.  
**Exit:** documented usable retailer coverage; no false completeness claim.

**S01 — First approved retailer adapter**
- Select source that passed B00. Implement `internal/retailers/<approved-key>/` adapter and fixtures, capabilities and parser version. Explicitly parse product/offer/stock/locality attributes only if supported.
- Gate: representative success & negative pages, parser drift and source 403/429 behavior, terms/rate compliance.

**S02 — Second independently approved retailer adapter**
- Same scope, separate parser tests; no copy-pasted selectors; document variant matching and technical/legal constraints.
- Gate: real-life controlled sample and no unsupported values falsely marked known.

**S03 — Coverage and source health disclosures**
- Display source assessment date, capabilities, per-field reliability limits and blocked/unsupported status; add enable switch only for approved sources.
- Gate: sources page has truthful coverage and no source silently queried.

**S04 — Fixture aging and maintenance strategy**
- Build source-parser fixture versioning, safe sample refresh procedure, drift detector on field completeness (not CAPTCHA evasion), rollback/disable mechanism.
- Gate: old fixture remains valid under known parser version, changed markup triggers explicit parser error.

**Conditional:** If fewer than two are lawfully/reliably feasible, deliver and test general structured-data/manual-URL watcher with clearly limited claims. Create product-PRD change **before** changing a committed R1 promise. B08 must not be marked complete with fabricated integrations.

**B08 certification:** source compliance + parser reliability review; signed source approvals.

## LP-P0-B09 — MVP integration, memory certification, release (G5)
**Dependencies:** B00–B08 with any approved product scope changes.  
**Exit:** R1 local release candidate and deployment runbook (not claimed until executed).

**S01 — End-to-end test pack and local security audit**
- Implement all R1 `AC-M/P/A/S/X/O` tests, HTMX/SSR accessible screens, private-network attack tests; run `make release-check`. Generate missing requirements list and resolve.
- Gate: all required contract tests PASS, no block-worthy P0/P1 defects.

**S02 — ARM64 stress/24h soak and memory study**
- On actual Pi under representative other-container load: cold start, browsing/search, manual fetch, 100 products×5 listings safe synthetic/approved requests, retries, high-volume parsing, notifications, SQLite backup and 24-hour soak.
- Read cgroup peak/OOM events and host delta; zero OOM; **total attributable Pi memory <500,000,000 bytes**. No disabling advertised features during tests.
- Gate: NFR-001–006/010, AC-R01–R12, evidence in `docs/evidence/B09/`; performance baselines and limits documented.

**S03 — Recovery/security/upgrade certification**
- Perform online backup and clean restore, induced SIGKILL while queue active, storage pressure, migrations, safe upgrade with old container stopped and rollback; inspect secrets/network egress/license/vulnerabilities.
- Gate: NFR-008/009/011/014 and AC-O*/X* satisfied.

**S04 — Independent final review and R1 publishing**
- Independent principal-level architecture/security/UX/ops review, release notes, supported retailer matrix, installation/upgrade/backup docs, version tag and rollback plan.
- Gate: no unresolved hard-contract violation; reviewer sign-off; publish only then.

# C. P1 — Discovery, eligible promotions and richer comparisons

## LP-P1-B10 — Assisted discovery (G6 part 1)
**Dependencies:** certified R1, compliant search-source candidates.

**S01 — SearchProvider interfaces and bounded workflows**
- `internal/discovery/{service,registry,candidates}.go`; provider interface for assessed retailer site search; strict URL/rate/budget cap; store candidate origin and unverified state.
- Gate: partial provider failure, dedupe, pagination under budget, no source bypass; FR-006 and AC-M08/M09.

**S02 — Optional user-managed remote search provider**
- `internal/discovery/providers/<provider>/` with opt-in settings, key validation/redaction, quota/spend guard and privacy notice. If no suitable provider, omit feature rather than hard-require a paid service.
- Gate: zero external calls until enabled, disabled/offline catalog works, rejection at quota and no extra Pi container. ADR-012.

**S03 — Candidate review UX**
- `internal/httpui/search.go` groups exact/possible/other, filters scent/pack and approval preview; attach to existing canonical product only after validated extraction.
- Gate: UX Search state matrix, U-02, AC-M09; no discovery snippet creates a direct deal alert.

**S04 — Search-source reliability**
- Per-source response/candidate count, search-page change fixtures and error reporting; online search never blocks local catalog.
- Gate: bounded memory during large candidate inputs, transparent source coverage.

## LP-P1-B11 — Advanced promotion and unit-pricing intelligence
**Dependencies:** certified R1, product promotion semantics.

**S01 — Accurate units and pack comparisons**
- Formal rational quantities and multiple dimension classes, per-L/kg/unit alternative ranking; no cross-unit false comparisons; pack requirements explicit.
- Gate: 750 ml vs 1L separated exact groups yet comparable per-L only in alternate view. FR-018.

**S02 — Loyalty/multibuy/coupon contracts**
- Retailer-specific eligibility extraction where supported, validity dates, minimum quantity and conditional shopping basket totals; exact evidence and fail-closed ranking.
- Gate: AC-P07–P09/P12, explicit public-vs-member watch and preview tests. FR-019/027.

**S03 — Product history and conservative trends**
- Efficient SQL aggregates over valid comparable observation series; small client chart with historical timestamps, explanatory claim-vs-observed labeling, no invented savings.
- Gate: AC-P05/P06, NFR-004, paginated history and cache/memory profile. FR-020.

**S04 — Extended notification channels**
- Home Assistant webhook, ntfy/Gotify adapters through existing HTTP outbox, optional email only with safe configuration (no SMTP service container); prefs and endpoint test.
- Gate: independent dedupe/retry/redaction per destination, no extra default daemon. FR-028.

## LP-P1-B12 — R2 integrated certification
**Dependencies:** B10–B11.
**S01:** end-to-end Domestos product-name search→confirm variant→watch→price change→eligibility-aware alert with explicit partial-store coverage.
**S02:** repeat Pi 500 MB tests with discovery, promotions, history chart and all added features simultaneously.
**S03:** independent batch/final review, source-policy refresh, backup/restore/migration drill and release documentation.
**Exit:** R2 only if all newly advertised workflows fit cap; disable/omit optional modules that do not pass and revise release scope.

# D. P2/P3 — Controlled expansion (not committed MVP scope)

## LP-P2-B13 — Coverage, reporting and admin improvements
- **S01:** prioritized adapters for additional **approved** South African sources; each includes assessment, fixtures, version and rollback; no blanket retailer promise.
- **S02:** history/price distribution/savings report only when based on independently observed comparable prices; explain sample gaps.
- **S03:** bounded streaming CSV export and configurable retention under privacy constraints.
- **S04:** optional authorized read-only household accounts with page-access review only if product need warrants complexity.
- **Exit:** Pi memory/backup/security certification repeated; PRD change precedes new major capability.

## LP-P3-B14 — Optional advanced rendering and ecosystem
- **S01:** feasibility study for user-operated **remote** chromedp/Rod browser on a *different* machine, subject to retailer policy, security and user opt-in.
- **S02:** secure remote renderer protocol with bounded input/output, no arbitrary URL access, no retailer credential theft, privacy notice and opt-out.
- **S03:** provider reliability, additional alert/channel integrations and integration API only after threat model and user demand.
- **Gate:** no local Chromium or hidden additional Pi runtime; if remote service runs on Pi it must share the exact 500 MB total budget and be recertified.

# E. Cross-cutting engineering standards

## E1. Product and code standards
Every new feature must preserve:
- exact-match and fresh-price semantics, never auto-“sale” for just a cheap standard price;
- positive integer cents, explicit ZAR, typed offer status and conditional eligibility;
- product/listing/observation/offer/watch/outbox separation and evidence/version lineage;
- SSR/HTMX forms with server validation, CSRF, safe output escaping and no external runtime CDN;
- one process, no background browser, no unbounded goroutines/body/queue;
- source approval and enforced fetch allowlists, stop/backoff on block;
- scheduled jobs outside SQL transactions, serialized writes and durable replay;
- safe restart/backup and truthful source failures.

## E2. Per-slice working contract / agent task template
```text
Slice ID and scope:
Product FR/NFR + UX/AC references:
Technical design references / ADR:
Dependencies and known interfaces:
Files allowed to modify:
Functional changes:
Database/migration:
Security/SSRF/retailer policy effect:
Memory/allocation/concurrency impact:
Tests & local commands to run:
Negative/failure cases:
Observability and rollback:
Definition of done:
Independent batch-review notes:
```
An implementation run can execute multiple slices but should produce separate commits and validation summaries, one per slice, to preserve review traceability.

## E3. Memory engineering checklist for every batch
1. Declare number of services/processes added (must remain one Pi container by default).
2. List response/body/DOM size caps and allocation hotspots.
3. State maximum in-flight fetch/search/SQLite query jobs and associated memory.
4. Avoid reading entire histories or exports into slices; use paginated/streaming queries.
5. Profile or justify all new caches, goroutines, workers, image handling and third-party libraries.
6. Capture cgroup peak, `memory.events`, Go heap and host delta in batch evidence.
7. Check overload shedding and error state, not only normal throughput.
8. Re-run full 500 MB acceptance on changes to parsing, discovery, GC, SQLite or notification pipelines.

## E4. Threat/security checklist for every external integration
- Document published source terms, robots, enabled capability and frequency.
- Restrict outbound scheme/port/domain and resolved IP, redirects and DNS rebinding.
- Limit response decompression, timeouts, retries, requests and login/session data.
- Treat HTML/JSON as untrusted and escape template output; no eval/shelling out.
- Never store retailer credentials by default; no proxies/CAPTCHA circumvention.
- Webhook destination is an SSRF concern too; allow safe LAN endpoints (e.g. HA) **only** through separately explicit and tightly scoped user-configured notification endpoint policy, never by weakening retailer-fetch restrictions. Network boundaries for webhooks and retailer fetches are distinct.
- Redact sensitive error data; audit access, watch edits, source approvals and deliveries.

## E5. Design/UX completion checklist per screen
- Full-page SSR route plus HTMX fragment behavior and no-JS fallback.
- Mobile 320/375 px and desktop, accessible keyboard/focus/labels/live regions.
- Loading, empty, blocked, stale, invalid, unauthorized, offline and error states.
- Source coverage and timestamp visible on any price/offer display.
- Conditional/member-only/multibuy terms and unit-size equivalence never concealed.
- Clear primary action, back/cancel path and error preservation.
- Minimal small assets; all CSS/JS served locally and cached.

## E6. Data and migration checklist
- Schema change includes indexed queries, FK/unique invariants, migration tests and version check.
- Backup or rollback strategy identified before release; no unbounded DB transaction/lock.
- Job/outbox idempotency and any reprocessing behavior documented.
- Retention does not destroy alert evidence; no raw HTML or secrets stored indiscriminately.
- SQLite WAL/online backups and `PRAGMA integrity_check` validated after faults.

# F. Quantified gates, risks and go/no-go

## F1. Release gates
| Gate | Target batch | Required outcome |
|---|---|---|
| G0 | B00 | Approved source feasibility or documented honest fallback; Pi baseline |
| G1 | Already met docs | Product requirements, UX and acceptance merged before this plan |
| G2 | B01–B03 | Single Go+SQLite runtime, local gate scripts, secure auth/SSRF, tested backup |
| G3 | B04–B06 | Bounded parser + immutable observation + canonical comparison + full UI |
| G4 | B07 | Watch rules, episode dedupe, outbox delivery and negative-price suite |
| G5 | B08–B09 | Transparent source coverage, independent review, full Pi 24h/500 MB certification |
| G6 | B10–B12 | Optional discovery/promotions are accurate and fit within same Pi limit |

## F2. Risk register and preemptive actions
| Risk | Likelihood/impact | Prevention / blocker |
|---|---|---|
| Dynamic or blocked retailer content | High / high | B00 feasibility first, manual-URL fallback, remote renderer only off-device |
| False product equivalence | High / high | Hard variant/size/pack conflicts and manual match confirmation |
| Promotion/stock ambiguity | High / high | Evidence-backed offer taxonomy, public-only default, fail closed |
| DOM or decompression memory blowup | Medium / critical | Hard expanded byte cap, parser fixtures and memory watchdog |
| Parallel jobs / SQLite lock contention | Medium / high | One worker, short transactions, unique keys and lease tests |
| SSRF into home LAN | Medium / critical | Centralized outbound policy, DNS/redirect tests and distinct webhook policy |
| Duplicate alerts after crash | Medium / medium | Rule versions, unique episodes, outbox idempotency and restart test |
| Memory cap during backups/updates | Medium / critical | Streaming backup, no dual containers, 24h Pi combined-workload test |
| No permitted reliable sources | High / high | Scope gate and honest unsupported statuses rather than evasion |
| Dependency changes | Medium / medium | Pinned versions, local vuln/license review, local gate evidence |
| Overdesign / delivery slow | Medium / medium | R1 manual-watch focus, no multi-user/SearXNG/Chrome/SPA in baseline |

## F3. Definition of Ready per batch
Dependencies merged; product/technical contract stable; tests/fixtures and data migrations sketched; resource/SSRF/scope questions resolved or marked gating; explicit files/interfaces and acceptance IDs assigned. A slice with a missing source-policy answer is not ready for live scraping.

## F4. Definition of Done per slice
Code compiles on ARM64; targeted unit/integration/security negative tests run locally; targeted docs updated; no new unbounded allocation or bypass; tests and actual results recorded with SHA; local reviewer checks changed behavior. Only then merge implementation PR.

## F5. Definition of Certified Batch
Independent reviewer checks integrated behavior, package/data contracts, security, Pi resource effects and local evidence. All blocking findings remediated. Batch is certified only if evidence shows tests were **actually** run; a reviewer must not mark drafts as pass.

## F6. Definition of R1 Release
Must-have FR and NFR acceptance passed; independently reviewed; two validated source adapters *only if proved feasible and permitted* (or product-scope revision explicitly merged first); manual URL flow usable; price alerts correct; no false deal; source failures transparent; recoverable SQLite; actual Pi **<500,000,000 bytes** total attributable runtime memory, 0 OOM, 24h representative soak; documented rollback, local security and access review. Any hard failure blocks release.

# G. Repository delivery and immediate first implementation steps
**This plan is documentation, not software implementation.** First coding PR after the plan merges:
1. Review B00 retailer feasibility and Pi baseline; record evidence without asserting live results.
2. Implement B01 Go module/Chi routes and minimal SSR shell with local tests.
3. Build lean ARM64 image and enforce/inspect 440MB cgroup cap.
4. Execute first Pi startup/resource smoke and record actual peak; adjust allocation targets while respecting 500MB contractual max.
5. Continue with B02 data model and B03 security in separate dependency-safe branches.
6. Hold B04 live retailer work until B00 site approval; fixture-only development may proceed earlier.

## H. Engineering quality certification matrix
| Quality dimension | Primary batches | Required final evidence |
|---|---|---|
| Functional product discovery/tracking | B04/B06/B08/B10 | AC-M/P, curated source fixtures and honest coverage |
| Alert correctness | B07/B11 | AC-A, no false-positive fixtures, restart dedupe |
| Security and privacy | B03/B04/B07 | AC-X/S, SSRF/CSRF/XSS/report |
| Storage and recovery | B02/B05/B09 | AC-O, backup/migration/kill test |
| Performance/memory | B01/B04/B05/B09/B12 | AC-R, cgroup peak + host delta + 0 OOM |
| UX/design/accessibility | B01/B06/B07/B10/B11 | U-01–U-10, mobile/keyboard/SSR tests |
| Source compliance | B00/B08/B10/B13 | Dated per-source terms/capability assessments |
| Architectural conformance | all batches | Package/invariant review, ADR changes, local gate logs |
| Deployment/supportability | B01/B09 | ARM64 image, runbooks, rollback, verified backups |

## I. Completion state (as of planning)
- **Done:** Product PRD, UX specification and acceptance matrix were authored and merged to `main` before this document.
- **Planned only:** B00–B14; no code, fixtures, performance validation, approved retailer integrations, security certification, or release has been executed.
- **Unknown:** permissible source coverage, actual Pi memory peaks, final deployment image size and real scraping reliability. These are explicit pre-release verification items.

No feature or operational capability should be described as deployed solely because this detailed plan exists.
