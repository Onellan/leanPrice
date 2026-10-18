# leanPrice — Product Acceptance Matrix and Release Contract
**Version:** 1.0 | **Status:** Product baseline | **PRD:** [PRD.md](PRD.md) | **UX:** [UX-SPEC.md](UX-SPEC.md)

This document is a **test contract**, not evidence that tests have passed. No requirement is implemented by virtue of inclusion here. Each scenario must be linked to a local test, actual result, artifact and review sign-off before release.

## 1. Definition of Done / release evidence
A release requires: traceable PRD IDs; design review; unit and integration tests; parser fixtures; security and SSRF tests; ARM64 Docker smoke; 24-hour 100-product/500-listing workload simulation with permitted fetches/mocks; memory and host-impact report; backup/restore drill; failure injection; independent code review; local validation log; migration reversibility policy; docs/help update. Do **not** use GitHub Actions as a required gate.

Results must record UTC timestamp, Pi model/OS, Docker version, image digest, CPU/RAM baseline, cgroup memory peak/current, container OOM events, swap use, test size, source mix, network assumptions and scripts/runbook versions. Synthetic mocks are distinct from live-source feasibility evidence.

### Binary acceptance
- `PASS` requires objective evidence and all critical negative cases.
- `FAIL` means observed contract violation.
- `BLOCKED` means cannot test; do not classify as passed.
- `NOT IMPLEMENTED` is never a release success.
- `EXCEPTION` to a hard constraint is **not allowed**; revisit product scope or architecture.

## 2. Product identity and discovery
| Case | Given / when | Expected |
|---|---|---|
| AC-M01 | Valid approved retailer URL pasted | Safe URL validated before fetch; preview shows retailer, source URL, title, parsed size and confidence |
| AC-M02 | URL targets 127.0.0.1, RFC1918, metadata IP, local DNS alias, IPv6 loopback/ULA, or redirects there | Reject before HTTP request and after redirects; event logged safely |
| AC-M03 | URL on unapproved/disabled domain | User receives unsupported/disabled message; no fetch |
| AC-M04 | Domestos Lavender 750ml listing alongside Original 750ml | Separate canonical matches; no automatic merge |
| AC-M05 | Domestos Lavender 750ml vs Lavender 1L or 2×750ml | Do not label “same exact product”; explicit alternative size/pack |
| AC-M06 | Title and quantity uncertain | User confirmation required; cannot auto-link as exact |
| AC-M07 | Listing re-added | Idempotent URL normalization prevents duplicate listing; preserve history |
| AC-M08 | Search API not configured / network offline | Existing catalog usable; discovery capability clearly unavailable |
| AC-M09 | R2 retailer search returns duplicates / mixed sizes | Group and deduplicate candidates; review queue never silently accepts ambiguous match |

## 3. Price, promotions and ranking
| Case | Given / when | Expected |
|---|---|---|
| AC-P01 | JSON-LD Offer in ZAR with price 29.99 | Store integer 2999 cents and explicit ZAR, raw evidence digest, observed timestamp |
| AC-P02 | “R 1 299,90” localized HTML | Parse 129990 cents using tested locale rules; never floating point |
| AC-P03 | Missing price / 0 / negative / malformed / wrong currency | Invalid observation, no new current price or alert |
| AC-P04 | Fresh ordinary price below R30 target | Can produce “below target” alert; **not** necessarily “on sale” |
| AC-P05 | Stale historic R25 but fresh R34 | R34 is the current public price; stale R25 excluded from now-ranking |
| AC-P06 | Retailer claimed “was R40, now R30” | Label source claim; do not state independently proven saving |
| AC-P07 | Member-only R25 vs public R35 | Default public comparison uses R35; member price separated |
| AC-P08 | Two-for-R50, single R30 | Multibuy effective R25/unit only if product quantity and minimum purchase verified; default public single-price comparison unchanged |
| AC-P09 | Coupon price excludes minimum basket or validity | Eligibility unknown; do not rank as unconditional public price |
| AC-P10 | In-stock R31, unknown-stock R27, out-of-stock R24 | Default best-available ranks verified in-stock R31; unknown indicated separately, out-of-stock excluded |
| AC-P11 | Listing lacks stock field | Stock = unknown, never inferred as in stock |
| AC-P12 | Multiple accepted exact listings from same retailer | Deduplicate equivalent offers by retailer/location/eligible terms; provide source list |
| AC-P13 | Search/provider returns product snippet price without exact product page proof | Mark unverified candidate; do not issue a price alert |
| AC-P14 | Promotion currency not ZAR | Clearly display currency; no ZAR comparisons absent reliable conversion policy (out of v1 scope) |

## 4. Watches, alerts and delivery
| Case | Given / when | Expected |
|---|---|---|
| AC-A01 | Admin sets target R30, public only | Persist validated rule with eligible offer scope |
| AC-A02 | New fresh exact public R29.99 observation | Create one versioned alert episode and delivery attempt |
| AC-A03 | Scheduler reprocesses unchanged same-price observation ten times | No repeat notifications for same episode |
| AC-A04 | Price recovers above target, then falls again after re-arm policy | New episode generated once; policy visible |
| AC-A05 | Parser emits R0 or brand mismatch | No alert; extraction/identity error recorded |
| AC-A06 | Retailer blocked / HTTP 429 | No alert; source backoff and next attempt visible |
| AC-A07 | Notification webhook fails once | Bounded retry with backoff; status visible and event idempotency retained |
| AC-A08 | Notification webhook fails repeatedly | Terminal/paused state and operator visibility; no unbounded retry |
| AC-A09 | User pauses watch or source | No new outbound checks or alerts for it after in-flight tasks finish |
| AC-A10 | Admin tests notification channel | Test is visibly and semantically different from price alert |
| AC-A11 | Old valid cached price is reread after restart | Not considered new fresh price or eligible alert |
| AC-A12 | Member-only offer qualifies but rule is public-only | No notification |
| AC-A13 | User changes threshold while old delivery queued | Versioned rule ensures traceable and correct decision; no spurious duplicate |
| AC-A14 | Concurrent checks report same qualified offer | Exactly one logical episode/event, regardless of delivery retry count |

## 5. Source, scheduler and scraping resilience
| Case | Given / when | Expected |
|---|---|---|
| AC-S01 | Source robots/terms review says disallowed | Adapter cannot be enabled |
| AC-S02 | HTTP timeout, 5xx, DNS failure | Classified error; bounded retries, never overwrite with zero |
| AC-S03 | HTTP 403/429 | Respect blocking/backoff; no evasion or credential rotation |
| AC-S04 | 30MB HTML body or decompression bomb | Request aborted at configured expanded bytes; bounded memory |
| AC-S05 | 10,000 product links on one listing page | Bounded parse/candidate count; no unbounded queue growth |
| AC-S06 | Slow/hanging domain | Deadline and per-domain concurrency limit enforced |
| AC-S07 | Normal parser CSS changed | Fixture/parsing error surfaced; previous good price retained as stale |
| AC-S08 | Admin “check now” pressed repeatedly | Idempotent/single pending job; UI explains it is queued |
| AC-S09 | Host restarts while jobs are leased | Due leases recovered safely; no duplicate alerts |
| AC-S10 | System offline | Local UI/history works; scheduler defers gracefully |
| AC-S11 | Provider terms change | Source flagged for re-assessment and disabled when policy requires |
| AC-S12 | Multiple source adapters enabled | Cross-domain and per-domain token limits observed |

## 6. Security and privacy
| Case | Expected |
|---|---|
| AC-X01 | Secure first-run provisioning rejects default/common passwords; setup token cannot be reused |
| AC-X02 | Sessions are HTTP-only, SameSite, correctly Secure under HTTPS; session fixation prevented |
| AC-X03 | Mutating POST/HTMX requests without CSRF token rejected |
| AC-X04 | Stored retailer titles/scripts rendered escaped; CSP disallows unexpected execution |
| AC-X05 | URL parser rejects schemes other than HTTPS (HTTP only for expressly approved source, if ever); revalidates each DNS resolution/redirect |
| AC-X06 | Credentials and webhook secrets never appear in logs, UI error messages or exports by default |
| AC-X07 | Reverse-proxy headers trusted only from configured proxies; no unsafe forwarded-host redirects |
| AC-X08 | Privileged endpoints blocked for unauthenticated clients, including metrics containing sensitive identifiers |
| AC-X09 | Account login throttled; errors do not reveal whether username exists |
| AC-X10 | No runtime third-party telemetry or unapproved external service calls |
| AC-X11 | Safe migrations and restore paths reject path traversal and malicious archive inputs |
| AC-X12 | Supply-chain dependency license and vulnerability report recorded locally |

## 7. Operational resilience
| Case | Expected |
|---|---|
| AC-O01 | Fresh install ARM64 with persistent storage succeeds and serves UI |
| AC-O02 | Upgrade across supported migration validates without losing price/alert history |
| AC-O03 | SQLite WAL-consistent backup restores into clean container and passes `PRAGMA integrity_check` |
| AC-O04 | Abrupt `SIGKILL` mid-job/restart recovers with no corrupt database and no logical duplicate alert |
| AC-O05 | Bad migration fails safely; last-known-good backup preserved; no unsanctioned automatic destructive rollback |
| AC-O06 | Retention/compaction never removes audit evidence required by an active alert episode |
| AC-O07 | Readiness reflects critical DB migration/startup state; liveness does not flap due to retailer outage |
| AC-O08 | UI surfaces last/next scheduled job, source status, suppressed alert reason and retry status |
| AC-O09 | Backup/restore help guides a non-developer operator and warns about overwrites |
| AC-O10 | Reboot/restart after storage fills reports clear error and preserves database integrity |

## 8. Memory and performance evidence
**Definition:** hard maximum = **500,000,000 bytes**; verify actual Pi cgroup v2 (or documented equivalent v1) `memory.current`, `memory.peak`, OOM counters, Docker stats, and whole-app container count. Configure a *smaller* kernel-enforced container memory limit (recommended starting point 450 MiB) and **zero additional swap allowance** where supported. Include page cache charges that cgroup tracks. Use conservative budgets, not marketing estimates.

| Case | Load profile | Pass criteria |
|---|---|---|
| AC-R01 | cold start + migrations | No OOM, peak <500 MB; responsive startup |
| AC-R02 | idle 30min | Stable memory; no unbounded goroutine/cache growth |
| AC-R03 | ten concurrent user requests + local history reads | Peak below cap, usable p95 latency, no OOM |
| AC-R04 | 100 canonical products × 5 listings, 24h synthetic/safe approved checks | Jobs processed or explicitly rate-deferred; no OOM, correct data |
| AC-R05 | parallel HTTP slow/error/huge bodies | Caps/deadlines stop growth; no OOM |
| AC-R06 | search across enabled sources with large response fixtures | Bounded candidate count and peak memory |
| AC-R07 | 1,000 history rows/product across representative watch set | Efficient paged queries; no OOM |
| AC-R08 | backup while jobs active | Valid database snapshot; no OOM |
| AC-R09 | 24h soak with user browsing, alerts, failures and periodic checks | Peak <500 MB; zero cgroup OOM events, zero runaway queue |
| AC-R10 | host already under realistic Docker load | leanPrice remains inside configured cgroup and preserves Pi safety |
| AC-R11 | intentional load above worker budget | Graceful queuing/backpressure or explicit rejection; no RAM ceiling breach |
| AC-R12 | restart/update | At most one active leanPrice runtime on Pi under budget; no overlapping old/new copies |

**Reporting:** if any function advertised as supported fails under the resource limit, the release fails. Disabling that function only for a benchmark is invalid. If 500 MB cannot be achieved, reduce scope or redesign; do not raise the ceiling.

## 9. Coverage and pilot proof
Pilot should include: an approved retailer's structured-data-only product, HTML-selectors-only page, missing-price page, standard/public/member/multibuy variations, changed structure, rate limit, invalid URL, wrong size and network failure. Fixtures must be sanitized and license-compliant; live network probes must obey published constraints.

Before declaring two “supported retailers,” record on which dates and regions each adapter was tested, what fields are actually supported, and the percentage of representative pages yielding valid results. A retailer with no permitted reliable price extraction is **unsupported**, not green.

## 10. User acceptance walkthrough
1. Launch on phone; complete first-run setup.
2. Add a supported Domestos exact-product URL; confirm identity.
3. Attach a second retailer listing (or review an intentionally ambiguous candidate).
4. Compare valid public offers and separately inspect loyalty/unknown/stale offers.
5. Create price-threshold rule; trigger deterministic simulated offer drop and confirm exactly one notification.
6. Repeat scan; no duplicate alert.
7. Simulate parser failure, 429 and oversized response; confirm honest statuses and old-price retention.
8. Take backup; recreate instance in staging; restore and compare history.
9. Inspect local performance report with idle, search and scraping peaks under 500 MB.

## 11. Traceability map
- AC-M* ↔ FR-001–009 and FR-030, NFR-007/011.
- AC-P* ↔ FR-010–020, NFR-007.
- AC-A* ↔ FR-021–029, NFR-007/012.
- AC-S* ↔ FR-030–039, NFR-006/011.
- AC-X* ↔ NFR-009/011/013.
- AC-O* ↔ FR-031–040, NFR-008/012/014.
- AC-R* ↔ NFR-001–006, NFR-010.
- UX U-01–U-10 ↔ [UX-SPEC.md](UX-SPEC.md).

## 12. Review roles and sequencing
Product Director signs off utility, semantics and priorities; UX reviewer signs off task flow, accessibility and messages; Architect signs off feasibility without diluting PRD hard constraints; Security/Privacy reviewer signs off outbound fetching, auth and secrets; Test lead signs off negative cases and repeatable evidence; Ops signs off ARM64 memory and recovery. These are review *roles* and do not imply reviewers have yet been appointed.

**Documentation release sequencing:** merge PRD + UX + this acceptance document first; only then start technical design from the merged `main` commit. Technical planning must cite these contracts, make uncertain integrations explicit, and produce implementation slices with local gates.
