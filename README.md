# leanPrice

A lightweight, self-hosted product discovery, price comparison, historical price tracking and sale-alert application for South African retailers, designed for Raspberry Pi ARM64.

**Status:** The local implementation now includes the Go/Chi SSR runtime, versioned SQLite storage and backups, administrator authentication, bounded outbound clients, exact pricing/matching, durable scheduling and alerts, catalog/watch workflows, offer/history views, a bounded CSV export and an explicit configured-webhook test action. Workstation tests, static analysis and ARM64 builds pass. The project uses the MIT License ([`LICENSE`](LICENSE)). Retailer adapters and URL-add confirmation, webhook endpoint configuration, independent reviews and Raspberry Pi certification remain open. Per-watch notification preference and versioned editing are implemented. Nothing is deployed.

**Non-negotiable:** the complete Pi-hosted runtime, including searches, scraping and notifications, must remain below **500,000,000 bytes RAM** during supported workflows. The single runtime container's enforced limit must be lower. Optional external search/browser services are not included in the Pi runtime and require explicit opt-in.

## Product definition (merged first)

1. [Product Requirements Document](docs/product/PRD.md) — vision, scope, requirements, release gates, risk and pricing semantics.
2. [UX and Interaction Specification](docs/product/UX-SPEC.md) — screens, flows, design system, states and accessibility.
3. [Product Acceptance Matrix](docs/product/ACCEPTANCE.md) — positive/negative scenarios, retailer feasibility and hard memory tests.

## Technical implementation documentation

1. [Architecture and Engineering Standard](docs/technical/ARCHITECTURE.md) — one-container Go+SQLite architecture, package boundaries, scraping and memory safeguards.
2. [Data Model and API Contracts](docs/technical/DATA-CONTRACTS.md) — entities, schema sketch, business invariants and routes.
3. [Detailed Implementation Plan](docs/plans/IMPLEMENTATION-PLAN.md) — staged **Priority → Batch → Slice** delivery, P0 MVP through P3 optional features.
4. [Plan execution](docs/plans/IMPLEMENTATION-PLAN-EXECUTION.md) — dependency order, implementation evidence and external gates.
5. [Architecture Decision Records](docs/technical/ADRS.md) — alternatives, decisions, trade-offs and verification triggers.
6. [Local Validation and Release Runbook](docs/technical/VALIDATION.md) — local quality gates, Pi load tests, source policy and rollback.

## Agent skills and delivery guidance

- [Root agent instructions](AGENTS.md) — authority hierarchy, Pi memory, scraping/price safety and local-only delivery rules.
- [Skill registry](.agents/skills/README.md) — 23 reusable and leanPrice-specific skills, routed by task.
- [Cross-repository transfer audit](docs/skills/TRANSFER-AUDIT.md) — 26 repositories scanned, source lineage and exclusions.

## Engineering policy

- The PRD/UX/acceptance baseline was merged into `main` **before** technical documents were authored.
- Current foundation: Go 1.27.2 + Chi 5.5.0, server-rendered HTML, locally bundled HTMX 2.0.11, build-time Tailwind 4.3.0 and SQLite schema version 4. It includes bounded catalog/history/activity pagination, audited source controls and versioned watch notification preferences. No live retailer source is approved or enabled.
- No mandatory MySQL, Redis, SearXNG, Selenium, Chromium or local headless browser; Colly/goquery/structured product metadata for approved HTTP retrieval.
- Target retailers are **candidates**, not integrated until technical and terms assessment passes; unsupported source errors remain explicit.
- All validation and independent batch/final reviews must be evidenced **locally**; do not add/edit `.github/workflows/*` or use GitHub Actions as a gate.
- No fictitious price/deal, false exact match, unbounded queue or unapproved URL fetch.

## Local development

Run `make bootstrap` with the pinned Go toolchain, then `make lint test test-contract test-fixtures backup-drill build-arm64`. `make image-arm64` builds the ARM64 image using the locked Tailwind toolchain. `docker compose up --build` keeps the published port on loopback by default. The admin UI is authenticated; keep it on a trusted local network, and do not enable/fetch retailer sources without dated approvals. `make release-check` reports missing external certification evidence after it runs local gates. See [Contributing](docs/CONTRIBUTING.md), [operations](docs/OPERATIONS.md) and [code standards](docs/technical/CODE-STANDARDS.md).

The current execution gaps and evidence are tracked in [the detailed execution plan](docs/plans/IMPLEMENTATION-PLAN-EXECUTION.md). All five assessed retailer candidates remain disabled; source-backed refresh, R1/R2 release and Pi memory compliance are blocked pending the approvals and target hardware recorded there.
