# B08 — Approved retailer adapters
**Status:** BLOCKED; no source adapter is implemented or enabled  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-001/010–017/036–037; NFR-005/009; AC-S01–S12; source feasibility plan.

## Evidence and boundary
- Dated assessments are present for Takealot, Clicks, Makro, Checkers and Dis-Chem under `docs/evidence/sources/`.
- Assessment states: Takealot disallowed; Clicks, Makro and Checkers require further review; Dis-Chem requires written consent. **Zero approved sources.**
- The source registry is default-off and rejects any fetch/job without exact approved+enabled host state. No live retailer request was made.

## Required to resume
The source owner/operator must provide written permission for the exact host, path/data use, request rate, locale and retention. Refresh the dated feasibility assessment and obtain the required separate approval. Then implement one adapter and sanitized permitted fixtures at a time. If fewer than two sources qualify, obtain the plan’s explicit R1 scope decision. No guessed source or scraper workaround is acceptable.

## Acceptance
B08 S01–S04, parser drift, source health disclosure, fixture aging, signed source approval and independent compliance review are BLOCKED, not passed.
