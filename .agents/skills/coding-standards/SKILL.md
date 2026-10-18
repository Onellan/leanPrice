---
name: coding-standards
description: Apply leanPrice Go, SQLite, scraping, HTMX, security and resource standards on every code slice.
---

# coding-standards

**Trigger:** all implementation and substantive code review.

Authoritative standards: docs/technical/ARCHITECTURE.md, DATA-CONTRACTS.md, VALIDATION.md, product PRD and acceptance. Keep clean Go packages, context propagation, typed errors, good naming, stable APIs, short transactions and deterministic tests. No float money; parse ZAR as integer cents; use rational units and explicit promo terms.

Prohibit unbounded I/O and goroutines. Scraped responses are untrusted: source allowlist, DNS/redirect validation, decompressed-byte cap, timeouts, per-host backoff, no CAPTCHA or access-control bypass. No local Chromium, MySQL, SearXNG, Redis or Node runtime. SSR HTML must escape untrusted content and enforce auth/CSRF; build Tailwind assets before deployment, not via CDN in production.

Only locally run and report existing commands. Never assume scripts mentioned in a plan already exist; implement them in B01. No GitHub Actions additions or hosted-CI gates. Keep migrations/rollback and ARM64 memory evidence tied to candidate SHA. For defects, fix present-day active slice; never silently rewrite historical sign-offs.

**Adapted from:** Onellan/tockrPlatform coding-standards plus ActualPlus local-transaction/recovery rules (generalized, no financial domain rules).
