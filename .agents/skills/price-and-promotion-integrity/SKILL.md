---
name: price-and-promotion-integrity
description: Guarantee monetary precision and honest verified deal classification across South African retailer offers.
---

# price-and-promotion-integrity

**Trigger:** parse/normalize prices, compare retailers, assess sales, stock, coupon/loyalty/multibuy or create alerts.

Use strictly positive integer ZAR cents (int64), explicit currency and validated decimal conversion. Quantity arithmetic uses rational values; never floats for money and never compare incomparable pack/unit conditions as exact offers. Keep observation source, UTC timestamp, parser/version, stock state, validity, offer type, evidence path/digest and eligibility.

Default public-only comparable price requires approved source, fresh valid observation, exact canonical listing and an eligible public offer. Distinguish true promo evidence from an ordinary price below the user threshold, an observed price decrease, and a *retailer-claimed* previous price. Avoid unsupported “you saved R…” unless independently observed comparable history proves it. Member/multibuy/coupon prices require explicit user eligibility and calculable effective basket cost. Unknown stock is not verified in-stock; stale or blocked sources must not win current ranking or trigger new alerts.

Table-driven tests: R29.99→2999; R 1 299,90 locale; missing/zero/negative/foreign currency; stale R25 vs fresh R34; member R25 vs public R35; 2-for-R50 requiring quantity; coupon terms unknown; expired special; misidentified size; mismatched structured/HTML sources. A single false deal is a blocking defect.

**Derived for leanPrice from:** ActualPlus accounting-correctness precision/invariant discipline **without importing finance/tax accounting rules**, Tockr domain-modeling, and leanPrice PRD/AC-P*.
