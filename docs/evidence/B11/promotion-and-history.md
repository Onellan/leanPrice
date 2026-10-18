# B11 — Conditional promotions and richer history
**Status:** partial domain support; source-backed semantics and complete user surface open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-018–020/027–028; AC-P05–P09/P12/P15; NFR-004; U-11–U-14.

## Implemented locally
- Exact rational compatible-unit cost and pack/basket comparison (`internal/pricing/quantity.go`, `offer.go`); incompatible dimensions/currencies are excluded.
- Domain and storage carry offer type, member/conditional eligibility, minimum basket quantity and stock. Rules require explicit eligibility selection and complete multibuy basket cost.
- Product detail summarizes a 30-day observed range and renders a server-side scatter chart of up to 100 validated public in-stock ZAR points. Each point has retailer/price/time text, different stores are not connected, and the table remains available as a no-JavaScript alternative.
- `TestObservedPriceRangeUsesOnlyComparableValidatedInStockPrices` verifies chart sample filtering and latest-page bounds; `TestPriceChartUsesBoundedIntegerCoordinatesAndTextLabels` checks presentation mapping. Related suites pass in `internal/pricing`, `internal/watches`, `internal/storage` and `internal/httpui`.

## Not implemented / blocker
No retailer-specific proof exists for member, multibuy, coupon or validity extraction. Coupons remain alert-ineligible. History uses bounded keyset pagination; additional channel adapters are absent. Finish source-specific behavior only with approved evidence and explicit product/UX scope. B11 is not accepted.
