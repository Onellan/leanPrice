---
name: tdd
description: Use behavior-first development for leanPrice domain and integration changes with valid local test context.
---

# tdd

**Trigger:** new behavior or reproducible regression with a meaningful test seam.

Start with product FR/NFR/AC and a valid test context: current SHA, correct Go package, test fixtures, tools, supported local commands and matching tests. A missing tool or zero matched tests is not RED. For new behavior: write small failing deterministic test (RED), implement minimal production behavior (GREEN), then refactor while preserving GREEN. For already-covered refactors, begin GREEN and retain green checks rather than manufacturing failure.

Preferred seams: exact product matching; integer-cents/quantity; JSON-LD versus HTML parser conflict; source 403/429 and decompression limits; SSRF/DNS redirects; queue lease and alert dedupe; HTMX/CSRF; memory backpressure. Negative fixtures essential. Never use unsafe live retailer scraping as a test shortcut.

Run locally and record executed results. Full batch acceptance remains independent. TDD success does not replace ARM64 cgroup certification.

**Adapted from:** Onellan/tockrPlatform tdd and test-execution.
