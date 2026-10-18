# leanPrice — Agent Working Rules

## Read before planning or coding
Authority order:
1. Explicit current user request and repository permissions.
2. `docs/product/PRD.md`, `UX-SPEC.md`, `ACCEPTANCE.md`.
3. `docs/technical/ARCHITECTURE.md`, `DATA-CONTRACTS.md`, `ADRS.md`, `IMPLEMENTATION-PLAN.md`, `VALIDATION.md`.
4. Approved active Priority → Batch → Slice plan and reviewed decisions.
5. Actual source, tests and the triggered `.agents/skills/<name>/SKILL.md`.

Skills define **how to work**, never new product requirements. Use [skill registry](.agents/skills/README.md) to load only relevant ones; don't preload all skills in every context. A skill cannot override PRD security, truthful pricing, source permission or memory constraints.

## Mandatory architecture
- Original leanPrice Go/Chi app, `html/template` + HTMX + build-time Tailwind, SQLite via reviewed driver, one ARM64 runtime container.
- Under **500,000,000 bytes attributable Pi-hosted memory** through all supported workloads; test on actual Pi. A lower Docker limit is needed and OOM kills are failures.
- No compulsory local browser/Selenium, Chromium, SearXNG, MySQL, Redis, cloud analytics, or Node runtime server.
- Retailers are **candidates**, not supported until dated permission+technical assessment. No CAPTCHA bypass, rotating evasion proxies or abusive requests.
- Only central hardened fetch service may retrieve approved retailer URLs; protect against SSRF, DNS/redirect rebinding, unbounded decompression and retries.
- Prices use integer minor units and reliable product identity; stale/unknown/member/multibuy conditions cannot create misleading public sale alerts.
- Network/webhook side effects occur outside SQLite write transactions. Durable jobs and deduped alert episodes survive restart.
- Frontend is mobile-first, semantic, SSR-capable without JavaScript, accessible and explicit about source evidence and price age.

## Work selection and delivery
- Track `LP-P*-B*-S*` current slice IDs. Preserve historical certified work; current defects become forward repairs.
- Model: plan → implement (multiple dependency-safe slices per run) → independent read-only engineering review → independent read-only acceptance testing → repair/recheck → local batch/final certification.
- Implementation and review must reference PRD FR/NFR, UX U and acceptance AC IDs. Do not self-approve independent gates.
- Risks PRICE/SOURCE/SEC/AUTH/DATA/CONC/PERF/DEPLOY require strict evidence.
- Test with actual implemented **local** scripts/commands only; plans describe future `make` targets that may not exist yet. Missing test harness/tool is Blocked, not Pass.
- Do **not** add or modify `.github/workflows/*`; GitHub CI/CD is not the validation route. Build/test/profile locally and record candidate SHA/results.
- Do not modify remote repos, push, merge or deploy without the user's authorization. GitHub source read and skill provenance are permitted when researching a skill.
- Never commit credentials, household/private data, captured restricted retailer pages or confidential source material copied from private repositories.

## Skill usage examples
- Source feasibility / scraper changes → `retailer-source-feasibility`, `safe-http-scraping`, `codebase-design`, `tdd`.
- Canonical product/matches or sales → `product-identity-matching`, `price-and-promotion-integrity`, `domain-modeling`.
- SQLite/scheduler/notifications → `sqlite-job-and-outbox-integrity`, `tdd`, `test-execution`.
- UI/HTMX/Tailwind → `frontend-design`, `coding-standards`.
- Raspberry Pi profiling/release → `pi-memory-certification`, `test-execution`, independent review/testing.
- Feature delivery → `plan-backlog-item`, `implement-backlog-plan`, `deliver-backlog-item`, `deliver-slice`.

See `docs/skills/TRANSFER-AUDIT.md` for repository scan, upstream lineage and exclusions. No project code has yet been implemented merely because skills have been imported.
