# leanPrice

A lightweight, self-hosted product discovery, price comparison, historical price tracking and sale-alert application for South African retailers, designed for Raspberry Pi ARM64.

**Status:** Product baseline and technical implementation plan documented. **No application code is implemented, verified or deployed yet.**

**Non-negotiable:** the complete Pi-hosted runtime, including searches, scraping and notifications, must remain below **500,000,000 bytes RAM** during supported workflows. The single runtime container's enforced limit must be lower. Optional external search/browser services are not included in the Pi runtime and require explicit opt-in.

## Product definition (merged first)

1. [Product Requirements Document](docs/product/PRD.md) — vision, scope, requirements, release gates, risk and pricing semantics.
2. [UX and Interaction Specification](docs/product/UX-SPEC.md) — screens, flows, design system, states and accessibility.
3. [Product Acceptance Matrix](docs/product/ACCEPTANCE.md) — positive/negative scenarios, retailer feasibility and hard memory tests.

## Technical implementation documentation

1. [Architecture and Engineering Standard](docs/technical/ARCHITECTURE.md) — one-container Go+SQLite architecture, package boundaries, scraping and memory safeguards.
2. [Data Model and API Contracts](docs/technical/DATA-CONTRACTS.md) — entities, schema sketch, business invariants and routes.
3. [Detailed Implementation Plan](docs/technical/IMPLEMENTATION-PLAN.md) — staged **Priority → Batch → Slice** delivery, P0 MVP through P3 optional features.
4. [Architecture Decision Records](docs/technical/ADRS.md) — alternatives, decisions, trade-offs and verification triggers.
5. [Local Validation and Release Runbook](docs/technical/VALIDATION.md) — local quality gates, Pi load tests, source policy and rollback.

## Agent skills and delivery guidance

- [Root agent instructions](AGENTS.md) — authority hierarchy, Pi memory, scraping/price safety and local-only delivery rules.
- [Skill registry](.agents/skills/README.md) — 23 reusable and leanPrice-specific skills, routed by task.
- [Cross-repository transfer audit](docs/skills/TRANSFER-AUDIT.md) — 26 repositories scanned, source lineage and exclusions.

## Engineering policy

- The PRD/UX/acceptance baseline was merged into `main` **before** technical documents were authored.
- Go + Chi + server-rendered HTML + HTMX + build-time Tailwind + SQLite via modernc.org/sqlite; original implementation (not a PriceBuddy fork).
- No mandatory MySQL, Redis, SearXNG, Selenium, Chromium or local headless browser; Colly/goquery/structured product metadata for approved HTTP retrieval.
- Target retailers are **candidates**, not integrated until technical and terms assessment passes; unsupported source errors remain explicit.
- All validation and independent batch/final reviews must be evidenced **locally**; do not add/edit `.github/workflows/*` or use GitHub Actions as a gate.
- No fictitious price/deal, false exact match, unbounded queue or unapproved URL fetch.

The next work is feasibility spikes and implementation batch `LP-P0-B00`, then foundation batch `LP-P0-B01`; plans alone are not evidence of working software.
