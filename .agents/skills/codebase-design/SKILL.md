---
name: codebase-design
description: Design deep Go modules and stable interfaces around leanPrice price, source and storage invariants.
---

# codebase-design

**Trigger:** changing package architecture, public contracts, adapter seams, or creating shared capabilities.

Preserve one Go binary with Chi + server HTML/HTMX + SQLite. Prefer few deep modules over layers of pass-through abstractions. Place invariants where every writer crosses them: pricing validation in pricing, match consistency in match/catalog, approved network targets in fetch, dedupe in jobs/watches/storage. Keep HTTP/request code out of repositories; keep raw SQLite details inside storage; keep retailer-specific parsing inside assessed adapters.

Design interfaces only for true variability (e.g. fetcher, retailer-specific extractor, notification transport), not hypothetical future databases or service meshes. No network calls inside write transactions; external webhook and source fetches live at explicit boundaries. Readability, deterministic fixtures, fakeable seams and bounded buffers matter more than premature configuration.

For each new abstraction state caller, hidden complexity, invariant ownership, smaller possible interface, test seam and deletion test. Report concrete alternatives, trade-offs and relevant ADR impact. Major contract changes require PRD/ADR updates first.

**Adapted from:** Onellan/tockrPlatform codebase-design; TockrCTRL deep-module design practices.
