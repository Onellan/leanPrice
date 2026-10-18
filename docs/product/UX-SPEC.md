# leanPrice — UX and Interaction Specification
**Version:** 1.1 | **Status:** Product baseline | **Parent:** [PRD](PRD.md)  
**Brand:** leanPrice | **Design voice:** calm, evidence-led, resource-conscious  
**Surfaces:** desktop browser and mobile browser; self-hosted LAN-first.

## 1. Experience goals
- Users can get from **“I want to track Domestos”** to a watched exact SKU without reading technical documentation.
- No visual or verbal cue should imply a deal was verified unless evidence supports it.
- Technical health should be inspectable but never obscure a simple price comparison.
- The UI must remain responsive when a source is blocked, offline, slow or paused for memory.
- A pasted URL is fetched only when its source is assessed, approved and enabled; manual submission never bypasses that gate.
- Every destructive action has a confirmation/undo path where possible.
- Use semantic server-rendered HTML; HTMX provides enhancements, not indispensable navigation.

## 2. Information architecture
| Area | Primary user question | Main actions | Empty state |
|---|---|---|---|
| Dashboard | “Anything worth buying or fixing?” | Open deal, add product, review stale sources | “Add a product URL to start watching prices” |
| Search (R2) | “Who sells this exact item?” | Search, refine by variant, confirm matches | “No verified results; paste a retailer URL” |
| Products | “What products do I monitor?” | Filter, open, edit, archive, add listing | “Your saved products appear here” |
| Watches | “When should I be notified?” | Edit threshold, pause, test delivery | “Choose a product to watch” |
| Deals | “Which fresh offers qualify?” | Filter eligibility, inspect conditions, retailer link | “No verified qualifying offers” |
| Activity | “What changed or failed?” | Timeline, inspect evidence/failure, retry safely | “Price checks and alerts will appear here” |
| Sources | “Are retailers working?” | Enable approved source, pause, inspect restrictions and health | “No retailer adapters assessed yet” |
| Settings | “Can I trust/operate this?” | Admin, notifications, schedules, backup/restore | Settings grouped by task, advanced collapsed |

Navigation: a compact left rail on desktop; bottom/slide-out navigation on small screens, with Search/Add visible globally. Use current-page title and concise breadcrumbs for drilldown. Never require more than one primary CTA per view.

## 3. Key page contracts
### Dashboard
**Above the fold:** summary “Verified deals”, “Watched products”, “Needs attention” and last completed check. Deal cards include full SKU/variant + pack size, retailer, public/member qualifiers, price in R, last observed time, deep link and reason for the alert. Never display a stale price as current.

**Below:** watches with last result, recent events and source health. On small displays, stack cards and put dense history tables behind a “View all” link.

### Search
Input label “Search products”; example “Domestos Lavender 750 ml”. Results grouped as **Exact matches**, **Possible matches (review needed)**, **Other variants** and **Unavailable/unverified listings**. Grouping is based on evidence and cannot be overridden by UI styling. Every candidate carries retailer, extracted name, size, candidate timestamp and “Not yet verified” when applicable. Bulk attach is gated behind match confirmation.

Search result states: loading/scheduled, no results, search provider unavailable, source disabled, source disallowed, quota exhausted, partial results. Show per-retailer coverage. Search must never promise it indexed “every website.”

### Add product / URL wizard
1. Paste URL; immediately validate scheme, hostname and approved/enabled source, without fetching unvalidated or unapproved targets.
2. Show extraction preview: item title, brand, quantity/pack, currency/price, source and timestamp, extraction confidence. R1 does not load retailer images directly in the user's browser; any later image support must use an assessed, safe delivery path.
3. Choose exact existing product, create a new canonical product, or “not the same.” Require user confirmation if identity data is incomplete.
4. Offer optional watch threshold and schedule; save without forcing an alert rule.
5. Show success state with canonical product and initial collection status; source limitations are visible.

Back/cancel actions do not create partially attached listings. A retry is bounded, idempotent and respects circuit-breakers.

### Product detail
Header: **Domestos Lavender — 750 ml — 1 bottle** (illustrative). Tabs/sections: Offers, History, Listings, Watch settings. Display current comparable offers in consistent currency and eligibility context, with independent sections for member-only/multibuy/non-equivalent/unknown. Default sort chooses known in-stock, fresh exact matches first; user can show unknown stock separately.

Offer row: retailer, standard/current price, contextual struck-through *retailer claim*, promotion conditions, stock, observation age, unit price when eligible, external link. A multibuy total is shown with its minimum qualifying quantity; its effective per-pack price is never substituted for a single-pack price. Each value exposes source and timestamp by opening “Details.”

### Watch rules
Form controls: canonical product or listing scope; **public single-pack offers** (default), with explicit member or multibuy eligibility only when the source and qualifying cost are known; target price ZAR; optional % drop, frequency and notification channel. When multibuy is selected, show the full qualifying basket total and quantity and apply the target to that total. Do not compare baskets with different quantities. Coupon offers remain informational and are not alert-eligible in R1. If both price conditions are enabled, UI exposes “either”/“both,” default “either,” matching the versioned evaluation contract. Show preview using the last valid observation *without sending an alert*. “Send test notification” is a separate, explicit action.

### Deals
Card headings indicate **“Verified price below target”**, **“Observed price decrease”**, or **“Retailer-labelled promotion”** based on evidence. Do not conflate these terms. Filters include retailer, freshness, promotion condition, product, alert status and stock. Click-to-store links open an external tab with no automatic checkout.

### Sources and operational health
Each retailer displays capability (manual-URL, search, watch, special-terms), enabled state, terms/feasibility decision date, last verified extraction, failure category, next permitted retry and local constraints. Action buttons: enable/disable if approved, pause/resume, inspect parser version, safe test with fixture. Unsupported source shows read-only explanation, not a fake “connect” button.

### Activity
Chronological timeline for saved product, price observation, parser warning, match decision, triggered alert, suppressed duplicate, failed notification and backup/migration. Each has an accessible reason code. Avoid exposing raw retailer response bodies or credentials.

### Settings
Sections: Household locale (ZAR/region), Sources, Schedules and concurrency, Notifications, Backups, Security, Resource usage. Default resource profile is “Pi / under 500 MB”; unsupported combinations are rejected with an explanation rather than silently accepted. Restore accepts only a documented leanPrice backup, checks version and integrity before replacement, invalidates existing sessions and requires clear confirmation. External notification secrets are re-provisioned from local configuration.

## 4. Reusable design system
- **Color:** neutral surfaces for ordinary cards; restrained green only for *verified eligibility*, amber for source uncertainty, red for errors; do not use red sale prices for unconfirmed discounts.
- **Typography:** system stack; high readability; no font downloads required at runtime.
- **Spacing:** consistent 4px scale; responsive 320px–desktop; tables horizontally scroll or collapse intentionally.
- **Components:** product identity chip, price-with-conditions, source pill, freshness timestamp, watch rule summary, confidence indicator, comparison row, alert episode card, error banner, empty state, confirmation dialog and notification outcome.
- **Icons:** meaningful actions only; no icon-only critical controls without accessible name.
- **Forms:** explicit labels, examples, keyboard submission, inline validation and preservation of draft state on recoverable errors.
- **Format:** ZAR “R 29.99” where locale applies, machine integer cents, relative+absolute timestamps; units “ml”, “L”, “g”, “kg”, count/pack.
- **Responsive:** 44px touch target guideline, source evidence remains accessible on mobile, no hidden savings caveats.

## 5. Standard copy and data truthfulness
| Situation | Correct copy | Avoid |
|---|---|---|
| Verified threshold | “Below your R30 target: R29.99 at Store X; checked 3 min ago” | “Guaranteed lowest” |
| Retailer says sale | “Retailer marks this as a special; original price not independently verified” | “You saved R10” |
| Price stale | “Last observed R29.99 two days ago — not current” | “Now R29.99” |
| Offer requires loyalty | “Member price; card required” | Unqualified “R29.99” |
| Same brand, different size | “Other size: 1 L; shown separately” | Adding it to 750 ml exact matches |
| Rate-limited | “Retailer temporarily restricting checks. Next attempt after …” | “Product unavailable” |
| Blocked/unpermitted | “Automatic tracking is not supported for this site” | “Try bypassing protection” |
| Unknown stock | “Stock not verified”; a price alert must carry this qualifier | “In stock” or an unqualified “available” claim |
| Conditional coupon | “Coupon terms apply; excluded from R1 ranking” | Treating a coupon as a public single-item price |
| No eligible deal | “No qualifying verified offer currently” | “No deals anywhere” |
| Partial results | “3 of 5 enabled sources responded” | “All stores checked” |

## 6. Core interaction states and HTMX behaviour
**SSR first:** GET navigation always returns complete HTML; HTMX request detection can return only partial markup. HX endpoints include CSRF checks and server validation; history/deep links remain meaningful. No redirect loops or duplicate forms when HTMX is unavailable. Return structured status codes (200/202/400/401/403/404/409/422/429/503) with HTML fragments and relevant `HX-Trigger` events where appropriate.

| Operation | Pending | Success | Failure |
|---|---|---|---|
| Add URL | “Checking URL…” with cancel | Preview and confirm | Unsupported/unsafe/unavailable; no partial save |
| Run check | “Queued” + position/ETA omitted | Timestamp and observation | Backoff/policy/parser error with reason |
| Save watch | Disabled submit only during request | Rule summary + server timestamp | Validation error preserving input |
| Product search | “Searching enabled sources…” | Grouped candidates and coverage | Partial list + per-source issues |
| Deliver test | “Sending test…” | Channel accepted | Redacted delivery failure |
| Restore | Explicit irreversible warning + backup validation | Integrity verified / restart notice | Previous database preserved |

Polling: bounded HTMX polls for short operations; cancel after completion, not endless every-second polling. Use accessible live region for check/notification updates. Never stream full scraped HTML to UI.

## 7. Accessibility and inclusion
WCAG 2.2 AA target: high contrast, focus management after partial swaps, errors announced and associated with inputs, semantic comparison tables, screen-reader-friendly status changes, no color-only differentiation, zoom to 200%, reduced motion. Locale-sensitive unit and currency formatting; readable absolute dates in tooltips/details.

## 8. Critical UX acceptance scenarios
- U-01: On 375px mobile width, user adds a valid URL, confirms identity, starts watch in <=4 primary actions after onboarding.
- U-02: Domestos Lavender 750 ml appears separately from Original 750 ml and Lavender 1 L; candidate distinction accessible.
- U-03: An unknown member condition cannot appear as a public cheapest offer.
- U-04: Two-day-old price prominently states stale and cannot trigger a new deal.
- U-05: Source 429 displays rate-limited status/next permitted attempt, not an incorrect stock state.
- U-06: Disabled JavaScript retains navigation, basic forms and price/history access.
- U-07: Keyboard-only user can reach all pricing evidence and settings; screen reader announces save errors.
- U-08: Notification test and real deal send are visibly distinct.
- U-09: Product detail clearly says when only some retailers are covered; no universal “best price” claim.
- U-10: Low-memory paused jobs show deferred status with operator controls, without UI freeze.
- U-11: A 2-for-R50 offer shows R50 for two packs and may show R25 per pack only in a qualifying two-pack comparison; a R30 single-pack offer remains the one-pack price.
- U-12: An offer with unknown stock can show a qualifying price alert only with “stock not verified”; it is never presented as confirmed in stock or available.
- U-13: A coupon with incomplete or unsupported basket eligibility appears as conditional information, not as a public ranked offer or R1 alert.
- U-14: A multibuy watch shows its qualifying quantity and basket total; its target applies to that total, and different-quantity baskets are kept separate.

## 9. Prototype direction (ASCII, not final UI)
```text
leanPrice        Search products [____________________]  [+ Add URL]
--------------------------------------------------------------
Verified deals  [ 2 ]  Watched [ 18 ]  Source issues [ 1 ]

Recent verified offers
[Domestos Lavender 750 ml]
 R29.99  Store X · public price · 3h ago        [View details]
 Below your R30 watch target · stock not verified
 -------------------------------------------------------------
[No other qualifying deals]

Watches                              Source health
Domestos Lavender 750 ml ...         Store X   OK · 3h ago
Product B …                          Store Y   Rate limited
Product C …                          Store Z   Not assessed
```

The actual interface must be designed with components/flows first, reviewed against accessibility and correctness criteria, and implemented with server-rendered templates plus progressively enhanced HTMX.

## 10. Required design deliverables in implementation
Site map, low/high fidelity desktop/mobile layouts for every primary screen, component inventory, state matrix, responsive rules, copy glossary, screenshot tests and keyboard/accessibility walkthrough. Each page must link back to PRD FR IDs and UX U-01–U-14. Designs should document partial failure and staleness, not just happy-path cards.
