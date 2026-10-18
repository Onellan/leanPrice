# Makro source assessment

**Assessment date:** 2026-10-18  
**Geography:** South Africa  
**Decision:** **REVIEW REQUIRED; not approved**  
**Assessor:** technical assessment only; not legal advice or source-owner approval  
**Enabled capabilities:** none  
**Permitted request cadence for leanPrice:** 0 pending written clarification/approval

## Scope and evidence

- Policy: [Makro Terms of Use](https://www.makro.co.za/pages/terms-of-use), version date May 2026; reviewed 2026-10-18.
- Robots: [makro.co.za/robots.txt](https://www.makro.co.za/robots.txt), marked last updated 2026-09-02; reviewed 2026-10-18.
- Candidate page: `https://www.makro.co.za/` (no product page was fetched or probed).
- The site permits third-party marketplace sellers. Seller-specific product, stock and fulfillment fields were not tested.

The terms grant a personal, non-commercial, revocable licence to access website content in the manner permitted by the terms. Copying, reproduction, distribution, display or publication of content is permitted only where expressly allowed, interpreted restrictively, or with written permission. The terms reviewed do not expressly address automated household price monitoring. The robots file allows `/` to standard crawlers but disallows account/checkout paths, product search and search APIs, offer details/list, seller details, quick view and other routes. An `Allow: /` rule does not itself grant content-use permission.

## Capability and technical status

| Capability | Result |
|---|---|
| Manual URL refresh | Not approved; written clarification required |
| Product search | Disallowed by the current robots rules for listed search paths |
| Public/member price, stock, promotion, locality | Not technically sampled |
| JSON-LD/feed/API/HTML | Not assessed; no endpoint request made |
| Rate, body size, latency, extraction reliability | Unknown; no request made |
| Parser/version/fixtures | None; no captured page or fixture retained |

## Gate and re-review

Keep the adapter disabled. Request written clarification for low-rate, personal household retrieval and derived price evidence. Re-review robots on each exact product URL and exclude disallowed search/offer/seller endpoints. If permission is obtained, assess public vs member price, marketplace seller, store/locality, stock and promotion fields with permissioned fixtures before implementing an adapter.
