# Takealot source assessment

**Assessment date:** 2026-10-18  
**Geography:** South Africa  
**Decision:** **DISALLOWED for automated retrieval under the published terms reviewed**  
**Assessor:** technical assessment only; not legal advice or source-owner approval  
**Enabled capabilities:** none  
**Permitted request cadence for leanPrice:** 0 until Takealot gives written permission or the policy changes and is reviewed again

## Scope and evidence

- Policy: [Takealot Platform Terms & Conditions](https://terms-and-policies.takealot.com/terms-conditions/), reviewed 2026-10-18.
- Robots: [takealot.com/robots.txt](https://www.takealot.com/robots.txt), reviewed 2026-10-18.
- Candidate page: `https://www.takealot.com/` (no product page was fetched or probed).
- Geography/product context: South African online store; product price, availability and fulfillment are potentially address/collection dependent. This assessment did not test a locality-selected product page.

The current terms say users must not use robots, spiders, automatic devices or manual processes to monitor, copy, distribute or modify the platform/content, except technology used by internet search websites to direct users to the platform. They also restrict use of platform content without written permission. Robots allows `/` for general crawlers while disallowing account, cart, wishlist and several query patterns. Robots permission for search-engine crawling does not grant leanPrice permission to monitor or copy product information.

## Capability and technical status

| Capability | Result |
|---|---|
| Manual URL refresh | Disallowed by current evidence |
| Product search | Disallowed; search-engine exception does not cover this household tracker |
| Public/member price, stock, promotion, locality | Not probed; no fields may be claimed |
| JSON-LD/feed/API/HTML | Not assessed; no endpoint request made |
| Rate, body size, latency, extraction reliability | Not assessed; no request made |
| Parser/version/fixtures | None; no captured page or fixture retained |

## Gate and re-review

Do not implement or enable a Takealot adapter, fetch submitted URLs, or create page fixtures under the current policy. Re-review only after dated written permission from the source owner, a policy change that clearly permits this use, or a product scope limited to linking the user to the retailer without retrieval. If permission is supplied, assess representative product pages, location effects, normal/missing/ambiguous/promotion cases, blocked responses and parser drift before any source-specific code.
