---
name: pi-memory-certification
description: Design and independently verify the entire Pi-hosted leanPrice stack under 500 MB in supported workflows.
---

# pi-memory-certification

**Trigger:** Go runtime, scrape/search/parser, container, SQLite cache, scheduler, export, backup, deployment, new dependency, performance bug and release sign-off.

**Hard product limit:** <500,000,000 bytes attributable Pi RAM at all times. Default architecture is one container with a smaller configured cgroup hard cap (design starts 440,000,000 bytes) and no additional swap allowance. Do not count a companion SearXNG/Chromium/DB service as outside the budget if it runs on Pi. Externally hosted browser/API is optional and disclosed, never a hidden requirement.

Before claiming PASS: verify Docker actual byte limits and cgroup version; sample cgroup memory.current/peak/events, Docker stats, Go heap/pprof, host baseline and other-container contention. Measure cold start, idle, local search, large/slow/decompressed source, UI load, 100 products×5 listings, webhook retries, database backup, simultaneous pressure, 24h soak and stop-old-first upgrade. Require 0 OOM and no memory-cap violation; OOM at a hard limit is **FAIL**, not successful enforcement.

Optimize first: no in-process browser, bounded body/DOM/candidates, single worker, DB queue, streaming pagination/export, cache/page allocation budget, semaphore and backpressure. GOMEMLIMIT is soft, not proof of total RSS/cgroup memory. Pause new fetches under pressure while serving catalog/history. Distinguish provisional design budgets from actual Pi measurements; evidence must include SHA/image/config and raw samples.

**Derived for leanPrice from:** Tockr local release/performance skills and job-hunter Raspberry Pi resource rules, implementing NFR-001 and AC-R*.
