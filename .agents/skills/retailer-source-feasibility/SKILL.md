---
name: retailer-source-feasibility
description: Assess whether a South African retailer may be reliably monitored before enabling its leanPrice adapter.
---

# retailer-source-feasibility

**Trigger:** any new retailer, changed source URL, parser drift, recurring 403/429, requested retailer-search coverage or modified terms/rate policy.

Open docs/product/PRD.md, docs/technical/VALIDATION.md and ADR-004/014. **Candidate retailers are not supported by default.** Before writing a live adapter, document the source's current terms/robots/access constraints, country/store/loyalty context, public API/feed/JSON-LD/HTML availability, permitted request cadence and evidence retention permissions. Distinguish facts confirmed from assumptions; a page rendering in a browser does not imply fetch permission.

Deliver dated assessment under docs/evidence/sources/<source>-assessment.md with sample URLs, legal/technical decision, rate budget, capability matrix (manual URL, search, public/member prices, stock, promotions, locality), parser versions and representative sanitized fixtures. Mark APPROVED, UNSUPPORTED, DISALLOWED or REVIEW REQUIRED with a rationale; do not turn on the adapter until approved.

Test at least normal price, missing offer, ambiguous product, promo terms, different size, blocked/429/timeout and changed markup. Never defeat CAPTCHA, use session-cookie harvesting, identity rotation or private APIs to evade access controls. On 403/429, back off and reassess.

**Derived for leanPrice from:** Job Hunter scraping risk guidance (Onellan/job-hunter/AGENTS.md), Tockr adapter/source governance, and leanPrice PRD/ADRs. This is newly authored, not copied from a standalone upstream skill.
