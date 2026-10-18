# leanPrice — Architecture Decision Records (ADRs)
**Status:** Accepted design decisions with implementation in progress, 2026-10-19  
**Review:** v1.1 product/technical contract alignment; implementation, source approval and resource evidence are tracked separately.  
**Dependency:** [Product PRD](../product/PRD.md) merged to main before these records were created.  
**Change discipline:** every reversal requires a new ADR that supersedes the old one, product-impact review and updated implementation plan/acceptance tests. “Accepted” means design baseline, **not independently reviewed or performance-validated deployment**. See [the execution ledger](../plans/IMPLEMENTATION-PLAN-EXECUTION.md) for implemented and blocked decisions.

## ADR-001: Original leanPrice implementation in Go
**Decision:** Create original source in `Onellan/leanPrice` using Go, Chi, `html/template`, HTMX and Tailwind compiled at build time. No PriceBuddy code import/translation.
**Context:** Pi hosts multiple services; Laravel + separate database/browser is resource-heavy and introduces maintenance complexity. Original product features need tailored SA retailer and promotion semantics.
**Alternatives:** customize PriceBuddy Laravel/MySQL, Laravel+SQLite fork, Python crawler + UI, SPA + Go API.
**Why:** single Go binary is easier to bound, profile, deploy on ARM64 and integrate with existing idiomatic Go practices; server-rendered HTML avoids Node.js runtime.
**Tradeoffs:** must build an original UI, product model, scraper health, alert engine and migration tooling; up-front implementation larger than fork.
**Proof:** ARM64 image, SSR/HTMX fallback, quality gates and peak-memory soak.
**Status:** Accepted as design.

## ADR-002: Embedded SQLite with one writer
**Decision:** `modernc.org/sqlite` CGO-free driver; single persisted SQLite file, WAL mode, `foreign_keys` and tested busy timeout; begin with one open DB connection and short transactions. Jobs/alerts/sessions stored locally; no MySQL/Postgres/Redis.
**Alternatives:** MySQL/MariaDB, Postgres, separate Redis queue, file-only JSON.
**Why:** minimal idle overhead, ACID and straightforward backup, one container.
**Tradeoffs:** writer contention, migration/backup discipline, query and WAL tuning required; CGO-free driver performance/memory must be measured versus alternatives.
**Proof:** migration + concurrent job tests, corruption/restart/backup drills, `EXPLAIN QUERY PLAN`, memory profiles.
**Reversal trigger:** unacceptable memory/performance validated on Pi with no practical tuning route; any alternative must still meet the 500 MB total cap.
**Status:** Accepted as design; driver tuning provisional.
**Source:** https://pkg.go.dev/modernc.org/sqlite

## ADR-003: No local browser automation
**Decision:** default scraper uses approved HTTPS GET, optional Colly orchestration, JSON-LD/embedded JSON, goquery CSS extraction. No Selenium, local Chromium, chromedp or Rod runtime on Pi. Do not bypass CAPTCHA, source blocks, terms or rate limits.
**Alternatives:** SeleniumBase, chromedp/Rod + local Chromium, remote browser rendering, third-party proxy/scraping service.
**Why:** browser memory spikes threaten hard cap; many product pages publish usable structured metadata.
**Tradeoffs:** dynamically loaded products and login-only price pages may be unsupported.
**Proof:** source feasibility fixtures and reliability report; cap decompressed size and parser memory.
**Reversal:** remote renderer **outside Pi** may be an opt-in separate feature/ADR with documented cost/privacy; no in-process browser under 500 MB contract.
**Status:** Accepted, store-specific coverage unknown.
**Sources:** https://github.com/gocolly/colly ; https://github.com/PuerkitoBio/goquery ; https://developers.google.com/search/docs/appearance/structured-data/merchant-listing

## ADR-004: Explicit retailer capability/permission registry
**Decision:** every source endpoint/capability defaults disabled and unassessed; a source may be enabled only after a dated source-policy and technical review. Retailer rows are descriptive and do not carry a second enable switch. Capability flags describe manual URL, permitted search, public price, stock, promotion and locality. A pasted URL remains subject to the same assessment and destination checks as scheduled fetching. Runtime enforces rate limits and stops/backs off on 403/429.
**Alternatives:** generic crawl arbitrary domains; infer support when any HTML page parses.
**Why:** safer, maintainable, honest coverage.
**Tradeoffs:** onboarding sources requires work; coverage grows slowly.
**Proof:** source approval checklist, 403/429/robots fail-closed tests and maintained fixtures.
**Status:** Accepted.

## ADR-005: Exact-match-first canonical products
**Decision:** separate canonical product and store listing; hard-conflict rules on size/pack/variant trump fuzzy scores. Unknown match goes to manual review. Unit comparisons are an explicit alternate lens.
**Alternatives:** free-text approximate match and cheapest-price sorting.
**Why:** prevents false deals from 750 ml vs 1 L or scent mismatch.
**Tradeoffs:** lower auto-match recall, extra confirmation step.
**Proof:** Domestos and other curated negative fixtures.
**Status:** Accepted.

## ADR-006: Explicit offer eligibility and alert provenance
**Decision:** keep offer kind, eligibility, currency, stock, freshness, validity, minimum qualifying quantity and bounded evidence provenance. `amount_minor` is the total amount charged for `minimum_qty` saleable listing packs; ordinary single-pack offers use quantity 1. Public single-pack is the R1 default. Member/multibuy offers require explicit eligibility and a calculable qualifying basket; coupons remain informational and ineligible for R1 ranking/alerts until R2 models their full basket conditions. Unknown stock is never called in stock, and any price-only alert must carry the qualifier. Immutable alert episode references the observation and rule version.
**Alternatives:** one price per listing with “on sale” boolean.
**Why:** accurate promotions and explainable alerts.
**Tradeoffs:** bigger schema, more validation.
**Proof:** member/multibuy/stale/invalid false-positive regression suite.
**Status:** Accepted.

## ADR-007: SQLite durable queue + outbox
**Decision:** scheduler and one worker within Go process; persisted queued/leased/retry jobs, unique dedupe keys; notification outbox separately persisted. No always-running queue service.
**Alternatives:** goroutine-per-product, in-memory queue, cron inside container, Redis.
**Why:** restarts safe and bounded resources.
**Tradeoffs:** must handle lease expiration, idempotency and SQLite contention explicitly; downstream delivery cannot be guaranteed exactly once.
**Proof:** SIGKILL recovery, manual spam, alert-episode/delivery retry tests.
**Status:** Accepted.

## ADR-008: Application and cgroup resource governor
**Decision:** measure total attributable leanPrice memory; enforce one Pi container with initial kernel hard limit **440,000,000 bytes (440 MB decimal)** and no extra swap, leaving room below the **500,000,000-byte** system ceiling. Target a normal-workload cgroup peak no greater than **350,000,000 bytes**. `GOMEMLIMIT`, bounded HTTP/DOM, queue, query and request concurrency provide proactive backpressure; these targets are not evidence until measured.
**Alternatives:** rely on Go GC, 500 MB per service, idle-only measurement, browser time slicing.
**Why:** user constraint is a hard **whole stack** maximum, not a target.
**Tradeoffs:** slower jobs and incomplete retailer coverage under memory pressure; cgroup hard limits can OOM if application budgets fail.
**Proof:** cgroup memory peak/events and host delta under full ARM64 workload, no OOM; backup/search/scraping overlap test.
**Status:** Accepted as architectural intent; **budget not yet proven**.

## ADR-009: Build-time frontend, SSR-first, privacy-first
**Decision:** Chi + `html/template` pages, HTMX partials, pinned locally bundled assets, precompiled Tailwind. Login/session/CSRF with protected administration; no public JSON API or external CDN as MVP requirement.
**Alternatives:** React SPA, live Tailwind CDN, cloud frontend.
**Why:** fewer resources/dependencies and usable no-JS flows.
**Tradeoffs:** HTML partial discipline needed, complex interactions may need small custom JS.
**Proof:** JS-disabled smoke, form/focus/CSRF/accessibility tests.
**Status:** Accepted.

## ADR-010: Local-only validation and deployment
**Decision:** enforce Makefile/scripts for Go tests, vulnerability/license checks, lint/static checks, fixture/integration suites, ARM64 build, Pi 24h soak, backup/restore, security and accessibility audits. Migrations are forward-only; a failed upgrade rolls back by restoring the validated pre-upgrade backup with the previous application image, not by applying unreviewed reverse SQL. **Do not create/change GitHub Actions workflows** or use hosted CI as a quality gate.
**Alternatives:** GitHub Actions, cloud build/test, manual-only testing.
**Why:** user's established local-first development operation and reproducibility.
**Tradeoffs:** operator must run/record local gates; results need durable artifacts and independent review.
**Proof:** signed-off `docs/evidence/<release>/` reports with local command/commit/image fingerprint, summarized results and explicit blockers.
**Status:** Accepted.

## ADR-011: No user-entered arbitrary outbound fetch
**Decision:** retailer URL validation + source allowlist + DNS/IP and redirect revalidation, including disallowing private/internal/metadata addresses and dangerous schemes/ports. The hardened fetcher is the sole retailer-fetch authority and adapters cannot create raw clients. User-configured notification targets use a separate policy: a private endpoint may be allowed only as an explicit, exact administrator configuration, without weakening retailer-fetch restrictions.
**Alternatives:** raw Colly visit of any user URL; trust DNS/redirects; only block localhost text.
**Why:** SSRF is a critical risk for a server on a LAN containing Home Assistant and other devices.
**Tradeoffs:** some legitimate sources/CDNs require explicit allowlist adjustments and testing.
**Proof:** IPv4/IPv6/DNS/CNAME/redirect attacks, rebinding tests, and source approval.
**Status:** Accepted.

## ADR-012: Evidence-backed local-first operation
**Decision:** manual URL and watches work without a third-party search service. Optional remote discovery/search browser provider requires separate explicit user enablement, quota/budget and data-transmission disclosure. Retailer accounts, loyalty scraping behind login, stealth proxy and CAPTCHA bypass are excluded.
**Alternatives:** mandatory SearXNG stack, required paid search API, always-on remote browser.
**Why:** trustworthy offline catalog and Pi footprint.
**Tradeoffs:** automatic store discovery may be partial.
**Proof:** network-disabled tests show history/catalog/settings working; disabling provider does not disable watches.
**Status:** Accepted.

## ADR-013: One-at-a-time deployment on small Pi
**Decision:** stop old container before starting new version; verify/backup DB before migrations, smoke new container, restore safe backup on failed upgrade. No blue-green overlap on Pi because duplicated app memory may break cap.
**Alternatives:** rolling parallel duplicate container, blind image update.
**Why:** protects 500 MB total budget during updates.
**Tradeoffs:** short maintenance downtime.
**Proof:** upgrade/failed-upgrade drill and cgroup peak captures.
**Status:** Accepted.

## ADR-014: Store source policy/technical feasibility gate precedes promises
**Decision:** R1 requires two independently validated source adapters **only if permitted extraction is technically feasible**. Any manual-URL fallback is limited to assessed, approved, enabled sources; it is not permission to fetch arbitrary pasted URLs. If fewer than two sources pass, revise release scope and claims before release. If no source passes, R1 release remains blocked until the source scope or product workflow is revised.
**Alternatives:** claim full Checkers/PNP/Clicks/etc support before testing.
**Why:** production retailer pages and policies change, and Go HTTP alone may be insufficient.
**Tradeoffs:** initial retailer coverage is uncertain.
**Proof:** source-specific feasibility register, dated fixtures, compliant request tests and success-rate measurement.
**Status:** Accepted.

## ADR change log template
```text
ADR-ID:
Date / owner:
Status: proposed | accepted | superseded
Problem:
Options considered:
Decision and reason:
Product and resource effects:
Security/retailer policy effects:
Acceptance evidence:
Migration/rollback:
Supersedes:
```
