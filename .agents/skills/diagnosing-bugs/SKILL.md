---
name: diagnosing-bugs
description: Evidence-first triage for leanPrice parser, memory, concurrency, SQLite, UI and alert defects.
---

# diagnosing-bugs

**Trigger:** intermittent results, unexpected source blocking, failed repair, memory spike, wrong offer, missing/double alert, SQLITE_BUSY or HTMX state bug.

First establish expected PRD behavior, minimal reproduction, exact SHA/config, actual Pi/OS and observed symptom. Classify as: product bug, test invocation issue, tool/environment/fixture/prerequisite problem, changing third-party source, security denial, or missing specification. Do not edit product code to fix a test that was never validly executed.

Investigate at ownership seam: network/robots and source status; decompressed body limits; parse field origins/conflicting offers; brand/quantity match decisions; SQL constraints/transaction timing; job leases and alert episode key; cgroup memory.current/peak/events and Go pprof; HTMX response/CSRF errors.

Use sanitized fixtures and local tests; never increase concurrency or bypass site restrictions as a quick fix. Provide root cause, alternative hypotheses, confirmed evidence, smallest repair, regression/negative test and remaining unknowns. Roll forward as a new active slice when historical code is involved.

**Adapted from:** Onellan/tockrPlatform diagnosing-bugs and TockrCTRL practices.
