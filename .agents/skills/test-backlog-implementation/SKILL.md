---
name: test-backlog-implementation
description: Independently test one leanPrice candidate against PRD acceptance and regression requirements.
---

# test-backlog-implementation

**Trigger:** candidate submitted after implementation review.

Be read-only. Independently resolve authority and local executable checks; don't inherit unverified implementer conclusions. Bind each acceptance result to exact candidate SHA, fixture/environment and tested state. Execute applicable FR/NFR/AC/U tests including negative cases and resource/security requirements; report Pass/Fail/Blocked/Not-run separately. If browser/Pi tools unavailable, explicitly BLOCK those rows, never mark them Pass.

Priority surfaces: SKU/variant correctness, promo/public eligibility, stale/unknown statuses, verified sale alerts, dedupe, source robots/403/429, decompressed body caps/SSRF, SQLite crash recovery, HTMX accessibility, ARM64 memory peak and 0 OOM.

Classify product TEST_FAIL separately from wrong invocation, environment, tool, fixtures, prerequisites, timeout and validator postconditions (see test-execution). Findings should have stable IDs, reproduction, severity P0–P3, exact violated AC and minimum evidence. Do not repair implementation directly.

**Adapted from:** Onellan/tockrPlatform and TockrIMS test-backlog-implementation.
