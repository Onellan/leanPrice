# Dis-Chem source assessment

**Assessment date:** 2026-10-18  
**Geography:** South Africa  
**Decision:** **DISALLOWED for automated retrieval absent prior written consent**  
**Assessor:** technical assessment only; not legal advice or source-owner approval  
**Enabled capabilities:** none  
**Permitted request cadence for leanPrice:** 0 unless written consent is obtained and reviewed

## Scope and evidence

- Policy: [Dis-Chem Terms and Conditions](https://www.dischem.co.za/site-terms-and-conditions), reviewed 2026-10-18.
- Robots: [dischem.co.za/robots.txt](https://www.dischem.co.za/robots.txt); the research browser could not retrieve it. This is **unknown**, not an allow result.
- Candidate page: `https://www.dischem.co.za/` (no product page was fetched or probed).
- The terms state that site use is for personal, non-commercial shopping and information only, that no other use is permitted without prior written consent, and that unauthorized use/copying/reproduction/distribution of site content is prohibited. They also reserve online prices for South Africa and condition products on availability. Better Rewards and local delivery/store context may affect offers; no such fields were sampled.

## Capability and technical status

| Capability | Result |
|---|---|
| Manual URL refresh | Disallowed absent prior written consent |
| Product search | Disallowed absent prior written consent |
| Public/member price, stock, promotion, locality | Not technically sampled |
| JSON-LD/feed/API/HTML | Not assessed; no endpoint request made |
| Rate, body size, latency, extraction reliability | Unknown; no request made |
| Parser/version/fixtures | None; no captured page or fixture retained |

## Gate and re-review

Do not implement or enable a Dis-Chem adapter or fetch submitted URLs. Re-review only after dated written consent specifically covers automated household price monitoring and retention of derived product/price observations, or a policy revision grants that use. Then verify robots, public vs Better Rewards pricing, stock/locality, exact pack variants and promotion eligibility with permitted fixtures.
