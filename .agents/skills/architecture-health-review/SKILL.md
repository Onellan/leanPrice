---
name: architecture-health-review
description: Read-only architecture health review for leanPrice package locality, domain boundaries, security and Pi resource design.
---

# architecture-health-review

**Trigger:** independent architecture review, unexplained coupling, large refactor, review of new service boundaries.

Read docs/product/PRD.md, docs/technical/ARCHITECTURE.md, docs/technical/DATA-CONTRACTS.md, docs/technical/ADRS.md and the specific code/files changed. Review package locality, deep modules, cycles, implicit abstractions, shared writer invariants, data ownership, Go request/worker concurrency and bounded memory.

Must probe:
- HTTP handlers cannot implement persistence/price rules or fetch arbitrary URLs; retailer adapters use the hardened fetcher.
- Canonical product, listing, observation, offer, watch, alert episode and outbox are distinct and traceable.
- No network calls within SQLite write transactions; no unbounded job channels or Go goroutines.
- Source permission, safe URLs and source failures are enforced by common seams, not duplicated at each adapter.
- RAM guard, response limits, local-only deployment and no additional runtime containers remain consistent with the PRD.

Output a read-only findings list with file references, severity P0–P3, evidence, affected acceptance IDs and Priority → Batch → Slice remediation. Do not mutate code during review. Avoid broad, speculative redesign when the concrete seam is sufficient.

**Adapted from:** Onellan/tockrPlatform architecture-health-review, checked against TockrIMS/CTRL counterparts.
