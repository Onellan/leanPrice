# leanPrice — Data Model, API and Domain Contracts
**Status:** v1.1 technical contract; SQLite schema v4, pricing/matching/extraction, paginated product/history/activity views, product/watch workflows, source/operator controls, sampled history chart and bounded export are implemented locally; source-backed and external acceptance gates remain open  
**Depends on:** merged [PRD](../product/PRD.md), [UX](../product/UX-SPEC.md), [Acceptance](../product/ACCEPTANCE.md) and [Architecture](ARCHITECTURE.md).  
**Source-of-truth rule:** changes in pricing/business meaning require product-document revision first.

## 1. Core invariants
1. **No floating point money.** Amounts use positive integer minor units plus an explicit currency code and minor-unit scale. ZAR uses scale 2 (cents). Only currencies with an implemented scale are accepted; cross-currency ranking or alert comparison is prohibited without a conversion policy.
2. **No zero-price sale.** Price <=0, absent currency, ambiguous product or unassessed source cannot create a valid alert.
3. **Canonical product != retailer listing != observation != qualified offer.** These entities are separate with explicit foreign keys.
4. **No guess as known.** Stock/offer eligibility/confidence can be unknown. Unknown never becomes true during ranking.
5. **Immutable observation lineage.** A new collection creates a new attempt and observation; earlier history is not overwritten because a later parser fails.
6. **Every alert traces** to one validated observation, canonical match decision, rule version, eligible offer and notification episode.
7. **All user-supplied network targets are unsafe** until the corresponding source is assessed and approved, enabled by an administrator, and SSRF-validated, including every redirect and DNS re-resolution.
8. **One DB, local transactions.** Never hold write lock during HTTP or webhook call.
9. **Database schema changes are explicit, monotonic and tested** with fresh/upgrade/restore fixtures.

## 2. Domain entities and cardinalities
```mermaid
erDiagram
  RETAILERS ||--o{ SOURCES : exposes
  SOURCES ||--o{ LISTINGS : contains
  CANONICAL_PRODUCTS ||--o{ LISTINGS : matched_to
  LISTINGS ||--o{ OBSERVATIONS : measured_by
  OBSERVATIONS ||--o{ OBSERVED_OFFERS : yields
  CANONICAL_PRODUCTS ||--o{ WATCHES : watched
  WATCHES ||--o{ WATCH_RULE_VERSIONS : versioned_as
  WATCH_RULE_VERSIONS ||--o{ ALERT_EPISODES : triggers
  OBSERVED_OFFERS ||--o{ ALERT_EPISODES : evidence
  ALERT_EPISODES ||--o{ DELIVERY_OUTBOX : sends
  SOURCES ||--o{ JOBS : checked_by
  LISTINGS ||--o{ JOBS : checked_by
```

- `retailers`: brand identity (Clicks, Dis-Chem etc) without implying an adapter exists.
- `sources`: integration/locale-specific endpoint capability, review status and rate policy. A retailer may have different sources/sites.
- `canonical_products`: user-approved exact product identity.
- `listings`: specific URL/product SKU and match status to a canonical product; unconfirmed candidate may be unattached.
- `observations`: immutable attempt result with extraction facts, metadata and status, including errors.
- `observed_offers`: zero or more price/eligibility terms associated with successful observation.
- `watches`, `watch_rule_versions`: user intent and immutable rule logic versions.
- `alert_episodes`: deduped threshold/promotion event tied to verified offer.
- `delivery_outbox`: eventual delivery work/receipt.
- `jobs`: durable scheduled/adhoc tasks and leases, deduped keys.
- `source_events`, `match_reviews`, `admin_audit`: meaningful state transitions and manual decisions.
- `settings`, `admin_users`, `sessions`: private single-instance metadata and authentication.

## 3. Reference DDL and authoritative migrations
Types are SQLite primitives; IDs are opaque TEXT UUID/ULID unless justified otherwise, all timestamps UTC epoch milliseconds INTEGER, all prices INTEGER cents. The migration author must add CHECK and foreign-key constraints equivalent to those shown and test all indexes with query plans. Canonical quantity is a positive rational normalized to its dimension's base unit (`L`, `kg`, or `item`); the pack count is separate. The database and service must reject inconsistent dimension/unit pairs. A retailer is descriptive; only an individual source can be assessed, enabled, paused, or rate-limited. Enabling requires `assessment_state='approved'`; pausing disables the source without erasing its assessment history.

```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE retailers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE sources (
  id TEXT PRIMARY KEY,
  retailer_id TEXT NOT NULL REFERENCES retailers(id),
  source_key TEXT NOT NULL UNIQUE,
  base_host TEXT NOT NULL,
  locale TEXT NOT NULL DEFAULT 'en-ZA',
  currency TEXT NOT NULL DEFAULT 'ZAR',
  capability_json TEXT NOT NULL DEFAULT '{}',
  assessment_state TEXT NOT NULL DEFAULT 'unassessed'
    CHECK (assessment_state IN ('unassessed','review_required','approved','disallowed','unsupported')),
  assessment_ref TEXT,
  assessment_at INTEGER,
  enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
  min_interval_ms INTEGER NOT NULL DEFAULT 10000,
  next_allowed_at INTEGER NOT NULL DEFAULT 0,
  parser_version TEXT NOT NULL DEFAULT '0',
  CHECK (enabled = 0 OR assessment_state = 'approved'),
  CHECK (assessment_state != 'approved' OR
    (assessment_ref IS NOT NULL AND assessment_at IS NOT NULL))
);
CREATE TABLE canonical_products (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  brand TEXT, product_line TEXT, variant TEXT,
  quantity_numerator INTEGER NOT NULL CHECK (quantity_numerator > 0),
  quantity_denominator INTEGER NOT NULL CHECK (quantity_denominator > 0),
  quantity_dimension TEXT NOT NULL CHECK (quantity_dimension IN ('volume','mass','count')),
  quantity_unit TEXT NOT NULL CHECK (quantity_unit IN ('L','kg','item')),
  pack_count INTEGER NOT NULL DEFAULT 1 CHECK (pack_count > 0),
  gtin TEXT,
  archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0,1)),
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  CHECK ((quantity_dimension = 'volume' AND quantity_unit = 'L') OR
    (quantity_dimension = 'mass' AND quantity_unit = 'kg') OR
    (quantity_dimension = 'count' AND quantity_unit = 'item'))
);
CREATE TABLE listings (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES sources(id),
  canonical_product_id TEXT REFERENCES canonical_products(id),
  external_sku TEXT,
  url TEXT NOT NULL,
  url_key TEXT NOT NULL,
  location_key TEXT NOT NULL DEFAULT '',
  match_state TEXT NOT NULL DEFAULT 'candidate'
    CHECK (match_state IN ('candidate','exact','possible','rejected','suspended')),
  match_reason TEXT,
  match_algorithm TEXT,
  first_seen_at INTEGER NOT NULL,
  last_checked_at INTEGER,
  UNIQUE (source_id, url_key, location_key),
  UNIQUE (id, canonical_product_id),
  CHECK (match_state != 'exact' OR canonical_product_id IS NOT NULL)
);
CREATE INDEX idx_listings_product_match
  ON listings(canonical_product_id, match_state);
CREATE TABLE observations (
  id TEXT PRIMARY KEY,
  listing_id TEXT NOT NULL REFERENCES listings(id),
  job_id TEXT,
  observed_at INTEGER NOT NULL,
  fetch_status TEXT NOT NULL,
  http_status INTEGER,
  parser_version TEXT,
  evidence_digest TEXT,
  evidence_location TEXT,         -- JSON-LD path or selector, NOT full HTML
  observed_title TEXT,
  observed_gtin TEXT,
  observed_quantity_json TEXT,
  observed_stock TEXT NOT NULL DEFAULT 'unknown'
    CHECK (observed_stock IN ('in_stock','out_of_stock','unknown')),
  failure_code TEXT,
  source_url TEXT NOT NULL,
  validation_state TEXT NOT NULL
    CHECK (validation_state IN ('valid','invalid','uncertain')),
  CHECK (length(source_url) > 0)
);
CREATE INDEX idx_obs_latest ON observations(listing_id, observed_at DESC);
CREATE TABLE observed_offers (
  id TEXT PRIMARY KEY,
  observation_id TEXT NOT NULL REFERENCES observations(id),
  offer_kind TEXT NOT NULL CHECK
    (offer_kind IN ('standard','public_special','member','multibuy','coupon','unknown')),
  currency TEXT NOT NULL CHECK (length(currency) = 3 AND currency NOT GLOB '*[^A-Z]*'),
  currency_scale INTEGER NOT NULL CHECK (currency_scale BETWEEN 0 AND 3),
  amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
  original_claim_minor INTEGER,
  minimum_qty INTEGER NOT NULL DEFAULT 1 CHECK (minimum_qty > 0),
  membership_label TEXT,
  coupon_condition TEXT,
  valid_from INTEGER, valid_until INTEGER,
  offer_evidence TEXT NOT NULL,
  eligibility_state TEXT NOT NULL
    CHECK (eligibility_state IN ('public','conditional','unknown')),
  CHECK (offer_kind = 'multibuy' OR minimum_qty = 1),
  CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until >= valid_from)
);
CREATE INDEX idx_offers_by_observation ON observed_offers(observation_id);
CREATE TABLE watches (
  id TEXT PRIMARY KEY,
  canonical_product_id TEXT NOT NULL REFERENCES canonical_products(id),
  listing_id TEXT REFERENCES listings(id),
  active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
  created_at INTEGER NOT NULL,
  current_rule_version INTEGER NOT NULL DEFAULT 1,
  FOREIGN KEY (listing_id, canonical_product_id)
    REFERENCES listings(id, canonical_product_id)
);
CREATE TABLE watch_rule_versions (
  watch_id TEXT NOT NULL REFERENCES watches(id),
  version INTEGER NOT NULL,
  target_minor INTEGER CHECK (target_minor > 0),
  drop_basis_points INTEGER CHECK (drop_basis_points BETWEEN 1 AND 9900),
  condition_operator TEXT NOT NULL DEFAULT 'any'
    CHECK (condition_operator IN ('any','all')),
  allow_member INTEGER NOT NULL DEFAULT 0 CHECK (allow_member IN (0,1)),
  allow_multibuy INTEGER NOT NULL DEFAULT 0 CHECK (allow_multibuy IN (0,1)),
  notification_channel TEXT CHECK
    (notification_channel IS NULL OR notification_channel IN ('in_app','webhook')),
  rearm_policy_json TEXT NOT NULL DEFAULT '{}',
  created_at INTEGER NOT NULL,
  PRIMARY KEY (watch_id, version),
  CHECK (target_minor IS NOT NULL OR drop_basis_points IS NOT NULL),
  CHECK (condition_operator != 'all' OR
    (target_minor IS NOT NULL AND drop_basis_points IS NOT NULL))
);
CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  job_kind TEXT NOT NULL,
  source_id TEXT REFERENCES sources(id),
  listing_id TEXT REFERENCES listings(id),
  dedupe_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL
    CHECK (status IN ('queued','leased','retry_at','completed','blocked','dead')),
  attempt_count INTEGER NOT NULL DEFAULT 0,
  due_at INTEGER NOT NULL,
  lease_until INTEGER,
  leased_by TEXT,
  last_error_code TEXT,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE INDEX idx_jobs_due ON jobs(status, due_at);
CREATE TABLE alert_episodes (
  id TEXT PRIMARY KEY,
  watch_id TEXT NOT NULL REFERENCES watches(id),
  rule_version INTEGER NOT NULL,
  observed_offer_id TEXT NOT NULL REFERENCES observed_offers(id),
  episode_key TEXT NOT NULL UNIQUE,
  reason_code TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  closed_at INTEGER,
  FOREIGN KEY(watch_id,rule_version)
    REFERENCES watch_rule_versions(watch_id,version)
);
CREATE TABLE delivery_outbox (
  id TEXT PRIMARY KEY,
  alert_episode_id TEXT NOT NULL REFERENCES alert_episodes(id),
  destination_key TEXT NOT NULL,
  idempotency_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL
    CHECK (status IN ('pending','leased','retry_at','delivered','dead')),
  attempt_count INTEGER NOT NULL DEFAULT 0,
  next_attempt_at INTEGER NOT NULL,
  last_status_code INTEGER,
  last_error_code TEXT,
  delivered_at INTEGER,
  UNIQUE (alert_episode_id, destination_key)
);
```

This DDL is a **reference sketch**, not the executable schema. The authoritative forward-only schema is in `migrations/0001_core.sql` through `0004_watch_notification_channel.sql`; migration tests enforce its checksums, constraints and upgrade behavior. Compare any contract change against those files and add a new migration rather than editing an applied migration. The SQLite composite foreign keys enforce that an optional selected listing belongs to the canonical product and source. Storage transactions additionally enforce alert lineage from an exact eligible offer through the current watch rule and episode, including the selected notification destination where one is recorded. Network calls remain outside write transactions.

## 4. Normalized extraction contract
A fetcher returns immutable bounded response metadata; adapter parsing never performs network calls behind a handler's back. Every parser returns an extraction plus per-field origin and warnings. Persist only the normalized facts, field origins, evidence digest, bounded selector/JSON-LD path and sanitized source URL needed to explain a decision. Do not persist full response bodies or raw HTML by default. URLs must remove userinfo and redact credential-like query values while retaining only assessed SKU-identifying parameters; redirects are revalidated before the final sanitized URL is recorded.

```go
type ExtractedPrice struct {
    Currency          string
    AmountMinor       int64
    Kind              OfferKind
    OriginalClaimMinor *int64
    MinPurchaseQty    uint32
    Membership        *string
    ValidFrom, ValidTo *time.Time
    EvidencePath      string
    Origin            FieldOrigin
}
type FieldOrigin struct {
    Kind              string // json_ld, meta, selector, or reviewed_manual
    Path              string // bounded selector or structured-data path
    Confidence        Confidence
}
type Extraction struct {
    ObservedAt        time.Time
    SourceURL         string
    Title, Brand      string
    Variant, GTIN     string
    Quantity          Quantity
    Stock             StockState
    Offers            []ExtractedPrice
    Confidence        Confidence
    ParserVersion     string
    EvidenceDigest    string
    Warnings          []ErrorCode
    FieldOrigins      map[string]FieldOrigin
}
```

Price precedence is not hard-coded globally: adapter supports explicit field origin metadata; inconsistent markup triggers a `PRICE_CONFLICT` rather than choosing lowest. Persisted evidence is bounded and sanitized; fixtures may contain sanitized excerpts only where rights allow. PII and secrets never enter evidence fields. Decimal-to-minor-unit conversion must be exact for the currency or fail. Rational quantity and effective basket comparisons use checked integer arithmetic (or overflow-safe cross multiplication); overflow is a validation failure, and rounding is display-only after all rule decisions.

## 5. URL and identity
**URL key:** normalize safe public URL form (lowercase/IDNA host, remove fragment, standard port, normalize recognized tracking parameters cautiously without deleting SKU-identifying params). Redirects must be revalidated; no vanity canonical URL redirect to internal host. Source host allowlist must include configured legitimate CDN subhosts only after assessment, not `*.retailer.com` by default.

**Match ordering:**
1. Check source identity and listing URL key.
2. GTIN exact when present and known trustworthy; reject conflict with size/brand.
3. Normalized brand/product-line/variant, quantity dimension/value/pack and known SKU.
4. Hard mismatch → rejected; insufficient fields → possible; high-confidence exact only with no hard conflict.
5. Manual match confirmation persists reviewer identity/time/rationale; changes audit and can trigger re-evaluation without retroactively mislabelling old alerts.

**Quantity model:** numerator/denominator rational, dimension `volume|mass|count`, normalized base unit; e.g. 750ml = 750/1000 L. Pack count separate. Compare unit prices only within the same dimension and when full effective quantity known. Price per unit display uses deterministic decimal rounding; pricing/evaluation retains rational semantics. A known GTIN never overrides a hard brand, variant, pack-count, or quantity conflict. Candidate matches remain excluded until a reviewed exact decision; manual decisions record actor, time and rationale and never rewrite prior observation or alert identity.

## 6. Offer eligibility and ranking contract
The `observed_offers` row is **a declared offer**, not necessarily a verified eligible offer. Ranking service first filters:
- source approved/enabled, listing exact, observation validation valid, observed timestamp within configured freshness, supported currency and matching minor-unit scale, price >0;
- no unresolved mismatch/conflicting extraction; validity dates not expired;
- stock = in_stock preferred, stock unknown in separate qualified group, out_of_stock excluded from default purchase recommendation;
- `eligibility_state=public` for default public-only comparison;
- user setting explicitly permits member/multibuy before those are considered.

`amount_minor` is the total amount charged for `minimum_qty` saleable listing packs. Standard, public-special and member single-pack offers use `minimum_qty=1`; a multibuy records the complete qualifying basket amount and minimum quantity. Preserve the single-pack amount separately as its own offer when the retailer publishes it; never divide a multibuy into a fictitious one-pack offer. Display an effective per-pack value only alongside the qualifying basket total and quantity. Coupon offers are conditional/informational in R1 and are excluded from ranking and alerts until a later version models the complete basket and all eligibility terms. Unknown stock is not in-stock; a price-only alert may proceed only when its message explicitly says stock is unverified. Out-of-stock is excluded from default purchase recommendations and alerts.

Then compare integer effective basket totals for the selected purchase quantity. A multibuy target applies to its complete qualifying basket amount and quantity; only compare offers across retailers when their effective quantities match, otherwise keep them in separate groups. If source does not disclose shipping, taxes, fees or geographic context, label “online listed price; checkout may differ”; do not invent total landed cost.

**Discount semantics:**
- `threshold_met`: observed qualifying price <= configured cents target.
- `observed_decrease`: fresh validated new price lower than preceding valid comparable observation.
- `retailer_sale_claim`: retailer explicitly labels discount, unverified old amount if not independently observed.
- `historical_saving`: explicitly scoped past valid comparison, not standard MVP “special” badge.
- Alerts say exactly which condition fired.

## 7. Watch rule evaluation and deduplication
```text
new verified offer
 -> validate fresh / exact / scoped / eligible / in-date
 -> load immutable watch rule version
 -> evaluate target and/or percentage drop
 -> compare with previous same-context valid observation (if needed)
 -> determine episode state/re-arm policy
 -> insert unique alert episode + unique outbox delivery in one transaction
 -> asynchronous outbound delivery
```

Episode state persists across restarts. For percentage drops, comparison context must match location/eligibility/quantity, and missing baseline suppresses % rule with explicit reason. A rule change increments version; current queued deliveries retain their original rule version and evidence. The operator may suppress old deliveries if desired, but must be auditable.

For a multibuy explicitly allowed by the rule, `target_minor` applies to the complete `amount_minor` for that offer's `minimum_qty`; the UI and alert must state both the total and qualifying quantity. Percentage-drop baselines must use the same quantity and eligibility context. Do not compare a multibuy basket threshold with a one-pack amount or another retailer's offer for a different quantity.

**Exactly-once logical event, best-effort external delivery:** unique constraints prevent two episode records; provider can receive a duplicate if network response is lost. Stable `Idempotency-Key` (if provider supports it) plus event-id payload and dedup guidance prevent or explain repeats. Never promise mathematically exactly-once third-party delivery.

## 8. Jobs and scheduler semantics
`due_at` is UTC. Scheduler wakes at a bounded interval and selects due jobs under memory policy and source rate policy. A job lease is owned by random process instance ID and expires; interrupted work safely retries. Queue unique dedupe key includes job type + listing/source + schedule bucket, not only listing forever; on completion, recurring scheduler creates subsequent bucket. Attempt limits and jitter are configured; 429 Retry-After overrides default earliest retry. `dead` and `blocked` are visible and not silently discarded.

**Concurrency invariants:** manual checks coalesce with existing due job; atomic claim requires SQLite write transaction; fetch outside DB transaction; commit observation + source status, then evaluate events. Queue age/length constrained; bursts deferred or rejected with `429` UI feedback; no hidden goroutine per watched item.

## 9. HTTP route contracts and implementation status
HTML first. Application pages and mutations require a session; login, setup, health and local static assets are public. External navigation is a normal anchor. HTMX GETs return the main fragment; mutation routes validate the same origin and CSRF token as ordinary POSTs.

| Route | Method | Auth | Behaviour / implementation status |
|---|---|---|---|
| `/` | GET | yes | Dashboard full page or selected fragment — implemented |
| `/login` | GET/POST | anonymous/CSRF as applicable | Login; rotate session on success — implemented |
| `/logout` | POST | yes+CSRF | Invalidate session; state change is never performed by GET — implemented |
| `/setup` | GET/POST | one-use local setup | Initial administrator setup — implemented |
| `/search` | GET | yes | Local catalog search — implemented; retailer discovery providers are absent/disabled |
| `/products` | GET | yes | Local catalog with bounded keyset pagination — implemented |
| `/products/new` | GET/POST | yes+CSRF | Create a canonical product identity — implemented |
| `/products/from-url` | POST | yes+CSRF | Validate URL and preview before attach — planned; no approved source or adapter exists |
| `/products/{id}` | GET | yes | Product identity, fresh exact offers, observed range, bounded sampled history chart and keyset-paginated observation table — implemented |
| `/products/{id}/edit` | GET/POST | yes+CSRF | Edit identity with optimistic version, rationale audit and exact-listing re-review — implemented |
| `/products/{id}/listings` | POST | yes+CSRF | Attach confirmed listing — planned behind B00/B08 approval and adapter gates |
| `/listings/{id}/check` | POST | yes+CSRF | Enqueue bounded idempotent refresh — planned; no manual check route is exposed |
| `/watches/new` | GET/POST | yes+CSRF | Preview current validated offers without sending an alert; save versioned criteria and selected in-app/configured-webhook destination — implemented |
| `/watches/{id}/edit` | GET/POST | yes+CSRF | Edit watch criteria and destination by appending an immutable rule version; stale updates conflict — implemented |
| `/watches/{id}/active` | POST | yes+CSRF | Pause/resume a watch — implemented |
| `/deals` | GET | yes | Fresh eligible ZAR public offers — implemented |
| `/activity` | GET | yes | Bounded keyset-paginated activity view — implemented |
| `/sources` | GET | yes | Assessment, capabilities, last success/failure, retry time, parser version and guarded operator control — implemented; approvals remain external |
| `/sources/{id}/enabled` | POST | yes+CSRF | Pause or enable an already approved source with source-event and admin-audit records; cannot self-approve — implemented |
| `/settings` | GET | yes | Password change, live app-cgroup resource panel and local operation guidance — implemented; cgroup view is not whole-host Pi certification |
| `/settings/notifications/test` | POST | yes+CSRF | Explicit fixed test delivery to a configured webhook; shares its bounded target policy — implemented |
| CLI `backup` / `restore` | command | local operator | Verified SQLite backup, compatibility validation, safety snapshot and restore recovery — implemented; no backup web routes |
| `/exports/history.csv` | GET | yes | Bounded streaming export of validated exact public history; excludes source URLs/secrets and formula-escapes text — implemented |
| `/health/live` | GET | LAN/no secrets | Process alive — implemented |
| `/health/ready` | GET | LAN/no secrets | App startup readiness — implemented |

**General responses:** 400 malformed requests; 401 no session; 403 policy forbidden; 404 absent; 409 stale version conflict; 422 domain validation; 429 backpressure/rate; 503 dependency/policy disabled. Ordinary route GETs return a full HTML document; HTMX GETs return the main content fragment. Form-validation responses currently render the full document, including for HTMX POSTs. Protect the setup route against remote first-user takeover.

For cross-component diagnostics, internal service results use typed codes rather than passing raw network exception strings into HTML.

## 10. Page viewmodels
`ProductCardVM`: exact display identity, safe image policy, watched state, count of current confirmed offers and latest check status.  
`OfferVM`: retailer/name, amount/currency, qualification, eligibility, stock, observation timestamp, age, link, evidence label, stale flag, unit price if comparable, excluded reason.  
`WatchVM`: rule summary, version, channel, last episode, re-arm criteria, enable state.  
`SourceHealthVM`: state, capabilities, assessment date, last success/failure, next allowed check.  
`JobVM`: queued/leased/backoff/blocked, last error code, due time, operator action.  
**Never** expose raw authorization tokens, browser cookies, full untrusted HTML, or unchecked destination URLs.

## 11. Error taxonomy and failure contract
| Code | Category | Public action |
|---|---|---|
| `SOURCE_NOT_APPROVED` | Policy | No fetch; explain assessment required |
| `URL_UNSAFE` | Security | Reject and audit; no retry |
| `HTTP_RATE_LIMIT` | Source | Backoff through Retry-After; no alerts |
| `HTTP_FORBIDDEN` | Source | Stop/backoff and operator review |
| `FETCH_TIMEOUT` | Network | Bounded retry; last price stale |
| `BODY_TOO_LARGE` | Resource | Abort safely; no parse/price |
| `PRODUCT_MISMATCH` | Identity | Candidate review; exclude exact ranking |
| `PRICE_MISSING` | Parser | No new price; keep history |
| `PRICE_CONFLICT` | Correctness | Manual parser review; no alerts |
| `OFFER_TERMS_UNKNOWN` | Eligibility | Exclude public best/alerts |
| `STOCK_UNKNOWN` | Availability | Qualified display, not confirmed in-stock |
| `DB_BUSY` | Storage | Bounded retry and operator alert after threshold |
| `RESOURCE_DEFERRED` | Runtime | Pause acquisition until below soft watermark |
| `NOTIFY_FAILED` | Delivery | Outbox retry and visible diagnostics |

## 12. Database operations and backup
- `PRAGMA foreign_keys=ON`; WAL configured at startup, autocheckpoint tested against SD/SSD behavior.
- Short write transactions; schema migration serialized before scheduler starts.
- Safe online backup, integrity verification, checksum manifest, atomic rename, restrictive permissions, tested restore *with container stopped*. Never copy the main `.sqlite` file alone while WAL active.
- A backup is a sensitive local artifact with restrictive permissions. It contains no sessions, webhook credentials or external secrets; after a successful restore, invalidate all sessions and re-provision external credentials.
- On restore: accept only the documented versioned artifact; validate schema compatibility, checksum and integrity, create a safety backup, stop jobs, swap atomically where possible, restart, verify readiness and alert/outbox consistency. Reject an invalid or incompatible artifact without replacing the current database. If replacement, restart or readiness fails, stop work and reinstate the pre-restore database from the safety backup before reporting failure.
- On old-version upgrade: backup first and apply supported forward-only migrations. Rollback is the prior application image plus a validated pre-upgrade database backup, not ad hoc reverse SQL or an unsupported downgrade.
- Consider database size/reliability on Pi SSD; no high-frequency fsync without demonstrated benefit.

## 13. Observability and privacy
Keep structured internal events for errors and alert decisions, not raw store pages. Store evidence digests/selector paths and timestamps for parser diagnosis. Logs and exports scrub session tokens, webhook secrets, home-network addresses and retailer cookies. Retention settings cannot erase referenced alert evidence while active; purge safe partitions only with explicit user action.

## 14. Implementation-linked traceability
| Design area | PRD references | Acceptance references |
|---|---|---|
| Canonical products/listings/matching | FR-001–009 | AC-M*, U-02 |
| Observations/offers/ranking | FR-010–020 | AC-P* |
| Watches/episodes/outbox | FR-021–029 | AC-A* |
| Scheduler/sources/health | FR-030–040 | AC-S*, AC-O* |
| Security/SSRF/auth | NFR-009/011 | AC-X*, AC-M02 |
| Resource and SQLite | NFR-001–008/010/014 | AC-R*, AC-O* |
| UI/HTMX/mobile | UX-SPEC | U-01–U-14 |

Every implementation slice will cite these IDs and add a migration, test fixture and observability artifact where applicable.
