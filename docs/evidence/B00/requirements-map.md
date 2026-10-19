# B00 — Product and technical traceability map
**Status:** requirement traceability is mapped; source-policy assessments cover five candidates with zero approved retailers; product/design sign-off remains independent and open.  
**Baseline:** product/technical docs v1.1 at start of `implementation/plan-relocation-and-full-delivery`.  
**Authority:** [PRD](../../product/PRD.md), [UX](../../product/UX-SPEC.md), [Acceptance](../../product/ACCEPTANCE.md), [Technical roadmap](../../plans/IMPLEMENTATION-PLAN.md), [Execution plan](../../plans/IMPLEMENTATION-PLAN-EXECUTION.md).

This map assigns each stable requirement and acceptance case to its implementation batch. It is not evidence that a product slice, reviewer sign-off or release gate has passed. Dated B00-S02 assessments are in `docs/evidence/sources/`; none authorizes retrieval. `B00-S03` target-Pi measurement remains an external gate.

## 1. Functional requirements
| Authority IDs | Planned implementation | Measurable gate |
|---|---|---|
| FR-001 | B03-S03, B05-S01, B06-S02, B08 | Assessment/enablement and URL/SSRF validation happen before every fetch; no unsafe target receives a request. |
| FR-002 | B04-S03/S05 | Normalized fields retain per-field origin/path, digest, parser version and fetch time; full response is not persisted by default. |
| FR-003–005, FR-009 | B04-S04/S05, B06-S02/S03 | Canonical/listing identity is explicit; uncertain matches require review; local catalog is editable, filterable and history-safe. |
| FR-006 | B10-S01–S04 | Search is opt-in, assessed, bounded, deduplicated and unavailable without affecting local catalog. |
| FR-007 | B04-S04, B10-S03 | Deterministic exact/possible/rejected decision with hard conflicts and auditable manual review. |
| FR-008, FR-018–019 | B04-S02, B06-S03, B11-S01/S02 | Compatible dimensions only; conditional eligibility, quantity and basket cost are explicit; no premature rounding. |
| FR-010–014 | B04-S03/S05 | Immutable observations, sanitized source URL, positive integer price, tri-state stock and freshness status. |
| FR-015–017 | B04-S02/S05, B06-S03, B08-S03 | Default ranking includes fresh exact comparable public offers; source and time visible; missing locality stays qualified. |
| FR-020 | B11-S03 | History summaries use valid comparable observations and disclose sample gaps. |
| FR-021–025 | B07-S01–S04 | Versioned rules, fresh evidence, deduplicated episodes, durable delivery and redacted outcomes. |
| FR-026 | B05-S02/S03, B06-S04 | Manual check coalesces into bounded durable work with last/next status. |
| FR-027 | B11-S02/S03 | Member/multibuy effective alert semantics are explicit and backed by comparable observations. |
| FR-028 | B07-S02, B11-S04 | Optional channels share bounded outbox policy; no mandatory additional service. |
| FR-029 | B05-S01/S03, B07-S03 | Source, schedule, watch and alert disablement is durable and audited. |
| FR-030 | B03-S01/S02 | One-use setup, authenticated sessions, CSRF-protected mutations, logout and recovery behavior. |
| FR-031–033 | B05-S01–S03, B06-S04 | Source, scheduler and skipped/deferred-job state is inspectable and truthful. |
| FR-034 | B02-S03, B09-S03 | Versioned consistent backup; compatibility/integrity validation; current DB preserved on restore failure. |
| FR-035 | B01-S01, B06-S04 | Readiness/liveness and human-safe diagnostics distinguish app state from retailer availability. |
| FR-036–037 | B05-S01, B06-S04, B08-S03/S04 | Explicit approval/enablement and a safe parser revalidation preview; no default unassessed fetch. |
| FR-038 | B05-S04, B06-S04 | Settings show enforced local budget and visible pause/defer state. |
| FR-039 | B01-S01/S02, B06-S04 | Catalog/history/settings remain local when offline; refresh reports unavailable. |
| FR-040 | B13-S03 | Implemented bounded CSV export streams validated exact public history, escapes spreadsheet formulas and excludes source URLs/secrets; see B13 evidence. |

## 2. Nonfunctional requirements
| Authority IDs | Planned implementation | Measurable gate |
|---|---|---|
| NFR-001/002/010 | B01-S03, B05-S04, B09-S02, B12-S02; repeat for B13/B14 changes | Actual target-Pi attributable peak below 500,000,000 bytes; starting cgroup cap 440,000,000 bytes; representative cgroup target 350,000,000 bytes; zero OOM and no harm to existing services. |
| NFR-003 | B01-S03 | Exactly one default runtime service; inspect image/Compose for banned local infrastructure. |
| NFR-004 | B06-S02/S03, B09-S02 | Warmed local p95 below 800 ms under the agreed representative workload. |
| NFR-005 | B05-S03, B08, B09-S02 | Safe synthetic/approved 100-product × 5-listing cycle within 24h subject to allowed source quotas. |
| NFR-006 | B03-S03, B04-S01, B05-S02/S04 | Bounded body, timeout, retry, queue and concurrency fault cases. |
| NFR-007 | B04-S02–S05, B07-S01, B09-S01 | Negative price, stale data, identity mismatch and offer-condition fixtures create no misleading alert. |
| NFR-008 | B02-S01–S04, B09-S03 | Crash, migration, WAL, backup, restore and integrity cases pass. |
| NFR-009/011 | B03-S02–S04, B04-S01, B07-S02 | Auth, SSRF, separate webhook policy, redaction, no telemetry and local dependency review. |
| NFR-012 | B01-S01, B05-S01–S04, B06-S04, B07-S02 | Auditable events, source health, resource samples and actionable failure codes. |
| NFR-013 | B01-S04 | Reproducible local Make targets and evidence; no GitHub Actions requirement. |
| NFR-014 | B02-S01/S03, B09-S03 | Supported forward upgrades and rollback with prior image plus validated pre-upgrade backup; no reverse SQL downgrade. |

## 3. Acceptance cases
| Acceptance IDs | Planned implementation | Primary validation |
|---|---|---|
| AC-M01–M03 | B03-S03, B05-S01, B06-S02 | Source/URL preflight and hostile-IP/redirect fixtures. |
| AC-M04–M07, M10–M11 | B04-S04/S05, B06-S02 | Variant/quantity/GTIN conflicts, idempotent URL, actor/time/rationale and immutable old alert evidence. |
| AC-M08–M09 | B10-S01–S04 | Offline catalog, bounded deduplicated candidate review. |
| AC-P01–P03 | B04-S02/S03/S05 | Provenance, exact cents conversion, invalid price/currency failure. |
| AC-P04–P07 | B04-S02/S05, B06-S03 | Threshold is not a sale claim; freshness and member eligibility. |
| AC-P08–P09, P15 | B04-S02/S03, B06-S03, B11-S02 | Full multibuy basket/quantity, coupon exclusion and unknown eligibility. |
| AC-P10–P14 | B04-S02/S05, B06-S03, B08 | Stock, dedupe, unverified discovery price and currency separation. |
| AC-A01–A05, A11–A16 | B07-S01/S03/S04 | Rule defaults, episode/rearm, stale suppression, unknown stock qualifier and multibuy basket target. |
| AC-A06–A08, A14 | B07-S02/S04 | Backoff, bounded retry, outbox replay and single logical event. |
| AC-A09–A10, A12–A13 | B07-S01–S03 | Pause semantics, test-notification distinction, member scope and rule-version trace. |
| AC-S01–S07, S11–S12 | B00-S02, B04-S01/S03/S05, B05-S01, B08 | Approved-source-only retrieval, hostile/slow/oversize/error cases, re-assessment and rate policy. |
| AC-S08–S10 | B05-S02/S03 | Coalescing, lease restart recovery and offline operation. |
| AC-X01–X04, X07–X09, X14 | B03-S01/S02, B06 | Setup, session, CSRF, escaping, proxy, roles, throttling and POST-only logout. |
| AC-X05 | B03-S03, B04-S01 | Scheme, address, DNS and redirect revalidation. |
| AC-X06, X10, X12 | B03-S04, B01-S04 | Redaction, no telemetry, local dependency/license/vulnerability evidence. |
| AC-X11, AC-O11 | B02-S03, B09-S03 | Invalid/incompatible artifacts rejected; replacement failure recovers safety DB. |
| AC-X13 | B07-S02 | Exact notification destination policy, no retailer SSRF relaxation. |
| AC-O01–O02, O04–O06, O10 | B01-S03, B02-S01/S02/S04, B09-S03 | ARM64 install, supported upgrade, crash, transactions, retention and storage failure. |
| AC-O03, O07–O09 | B02-S03, B01-S01, B06-S04 | WAL-safe restore, readiness/liveness, visible operations and help. |
| AC-O12 | B13-S03 | Authenticated bounded export filters validated public history and formula-escapes free text; local storage/HTTP tests pass, independent review remains open. |
| AC-R01–R12 | B01-S03, B05-S04, B09-S02, B12-S02 | Pi cold start through stress, backup, soak and one-runtime upgrade. |

## 4. UX scenarios
| UX IDs | Planned implementation | Primary validation |
|---|---|---|
| U-01–U-05 | B06-S02/S03, B07-S01 | Mobile approved-source add, exact variants, eligibility, stale state and rate-limit copy. |
| U-06–U-07 | B01-S02, B06 | No-JS navigation, keyboard, accessible forms and focus. |
| U-08 | B07-S02/S03 | Fixed test payload and explicit authenticated action are distinct from price alerts; mock transport/CSRF/redaction tests pass. |
| U-09–U-10 | B06-S03/S04, B05-S04 | Coverage caveats, resource defer state and usable UI. |
| U-11–U-14 | B06-S03, B07-S01/S03 | Multibuy basket display/watch, stock-qualified alert, coupon exclusion and quantity-aligned watch target. |

## 5. Open gates and decisions
- **Source policy:** B00-S02 assessments cover Takealot, Clicks, Makro, Checkers and Dis-Chem. Current terms or missing policy evidence block all five; no source has been approved. B08/live fetch work is blocked until policy and separate approval gates pass.
- **Pi capacity:** no target Pi is attached to this workspace. AC-R and R1/R2 release certification remain blocked until B00-S03 and actual ARM64 workload evidence are available.
- **Toolchain:** Go 1.27.2, Docker and Make are pinned/documented and local B01 checks pass; see `docs/evidence/B01/toolchain-and-foundation.md`.
- **Project license:** owner selected MIT on 2026-10-19 and the repository records it in [`LICENSE`](../../../LICENSE). `make license-report` now reports first-party packages as MIT; `modernc.org/mathutil` remains scanner-Unknown and is manually verified as BSD-3-Clause in the notices.
- **Rendered UI inspection:** no local browser is available; SSR, assets, keyboard semantics and responsive source were inspected, but rendered screenshot/accessibility review remains an independent gate.
- **Independent review:** no independent reviewer/tester identity is recorded. Implementer checks cannot close the review gates.
- **Remote renderer:** no remote host, protocol credentials or source permission is supplied; B14 cannot progress beyond a bounded feasibility/design artifact without them.

