# B06 — Product and operator workflows
**Status:** partial implementation; approved-source add flow and independent UX acceptance open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-001–005/009/015–017/031/035/039; U-01–U-07/U-09/U-10; AC-M/P/O/X families.

## Implemented locally
- Authenticated server-rendered dashboard, local catalog search, product create/edit, watches, deals, source status and activity (`internal/httpui/`, `web/templates/`).
- Product identity edit uses optimistic update time, required rationale and audit event; existing exact listing matches are demoted to possible for re-review.
- Product detail shows current offers only when the latest observation is fresh, successful, valid, exact and approved; retailer URL is shown only for HTTPS exact-host links with no query, fragment or userinfo. Historical prices preserve currency and scale in the observation table.
- 30-day observed price summary uses comparable validated in-stock history and explicitly disclaims savings/completeness.
- Watch creation supports ZAR target, relative decrease and any/all; forms preserve safe values and product/watch lists do not render under the create form. Product, activity and product-history pages use bounded keyset pagination with stable ordering; search cursors preserve the query and malformed cursors are rejected. Sources support audited pause and approval-gated enable, and settings displays app cgroup current/threshold/hysteresis.

## Local evidence
- `go test -count=1 ./internal/httpui ./internal/storage`: PASS. Includes cursor paging/tie-breaks and invalid cursor rejection, form persistence/stale edit conflict, no stale offer after failed latest observation, safe listing-link host checks, currency formatting, sample range filters, authenticated CSV route and source-status copy.
- Browser-rendered wide/narrow screenshot, screen reader, keyboard/focus walkthrough and independent UX/accessibility review: NOT RUN.

## Not implemented / blocker
- URL add → validated preview → user confirmation → listing attachment is absent. It depends on at least one assessed, permitted, enabled source/adapter; none exists. Do not present local catalog entry as retailer verification.
- Rendered browser/accessibility evidence and independent UX review remain open; the app-cgroup reading is not whole-Pi certification evidence.
