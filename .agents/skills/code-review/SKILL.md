---
name: code-review
description: Independent read-only engineering review of a precise leanPrice candidate before acceptance.
---

# code-review

**Trigger:** completed implementation slice, batch, defect repair or requested review.

Check the exact candidate diff/SHA and authoritative FR/NFR/AC/U IDs. Review only proven code and necessary neighboring seams, not speculative code quality. Be independent from implementation; do not fix your own findings.

Critical checks:
1. Product truthfulness: exact SKU/variant/pack matching, integer cents, qualified offer semantics, fresh observation, stock and no false sale alerts.
2. Safety: retailer approval, DNS/redirect SSRF, compressed body bounds, no CAPTCHA bypass, source rate limits, safe webhook target policy.
3. Durability: SQLite migration/backup, short transactions, queue leases, unique alert episodes and outbox retries.
4. Pi performance: bounded buffers, queues, goroutines, payloads, memory and absence of hidden browser/DB processes.
5. Frontend: SSR/HTMX fallback, escaping/CSRF, keyboard/mobile/stale/partial results.
6. Evidence: tests run locally on current candidate, clear failure classification, exact AC mapping, no claimed passing tests based on missing tools.

Label **R1** blocking correctness/security/contract defect, **R2** nonblocking improvement, **Note** optional. A review PASS only means no R1 remains; it is not acceptance or release.

**Adapted from:** Onellan/tockrPlatform and TockrCTRL code-review.
