# leanPrice — Product Requirements Document (PRD)
**Status:** Product baseline v1.1 — documentation only  
**Owner perspective:** Senior Product Director  
**Date:** 2026-10-18  
**Repository:** https://github.com/Onellan/leanPrice  
**Decision hierarchy:** This PRD governs the architecture and delivery plan; changes to user outcomes or hard constraints require a PRD revision before changing the implementation plan.

## 1. Executive decision
leanPrice is a **local-first, self-hosted product discovery, price comparison, historical price tracking and genuine sale-alert application**. Its starting market is South African online retail, using ZAR and retailer-specific locations/promotional terms. The primary deployment is one ARM64 Docker container on an existing Raspberry Pi 4B (4GB) already running other services.

A user can ask “Find Domestos Lavender 750 ml across supported South African websites, show comparable current offers, track the exact variant and tell me when a meaningful special appears.” leanPrice must distinguish a real promotion from a price-parsing error, a different pack size, a stale observation, a loyalty-only offer or an unavailable item.

**Hard product constraint:** the **whole Pi-hosted leanPrice stack must stay below 500,000,000 bytes of attributable memory at all times during supported operations**, not merely at idle. The starting design uses a 440,000,000-byte (440 MB decimal) container hard limit and measures any attributable use outside that cgroup against the remaining system budget. The representative normal-workload target is a cgroup peak of no more than 350,000,000 bytes. These are design targets, not performance evidence. In case of pressure, pause or slow jobs rather than silently exceed the cap. No local Chromium, Selenium, MySQL, Redis, Valkey, Node frontend server or second search container in the default runtime.

## 2. Problem and opportunity
Consumers must check multiple retailer websites to discover product availability and promotions. Names, pack sizes, prices, loyalty terms and location dependencies make manual comparison error-prone; retailers expose different page structures and occasionally block automated access. Existing heavyweight search/scraping stacks compete for limited Pi RAM. Users need a trustworthy answer with source, observation time and limitations, not an optimistic low number.

### Problem statements
- P-01: A user cannot easily find *the same* SKU/variant at several stores.
- P-02: Promotions are transient; manual checks miss discounts.
- P-03: Pack, quantity, per-unit price, membership conditions and geographic context are often hidden in comparisons.
- P-04: Price trackers can emit misleading alerts on stale, absent, malformed or blocked data.
- P-05: Heavy browser/database infrastructure consumes unacceptable resources on an already-busy Pi.

### Product promise
**Know when the right product is genuinely cheaper, where, and under what conditions — without a heavyweight always-on stack.**

### Principles
1. **Correctness over coverage.** Unsupported, blocked and uncertain listings remain unknown, never invented.
2. **Like for like.** Compare exact products first; related variants are visibly separate.
3. **Explainable offers.** Display retailer, URL, timestamp, currency, location, promotion type and caveats.
4. **Predictable resource use.** A 500 MB maximum is a release gate for every supported workload.
5. **Private by default.** Local data and no telemetry by default. Viewing saved products and history does not require a third-party service; price refresh still requires network access to an enabled, approved source.
6. **Respectful acquisition.** Prefer permissioned feeds/APIs, published metadata and low-rate HTTP; no CAPTCHA bypass or account abuse.
7. **Simple user journey.** Search → confirm match → watch → compare → act on alert.
8. **Degrade honestly.** Discovery and difficult retailers can fail independently without corrupting saved history.

## 3. Users and jobs to be done
**Primary persona — household price watcher:** uses a phone or desktop, monitors household consumables, wants quick matches and low-price notifications without managing scraping technology.
**Secondary persona — home-lab operator:** installs Docker on Pi, wants low resource usage, reliable backups, clear per-store health and no recurring services.
**Tertiary persona — comparison enthusiast (later):** wants pack-size-adjusted history and measurable savings across repeat purchases.

### JTBD
- When I search a product name, show likely matching listings with enough evidence for me to choose the correct size/variant.
- When I watch a product, periodically update prices from approved supported retailers without my intervention.
- When a *verified comparable offer* crosses my threshold, notify me once, with its conditions and a direct retailer link.
- When collection fails, distinguish unavailable, unknown, stale and blocked rather than reporting a zero price.
- When I inspect history, separate ordinary, promotion, member-only and multibuy values.
- When my Pi is busy, keep leanPrice responsive and defer noncritical jobs.

## 4. Business scope and positioning
- **Geography:** South Africa first; locale `en-ZA`, default currency ZAR. Design data types for future markets; do not claim international support in v1.
- **Channels:** retailer websites and permitted product search endpoints. No physical-store shelf-price guarantee.
- **Supported v1 categories:** common household groceries and personal-care consumables; extensible to other ordinary retail goods.
- **Retailer candidates (not yet verified):** Clicks, Dis-Chem, Checkers, Pick n Pay, Shoprite, Makro, Game, Takealot. **A name on this list is not an implemented or approved adapter.** Each requires a legal/technical feasibility assessment.
- **Deployment:** single household/self-hosted instance, authenticated admin, optional read-only local viewers later.
- **Cost:** open-source-first, no subscription needed for manual URL watching. Optional third-party discovery API may have a separate charge.
- **Open-source strategy:** original implementation in this repository, distributed under the MIT License in [`LICENSE`](../../LICENSE); avoid transplanting non-permissively licensed PriceBuddy code or branding.

## 5. Product boundaries and release slices
### MVP (R1) — trustworthy watch list
- Single administrator, safe setup, dashboard and mobile-responsive UI.
- Manual submission of a product URL from an assessed, approved and enabled source; validation and confirm-before-link. A user-submitted URL does not authorize a fetch to an unapproved or disabled source.
- Canonical products, exact offers, latest observations, comparison with location/terms.
- Periodic HTTP-only refresh and historical observations.
- Price-target and percentage-drop rules; notification via one webhook protocol initially.
- Source health and stale indicators; operator controls, backup/restore and runtime metrics.
- At least **two** independently validated retailer adapters *if compliant extraction is feasible*. If fewer than two pass, revise the release scope and claims before release; any manual URL flow remains limited to assessed, approved and enabled sources. If no source is approved, R1 release is blocked until the source scope or product workflow is revised. No fictional compatibility.

### R2 — assisted discovery and better promotion intelligence
- Product-name search spanning validated retailer-search adapters; optional remote search API integration with explicit opt-in.
- Candidate match queue, barcode/GTIN matching, normalized unit price and alternatives.
- Promotion types: regular, public special, member-only, multibuy, coupon with eligibility flags and date windows.
- Alerts for effective comparable price and planned fallback/duplicate prevention.

### R3 — broader coverage and insights
- Expand retailers after feasibility checks; history charts, savings estimates with conservative baseline, CSV export, sharing read-only views.
- Optional **remote** browser-rendering adapter outside the Pi (not default; distinct privacy/availability/cost implications).

### Non-goals for R1/R2
- Purchasing, checkout, price guarantees, automated login to retailer accounts, coupon abuse, bypass of paywalls/CAPTCHAs/anti-bot systems, physical in-store inventory, real-time minute-by-minute prices, large-scale scraping, ML models on the Pi, national retailer completeness, financial advice, commercial shopping arbitrage.

## 6. Functional requirements (traceable)
Priority notation: **M** must for R1, **S** should for R2, **L** later. IDs are stable test/backlog references.

### Discovery, products and matching
| ID | Priority | Requirement and observable result |
|---|---|---|
| FR-001 | M | Add a product listing by URL only after the corresponding source is assessed, approved and enabled; validate the destination server-side before fetching. |
| FR-002 | M | Extract title, brand, variant, quantity, pack count and barcode/GTIN when available; preserve bounded provenance (field origin/path, evidence digest and parser version), not full raw source pages. |
| FR-003 | M | Create/edit a canonical product; attach many retailer listings and mark exact, possible or rejected match. |
| FR-004 | M | Never auto-merge ambiguous variations; expose confidence and require confirmation for uncertain matches. |
| FR-005 | M | Search/filter the local catalog by name, brand, size, watched state and retailer. |
| FR-006 | S | Search across eligible retailers using permitted queries or opt-in external discovery and return deduplicated candidates. |
| FR-007 | S | Use GTIN, brand, normalized quantity, variant and name similarity in a deterministic matching pipeline. |
| FR-008 | S | Show related products separately and per-unit price for mathematically comparable quantities. |
| FR-009 | M | Support listing suspension/retirement and user removal without destroying unrelated product history. |

### Observation and offers
| ID | Priority | Requirement and observable result |
|---|---|---|
| FR-010 | M | Each observation records listing, a sanitized source URL, fetch timestamp, status, evidence digest, field provenance, currency and parser version; raw response bodies are not persisted by default. |
| FR-011 | M | A valid price has explicit currency and positive integer minor units (ZAR cents); zero/negative/parse failure is not a sale. |
| FR-012 | M | Distinguish current standard price, public special, member-only, multibuy, coupon and unknown; unknown terms cannot be silently treated as general-sale price. |
| FR-013 | M | Record stock status as in-stock, out-of-stock or unknown, with separate extraction confidence. |
| FR-014 | M | Surface latest observation age and health; stale data excluded from “current best” and new alerts. |
| FR-015 | M | Comparison views show only validated exact matches in the default ranking; other offers under separate headings. |
| FR-016 | M | Display source deep link and observed-at timestamp; never imply guaranteed checkout price or availability. |
| FR-017 | S | Support chosen locality/store context and notify when retailer prices cannot be localized. |
| FR-018 | S | Normalize pack-based unit cost with explicit units, quantities, restrictions and rounding. |
| FR-019 | S | Annotate promotion validity, minimum basket, loyalty requirements and multibuy quantity. |
| FR-020 | L | Charts of historical price distribution and robust “typical price” bands from clean observations. |

### Watches, alerts and user actions
| ID | Priority | Requirement and observable result |
|---|---|---|
| FR-021 | M | Watch/unwatch a canonical product or an individual retailer listing. |
| FR-022 | M | Configure a target absolute price and/or relative decline threshold; fresh public single-pack offers are the default, while member or multibuy eligibility must be explicitly selected with its qualifying basket cost known. |
| FR-023 | M | Alert only after a fresh, credible price observation meets the rule and applicable product equivalence. |
| FR-024 | M | Deduplicate repeated equivalent alerts by product, retailer, offer condition and episode; re-arm only after recovery or user-configured policy. |
| FR-025 | M | Show notification delivery state with retries/backoff and redacted diagnostics. |
| FR-026 | M | Provide manual “check now” with bounded queueing, plus last/next scheduled check. |
| FR-027 | S | Offer price-drop vs historical-typical-price distinction and member/multibuy-effective alerts. |
| FR-028 | S | Optional Home Assistant/ntfy/Gotify/email delivery interfaces without forcing additional containers. |
| FR-029 | M | Allow disabling individual sources, schedules, watches and alerts. |

### System, administration and transparency
| ID | Priority | Requirement and observable result |
|---|---|---|
| FR-030 | M | Secure first-run admin setup, authenticated sessions, CSRF protection, logout and recovery guidance. |
| FR-031 | M | Show product/listing/adapter state: healthy, stale, unsupported, blocked, error, paused, rate-limited. |
| FR-032 | M | Configurable check intervals, per-domain throttles, retention and global concurrency. |
| FR-033 | M | Inspect queue/scheduler status and why a listing was skipped or deferred. |
| FR-034 | M | Create and restore a versioned application backup using a SQLite-consistent snapshot; check schema compatibility before restore and preserve the current database if validation or restore fails. |
| FR-035 | M | Readiness, liveness, metrics and human-readable operational errors; logs redact secrets and personal values. |
| FR-036 | M | Explicit enablement/disablement of each retailer; no default automated scraping of unassessed sources. |
| FR-037 | S | Operator-controlled “revalidate parser” preview with sanitized fixtures and diagnostics. |
| FR-038 | M | Settings display local RAM limit and operational throttles; surface when jobs are paused for pressure. |
| FR-039 | M | Work without network after deployment for local catalog/history/settings (refresh remains unavailable). |
| FR-040 | L | CSV export of product, retailer, validated price and observation timestamp, with sensitive values excluded. |

## 7. Trust, comparisons and pricing semantics
A **canonical product** is the intended item (brand, range, scent/flavour, quantity, measure, pack count, SKU/GTIN where available). A **listing** is a retailer-specific URL/SKU for that item. An **observation** is the measured state of that listing at a point in time. An **offer** is a price with explicit conditions, currency and validity.

“Cheapest now” means lowest **fresh, comparable, available (when availability is known) offer** meeting current selected eligibility; unknown stock appears with qualification and must not outrank confirmed in-stock offers by default. A stale listing cannot win a current comparison.

**Savings claims:** if a page displays R29.99 against R39.99, show a claimed crossed-out original price only as a retailer-reported claim. A historic reduction requires leanPrice's independently observed previous price and clear timestamps. Do not convert “was R39.99” from unverified page copy into established price history.

**Special classification:** a low price is not automatically a “sale.” If no discount evidence exists, show “Price below your threshold,” not “On sale.” Member and multibuy offers remain distinct and are eligible for comparison or alerts only when explicitly selected and their qualifying basket cost is known. Coupons may be shown as conditional information, but are excluded from ranking and alerts until R2 models the full eligibility and basket cost. Unknown stock is never presented as in stock; a price-only alert may still be sent for an otherwise qualifying offer, but it must say stock is unverified.

**Freshness:** defaults are configurable; suggested current window 36h for daily monitoring. Failed fetches never overwrite the last good reading with zero; UI may show latest known price with “stale” instead of a false current price. Alerts require newly observed data, not cached data reinterpreted as fresh.

**Matching example:** Domestos Lavender 750 ml is not automatically equivalent to Domestos Original 750 ml, a 1 L bottle, a case of two, or another brand. Per-litre comparison is a separate, opt-in value lens; it does not redefine exact-item equality.

## 8. Main user journeys with acceptance intent
1. **Onboard:** Install container, open local URL, create admin credentials, choose ZA settings, optional alerts and default local store context. No retailer scraping begins until an assessed/opted-in adapter is enabled.
2. **Add by URL:** Paste a URL → validate that its source is assessed, approved and enabled and its destination is safe → preview extracted data, bounded evidence and source health → choose a canonical product or create one → confirm → queue the first check → watch.
3. **Search by name (R2):** Enter “Domestos 750ml” → show sources and timestamps → list candidate matches grouped by exact/possible/other → refine scent/size → attach chosen listings → configure watch.
4. **Compare:** Open product → see ranked comparable fresh offers plus excluded/stale/unknown groups → view public/member/multibuy conditions and per-unit prices when valid → open retailer listing.
5. **Alert:** Configure “public offer at or below R30.00” → job fetches fresh validated offer → show event evidence → send one alert with direct URL and timestamp → record delivery and cooldown.
6. **Failure:** Source 403/429, network timeout or parser change → mark blocked/rate-limited/error; backoff and explain; keep historical price; no manufactured sale alert.
7. **Operate:** View queue, source health, memory and backlog → pause adapter, force safe check, tune schedule, back up consistent SQLite snapshot, restore into compatible version.
8. **Recover:** Power loss/restart → database integrity, bounded queued job recovery and notification idempotency; no duplicate episode alerts.

## 9. UX and visual product requirements
Mobile-first information architecture: **Dashboard, Search, Products, Watches, Deals, Activity, Sources, Settings**. Global search and quick-add remain available from every primary screen; avoid dashboards full of technical jargon.

- Dashboard shows “new verified deals,” watches needing attention, source availability and last successful refresh; empty state teaches the add-product flow.
- Search displays explicit “results are discovered, not yet verified prices” badges; separates retailer candidates from saved data.
- Product detail has identity/variant, exact matches, current prices, source links, conditions, history and watch controls.
- Watch setup uses one concise panel for target price, public/loyalty eligibility, scope and notification channel.
- Activity is an append-only timeline of validations, price changes, alerts and failures with actionable status.
- Sources page makes source consent, availability, limits, parser version and last success obvious.
- Accessibility: WCAG 2.2 AA design target; real buttons/labels, keyboard navigation, visible focus, semantic tables, reduced-motion compatibility and comprehensible failure text.
- Visual language: understated, high-density enough for desktop, readable cards on mobile, currency formatted in ZAR, no sale-colored badge for uncertain data, consistent disclosure of timestamps and promotion terms.
- HTMX progressive enhancement: full-page GETs remain functional without JS; dynamic partials and polite live regions enhance responsiveness. No SPA dependency.

Detailed states and wireframes live in [UX-SPEC.md](UX-SPEC.md).

## 10. Nonfunctional and service requirements
| ID | Requirement | Release evidence |
|---|---|---|
| NFR-001 | **500,000,000-byte** maximum for all leanPrice Pi-hosted runtime processes/containers during supported workflows | Repeatable memory load test, cgroup metrics and no OOM |
| NFR-002 | Target cgroup peak <=350,000,000 bytes during representative normal load, including cgroup-charged cache, to preserve headroom | ARM64 measurements; target may be revised only by documented product decision, never the hard cap |
| NFR-003 | One ARM64 Docker service at runtime; no local browser, external database or JS server | Compose review and container inspection |
| NFR-004 | Search/catalog UI p95 <800 ms locally on warmed data under agreed test load; pages use small payloads | Local benchmarks on Pi; measure and report |
| NFR-005 | 100 monitored products × up to 5 monitored listings each; completion within 24h at permitted request rates (subject to source availability) | Synthetic + approved integration soak test |
| NFR-006 | Bounded HTTP body bytes, timeouts, retries, queue sizes and concurrent requests | Fault injection + memory report |
| NFR-007 | No inaccurate “sale” from parsing errors, zero values, stale observations or mismatched variants | Deterministic negative fixtures |
| NFR-008 | SQLite durable after forced termination; restore process proven | Integrity check, recovery and backup drill |
| NFR-009 | No embedded secrets, third-party telemetry or undocumented external request from runtime | Offline/network audit; configuration review |
| NFR-010 | No service disruption to other Pi apps under configured resource ceiling | Host observation of contention, OOM and CPU/disk |
| NFR-011 | Security baseline: authentication, CSRF, CSP, secure session cookies, SSRF prevention for retailer fetches and separately governed notification targets, rate limits, dependency review | Local security gate |
| NFR-012 | Observability: event logs, job audit, source health, memory samples and actionable failure codes | Operator acceptance suite |
| NFR-013 | Fully automated local tests/builds; GitHub Actions **not** a validation dependency | Scripts and signed-off local run report |
| NFR-014 | Explicit supported forward-upgrade paths and configuration versions; rollback uses the prior application image plus a validated pre-upgrade backup, not unreviewed reverse SQL | Fresh install, upgrade, failed-upgrade and restore drills |

Memory wording: cgroup-accounted memory is measured on the deployed host, where cache and shared-page charging affect readings. Runtime cap enforcement is set beneath 500 MB to allow operational margin. A hard limit may cause termination; **passing requires zero memory-related terminations** in defined scenarios, not merely “the container was capped.”

## 11. Retailer feasibility and governance
**Before enabling an adapter** record: source URL and version/date, public accessibility, terms/robots considerations, permitted retrieval method, requested frequency, supported location/membership context, extraction reliability, failure behaviours and test fixtures. Site policies can change. Never represent a target retailer as supported merely because its name appears in this PRD.

**Acquisition priority:** authorized official feed/API > publicly exposed structured product metadata > ordinary HTML extraction > permitted site search > optional remote rendering. A manually submitted URL remains subject to the same source assessment and destination checks as scheduled fetching. No CAPTCHA bypass, rotating identities to evade limits, password sharing or circumvention of blocking. Treat 403/429 or a disallowing source policy as a stop/backoff signal. Do not retain full product HTML by default; store bounded, redacted provenance for diagnostics. Search APIs require disclosure and opt-in.

## 12. Privacy, security, and data policy
- Default private LAN deployment. Strong initial password, session timeout, secure cookie settings for HTTPS, reverse-proxy guidance.
- No third-party analytics or cloud AI; web fetches only when user enables a source or remote discovery.
- Avoid storing passwords for retailer accounts; no checkout automation.
- SSRF block localhost, RFC1918/ULA/link-local/metadata/loopback and DNS rebinding on user-supplied URLs; allowlist supported retailer domains, redirect revalidation and strict outbound timeouts.
- A source's HTML/JSON is untrusted: sanitize displayed data, escape all templates, reject remote execution and script inclusion; cap sizes.
- Application backups contain household watch preferences and price history, so treat them as sensitive local files with restrictive permissions and documented deletion controls. Do not include live sessions or webhook credentials; invalidate sessions after restore and require external secrets to be re-provisioned from local configuration.
- User-configured notification destinations use a separate, explicit endpoint policy. Do not weaken retailer-fetch SSRF controls to permit them; allow a private-network destination only when the administrator has deliberately configured that exact destination.
- No unauthorized personal data collection. Respect retailer terms, rate limits and IP blocking rather than evading them.

## 13. Success metrics and instrumentation
The primary metric is **verified useful price alerts per watched product**, not scraping volume. Event instrumentation is local/optional and never externally reported.

| Metric | Initial product target / definition |
|---|---|
| Match precision | 100% exact-match precision on curated critical SKU/variant fixtures; reject ambiguous candidates |
| Alert precision | 0 false sale/price alerts in negative regression fixtures; manual audit of pilot live alerts |
| Coverage transparency | 100% of enabled source cards disclose capability/health/last success |
| Freshness | >=95% of *eligible, accessible* watched listings refreshed within configured interval in synthetic soak |
| Dedupe | 0 repeated notifications for same unchanged offer episode |
| Resource | 0 breach of 500 MB cgroup ceiling and 0 OOM under acceptance workloads |
| Useful UX | From paste URL to watched listing in <=4 primary actions (excluding setup/ambiguity confirmation) |
| Restorability | Backup → clean instance restore yields equivalent watches/history/settings |
| Observability | Every failed check has a category (rate-limit, access, parsing, network, invalid, source-unsupported) |

Do not interpret retailer outages as application test failures for synthetic throughput; report separately.

## 14. Roadmap, priority and release gates
**Gate G0 — discovery/feasibility:** confirm legal and technical extraction from at least two candidate sources; establish expected price semantics and fixtures. If not feasible, narrow the MVP to manually supplied compatible URLs instead of inventing coverage.  
**Gate G1 — product contract:** UX workflow, canonical identity, pricing/alerts semantics and nonfunctional acceptance signed off.  
**Gate G2 — foundation:** Go/SQLite single-container, auth, lifecycle, local scripts, resource instrumentation and backup recovery.  
**Gate G3 — tracking:** adapter pipeline, observation provenance, reliable history and exact comparison.  
**Gate G4 — alerting:** dedupe, retry/notification, clear stale/unknown semantics.  
**Gate G5 — pilot:** ARM64 24h soak, 100-product test set, failure and upgrade drills, 500 MB hard cap proven.  
**Gate G6 — discovery expansion:** retailer search and optional remote providers after pilot.

No release gate may be passed using unsupported live coverage claims or by turning off memory-intensive functions that remain advertised as supported.

## 15. Risks, mitigations and decisions
| Risk | Consequence | Response |
|---|---|---|
| Retailers block or change HTML | Missing/incorrect prices | Per-store capability registry, fixtures, circuit-breaker, health/stale flags |
| HTTP-only cannot see prices | Limited coverage | Transparent unsupported status; opt-in remote rendering outside Pi later |
| Similar packs misidentified | False savings | Canonical product model, review queue, exact-match-first |
| Promotion terms omitted | False sale claim | Qualified offer taxonomy and fail-closed “unknown” |
| Unexpected Pi memory spikes | Other services affected | Work shedding, max response size, single job worker, cgroup cap, stress tests |
| SQLite writes lock or corrupt | Lost history | Short transactions, WAL, checkpoint/backup/restore tests |
| Discovered links contain malicious targets | SSRF/data leak | DNS/IP/redirect guard and retailer allowlist |
| Over-complex scope | Late delivery | R1-first gates and explicit deferrals |
| External search costs/privacy | User distrust | Opt-in providers, local URL mode first |
| Dependency/license incompatibility | Distribution risk | SBOM, license scan, original code, permitted upstream libraries |

### Product decisions (effective v1.0)
- D-P01: Brand is **leanPrice** (exact case in marketing/UI; repo remains `Onellan/leanPrice`).
- D-P02: Use an original Go application, not a port/fork of PriceBuddy.
- D-P03: Raspberry Pi ARM64 500 MB ceiling is mandatory for the *whole on-device system*.
- D-P04: SQLite is the source of truth for products, offers, observations, jobs and alerts.
- D-P05: No mandatory local SearXNG or local Chromium. Discovery via approved adapters/manual URLs and optional remote API.
- D-P06: South African retail with exact-match, freshness and sale-proof safeguards first.
- D-P07: Validation is performed locally; no GitHub Actions required.
- D-P08: First ship dependable alerts rather than claiming support for all shortlisted retailers.

## 16. Acceptance and traceability
Each implemented requirement must have: owning slice, test ID(s), observable UI/API behavior, negative cases, resource impact, local validation evidence, reviewer and date. Release sign-off requires test evidence for every M requirement and NFR; missing evidence means *not complete*. A deferred M item requires a PRD change, never a quiet exception.

See [ACCEPTANCE.md](ACCEPTANCE.md) for end-to-end scenarios and negative cases.

## 17. Open discovery questions (not blockers to documenting the solution)
- Which exact two retailer sites will pass the feasibility gate with permitted HTTP-only extraction?
- What is the chosen pilot locale/store for location-dependent sites?
- Which permitted remote discovery API, if any, delivers acceptable coverage/cost?
- How many daily requests do each source's conditions permit?
- What web extraction evidence may be retained without conflicting with source policy?

These should be resolved by controlled **local feasibility spikes**, recorded as ADRs. Do not guess answers or silently widen scope.

## 18. Product-definition completion criteria
This specification is complete as a **planning baseline**, not a claim that the application is implemented. Completion means coherent requirements, release priorities, UX flows, pricing correctness rules, safety/resource constraints, measurable acceptance and documented unresolved feasibility questions. The implementation plan may begin **only after this product document and its supporting UX/acceptance files are merged into `main`**.
