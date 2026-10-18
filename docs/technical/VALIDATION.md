# leanPrice — Local Validation, Security and Release Runbook
**Status:** **planned checks only**. No application exists yet, so none of these tests has been executed.  
**Contract:** [PRD](../product/PRD.md), [Acceptance](../product/ACCEPTANCE.md), [Architecture](ARCHITECTURE.md), [Data contracts](DATA-CONTRACTS.md).

## 1. Philosophy
Run all validation **locally** on development workstation and Raspberry Pi. No GitHub Actions changes and no GitHub CI/CD required. For each implementation slice, collect just enough objective evidence to show it works; reserve an independent full review for completed batches and final certification. A failing check blocks the corresponding merge/release. No self-declared “100% tests passed” without actually executing them.

### Three lanes of quality ownership
- **Lane A — implementation:** developers may implement multiple dependency-safe slices in one run; per-slice targeted unit/integration tests plus local build before commit.
- **Lane B — continuous guardrail:** lightweight architecture/security correctness review on each slice, constant checking of package boundaries, price truthfulness and RAM impact; not a heavyweight independent certification per trivial commit.
- **Lane C — independent review:** fresh independent batch review after all batch slices, then complete product/security/performance/recovery certification at release. Findings become scoped remediation slices; reviewers should not certify their own code.

All lanes obey the same acceptance matrix and evidence policy. The technical plan may be merged without execution evidence, but releases cannot.

## 2. Planned local commands (not yet in repository)
```bash
make bootstrap        # pinned Go/Tailwind/HTMX tool and dependency instructions
make fmt              # gofmt / template formatting
make lint             # go vet, staticcheck (pinned), lint checks
make test             # all unit tests
make test-integration # temp SQLite, fixtures, httptest routes
make test-security    # SSRF, CSRF, auth, escaping, dependencies
make test-contract    # price/match/alert golden fixtures
make build-arm64      # CGO_ENABLED=0 GOOS=linux GOARCH=arm64 build
make image-arm64      # local arm64 image build
make smoke-pi         # LAN-only health, login, watch workflow on actual Pi
make perf-pi          # normal/stress/local search and scraper profiles
make soak-pi          # 24h safe synthetic+approved workloads
make backup-drill     # concurrent backup, clean restore, integrity check
make release-check    # summary of all local gates; no GitHub Actions
```
Each target must become a real, portable script with nonzero exit on failure. Never label placeholder commands as completed.

## 3. Per-slice Definition of Done
- Code and tests reference specific `FR-*`, `NFR-*`, `AC-*` and/or `U-*` IDs.
- Tests include at least one realistic failure/negative path and no live network dependency for deterministic CI-equivalent suite.
- Documentation/ADR updated for changed contracts, migrations or operational flags.
- Security check if network, HTML, auth, sessions, secrets or outbound notifications touched.
- Memory footprint considered: no unbounded bytes, caches, searches, goroutines, job queues, generated HTML or DB reads. Targeted profile if relevant.
- Command, result, SHA/image ID, reviewer and remaining risks recorded in `docs/evidence/<batch>/`.
- Local gate passed before merging slice; no unrelated changes to `.github/workflows/*`.

### Batch certification
Fresh reviewer inspects integrated code, debt, incorrect assumptions, test breadth and package boundaries; runs batch suites and documents findings. Each blocking issue becomes remediation slice, retested and signed off before batch completed.

### Final certification
Fresh independent assessment against **all** product musts, security/identity/alerts, Pi memory and recovery; no claims of supported retailers without source-specific proof. Explicit PASS/FAIL/BLOCKED matrix.

## 4. Test layers and fixtures
| Layer | Examples | No live network? |
|---|---|---|
| Domain unit | money parsing, quantity rational normalization, GTIN, match conflicts, price freshness | Yes |
| Parser golden | sanitized/authorized JSON-LD, HTML selectors, hidden/member/multibuy terms, contradiction | Yes |
| HTTP integration | httptest server with redirect, 403, 429, timeout, large body, gzip bomb | Yes |
| SQLite integration | migrations, FK, unique constraints, queue leases, WAL backup, transactions | Yes |
| UI integration | SSR/HTMX route, auth/CSRF, accessibility attributes, forms and persisted state | Yes |
| Notification | mock webhook 2xx/4xx/5xx/drop+retry, idempotency header | Yes |
| Security adversarial | SSRF, XSS, CSRF, session fixation, missing privileges, host header/proxy | Yes |
| ARM64 smoke | image startup, health, catalog, source status, file permissions, migrations | Local Pi |
| Soak/resource | synthetic 100 products × 5 listings, 24h, search/UI/backup/failure overlap | Local Pi |
| Retailer feasibility | individually approved public product pages; rate/terms respected | Controlled live |

**Important:** live scraping is never required for deterministic core suites. Stores may be down/blocked; mark coverage uncertainty rather than inventing passing results.

## 5. SSRF and fetch security test plan
- Schemes: `file:`, `ftp:`, `javascript:`, `http:` outside explicit exceptions rejected.
- IP: localhost IPv4/v6, all RFC1918, loopback, link-local, ULA, multicast, IPv4-mapped IPv6, metadata targets, zero/ambiguous numeric representations.
- DNS/CNAME: public-looking hostname resolves private; first answer public then private on later resolution; CNAME internal; invalid IDN.
- Redirect: approved public product URL redirects to private IP, non-allowlisted host, excessive hops or non-HTTPS; refuse.
- Ports: nonstandard ports blocked unless explicitly assessed for source.
- Body: compressed/decompressed >2MiB, misdeclared Content-Length, transfer hang, stream stalling, malformed HTML, invalid UTF-8.
- Source/policy: disapproved domain, robots/terms disallow, 403/429, excessive retry and per-domain concurrency.
- Escaping: malicious product title, SVG/script, bad image links, HTML entity injection.
- Logging: redaction of session IDs, API tokens, webhook URLs and cookies.
- Authentication: setup one-time token, password hash memory, login rate-limit, CSRF on ordinary and HTMX requests, session TTL/cookie flags, protected metrics.

## 6. Pricing and matching contract fixtures
Must include all product AC-M* and AC-P*, with at least:
- Domestos Lavender 750 ml exact across stores, Original 750 ml different, Lavender 1 L different, twin-pack separate.
- Name casing and Unicode variants; GTIN trustworthy/absent/conflict; manipulated titles.
- Price “R 29.99”, “R1 299,90”, 0, -1, blank, duplicated JSON-LD conflicting with HTML, USD instead of ZAR.
- Public price R34, member R29, 2× multibuy R50, coupon requirement unknown.
- Out-of-stock vs unknown-stock vs in-stock; fresh vs stale data.
- No false “sale” from a low but non-promo standard price; retailer old-price claim vs observed historic decrease.
- Notification triggers on fresh exact eligible offer only; percent baseline missing → suppressed with reason.
- Repeated identical scrape → no repeated alert episodes; restart/outbox replay → at-least-once network delivery semantics documented.

Golden fixtures carry adapter version, source origin, rights/permission note and expected normalized payload; no real credentials or unrestricted captured HTML archived.

## 7. SQLite operational tests
- Apply each migration from empty database; test upgrade from every supported released schema version with sample data.
- Validate `PRAGMA foreign_key_check`, `PRAGMA integrity_check`, indexes and query plans on 100/500/1000 products synthetic.
- Transaction cancellation during job claim/observation write/episode+outbox insert: no partial event.
- Power interruption simulation (`kill -9` Go process) while lease and while WAL append; restart, lease reclaim, dedupe and integrity.
- Backup during write activity with online backup method; independent restore from checksum and clean container; ensure rows and notification episode continuity.
- Full disk/readonly volume: clear errors, no destructive auto-fix; operator can recover after free space.
- Retention/compaction does not remove price observations referenced by ongoing alerts.
- Verify DB performance under constrained caches, `MaxOpenConns(1)`, and benchmark any proposal to change connection count.

## 8. Memory test contract
**System hard limit:** 500,000,000 bytes of total attributable incremental Pi memory use. **Starting container guard:** 440,000,000 bytes cgroup hard limit, `memswap_limit` same, no additional always-running process. Final value must be verified from Docker inspect and cgroup files. If ancillary Pi-hosted process is introduced, include it in measured sum.

### Measurements
1. Capture idle Pi host baseline: `free -b`, `docker stats --no-stream`, current other-container stats, CPU and disk activity.
2. Capture leanPrice cgroup path and `memory.current`, `memory.peak`, `memory.events`/OOM counters (or equivalent v1 files), not just Go heap.
3. Capture Docker `MemUsage`, image/process RSS/PSS where available, host used/free delta with consistent cache accounting, Go `runtime/metrics` and bounded `pprof` (local admin only).
4. Record cgroup peak **from before workload**, not after peak was reset or after restart.
5. Execute cold start, normal UI, HTMX partials, local search, manual URL fetch, recurring scrape, error+oversize+gzip attack, notification retry, SQLite backup, combined workload and 24h soak.
6. Report peak memory in bytes, extra host footprint outside leanPrice cgroup, swap usage, OOM kills, queue progress and latency. Never hide out-of-budget operations by disabling supported features.

### Example read-only inspection (actual path is host-specific)
```bash
docker inspect leanprice --format 'Memory={{.HostConfig.Memory}} Swap={{.HostConfig.MemorySwap}} OOMKilled={{.State.OOMKilled}}'
docker stats --no-stream leanprice
free -b
# On a cgroup v2 host, inspect the actual container cgroup:
# cat /sys/fs/cgroup/<resolved-leanprice-cgroup>/memory.current
# cat /sys/fs/cgroup/<resolved-leanprice-cgroup>/memory.peak
# cat /sys/fs/cgroup/<resolved-leanprice-cgroup>/memory.events
```
Do **not** guess the cgroup path. `memory.current` may include charged file cache; system-wide memory accounting and separate cgroup readings have overlapping/shared components. Do not sum all RSS and cgroup counts blindly. Report measurement method explicitly and preserve raw samples.

### Memory-pass requirements
- Each supported workload stays under the contractual 500 MB total, with measured container peak under its verified lower hard limit.
- `memory.events` records **0 OOM and 0 OOM kills** attributable to leanPrice.
- Work backlog drains or is transparently deferred for rate/resource reasons.
- No memory leak across repeated checks; compare steady-state after GC and 24h.
- No temporary dual containers during upgrade/restart.
- If any condition fails, **block release**, lower concurrency/response cap or replace expensive parser, then rerun. Do not raise cap.

## 9. Performance gates (provisional until Pi baseline)
- Local warmed dashboard/product p95 under 800ms in controlled representative workload; source network waits never block request handler.
- Search returns bounded candidates, pages at 50 max, local catalog paginated by keyset/cursor.
- Repeatable 100 canonical products × up to 5 listings synthetic daily cycle plus 24h soak; source-policy quotas take precedence over artificial completion.
- Durable queue remains bounded; operation is resumable after restart.
- Report CPU, disk throughput, memory, SQLite WAL/page-cache size, HTTP fetch bytes and p95/p99 separately. No claim of completion for actual retailers unless feasible.

## 10. Accessibility and UX gates
Keyboard-only navigation for all screens; focus moves reasonably after HTMX swaps; errors bind input labels, screen readers receive status; mobile viewport 320/375 px; local font; SSR no-JS functional. Automated a11y audit is useful but manual keyboard and accessibility text review are mandatory. Verify evidence-rich price display includes source, timestamp, stale/unknown and member terms.

## 11. Source feasibility process
For each candidate retailer, create `docs/evidence/sources/<source>-assessment.md` documenting:
1. Date, geography/store context, URLs, source/public terms and robots evaluation, allowed rate.
2. Permitted API/feed/JSON-LD/public-HTML extraction approach; no account or CAPTCHA evasion.
3. Sanitized fixture set with expected fields and parser version.
4. Sampling of representative pages and success/error rates, latency/body sizes and per-field extraction quality.
5. Source-specific limitations: locality, logged-in prices, membership, stock and promotion conditions.
6. Decision **approved / unsupported / disallowed / requires further review** with signer.
7. Rescan triggers when markup or source policies change.

Don't enable a source while the assessment is unresolved. If no two retailers pass, follow PRD's honest manual-URL fallback and update product scope before claiming release.

## 12. Evidence template
```markdown
# leanPrice local validation — <batch/release>
Date (UTC):
Git commit SHA:
Pi model / OS / Docker:
Image digest + configuration hash:
Tester / reviewer (different for final certification):
Applicable FR/NFR/AC IDs:
Commands actually executed:
Results (PASS/FAIL/BLOCKED):
Failed fixture(s):
Cgroup memory peak, events, host delta:
RSS/CPU/disk/latency/queue summary:
Source coverage tested and terms review:
Security/accessibility findings:
Backup/restore/migration result:
Open defects, severity and owner:
Go/no-go decision and rationale:
```

## 13. Operational acceptance / rollback checklist
Before deploy: verified image + backup + free disk + stopped old instance. During: migrations, readiness and UI smoke under Pi budget. After: memory/queue/alerts check, compare DB counts, verify webhook setup. Failure: stop container, capture redacted diagnostics, restore from validated backup where needed, restart prior compatible image and record remediation.

**Important:** This document prescribes validation; the only completed action to date is documentation authoring, not validation or software deployment.
