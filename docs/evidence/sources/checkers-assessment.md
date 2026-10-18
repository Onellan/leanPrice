# Checkers source assessment

**Assessment date:** 2026-10-18  
**Geography:** South Africa  
**Decision:** **REVIEW REQUIRED; not approved**  
**Assessor:** technical assessment only; not legal advice or source-owner approval  
**Enabled capabilities:** none  
**Permitted request cadence for leanPrice:** 0 pending policy review and source-owner approval

## Scope and evidence

- Candidate service: [Checkers South Africa](https://www.checkers.co.za/).
- Candidate public offer material: official [Checkers promotions site](https://specials.checkers.co.za/), including store/region-specific catalogues. The current search-indexed material reviewed includes locality-limited offers and Xtra Savings/card conditions; this demonstrates that locality and conditional-price semantics matter, not permission to retrieve product pages.
- Terms page: the research browser could not retrieve the candidate Checkers terms URL. The applicable terms are therefore unresolved.
- Robots: [checkers.co.za/robots.txt](https://www.checkers.co.za/robots.txt) was not retrievable by the research browser. This is **unknown**, not an allow result.
- Sample product URL, source API/feed, HTML/JSON-LD fields and retail price page were not inspected or requested.

## Capability and technical status

| Capability | Result |
|---|---|
| Manual URL refresh | Not approved; current terms/robots need review |
| Product search | Not approved; no search permission or interface established |
| Public/member price, stock, promotion, locality | Catalogue evidence shows regional/card conditions can exist; page-level fields not sampled |
| JSON-LD/feed/API/HTML | Unknown; no endpoint request made |
| Rate, body size, latency, extraction reliability | Unknown; no request made |
| Parser/version/fixtures | None; no captured page or fixture retained |

## Gate and re-review

Do not fetch Checkers URLs or treat public specials PDFs as permissioned machine-readable fixtures. Obtain current terms/robots directly from the operator and written clarification for low-rate household monitoring. If approved, determine whether online listings are store-local, Xtra Savings/member-gated, or sourced from a separate service; then build fixtures only through an authorized method.
