---
name: audit-codebase
description: Audit the complete leanPrice repository against product, architecture, security, scraping and local release authority.
---

# audit-codebase

**Trigger:** explicitly requested whole-repository audit; do not use for small single-file change.

Authority order: product PRD/UX/ACCEPTANCE → technical ARCHITECTURE/DATA-CONTRACTS/ADRS/IMPLEMENTATION-PLAN/VALIDATION → approved active plan → current source/tests. Record the exact audited Git SHA and actual test execution results, not inferred passes.

Audit categories: correctness of product matching, price/promotion/stock/freshness, source permissions, SSRF and login/CSRF, queue and webhook idempotency, SQLite history/backup, Chi/HTMX accessibility, dependency/license/security, test coverage, Pi cgroup peak/OOM, local release and documentation drift.

Distinguish not-yet-implemented/broken/unknown/untested rather than reporting a missing runtime as a failing executed test. Use existing local validators if present; if not, propose—not claim to execute—necessary scripts.

Output structured findings with ID, P0–P3, file/location, violated authoritative ID, concrete reproduction/evidence, impact, fix boundary, and proposed P→B→S. Perform no changes without separate authorization. A read-only audit is not independent release certification.

**Adapted from:** Onellan/tockrPlatform audit-codebase and TockrIMS audit-codebase.
