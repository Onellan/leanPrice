# Code standards

These standards turn the product and architecture contracts into reviewable code rules. `AGENTS.md` and the product documents remain authoritative if a conflict appears.

## Go and package boundaries

- Keep packages domain-focused; route handlers call use cases and repositories rather than issuing SQL or fetching retailer pages directly.
- Pass `context.Context` from the request/job boundary through I/O. Bound every request, response body, result page, retry, worker and queue.
- Return errors with safe context and preserve error categories. Never log passwords, cookies, webhook secrets, raw source pages or query strings that can contain user data.
- Use `html/template` escaping. Do not construct HTML from untrusted strings or add inline scripts/styles that weaken the content policy.
- Keep external network effects outside database write transactions. Do not create a background goroutine per product.

## Data and pricing

- Store ZAR as positive integer minor units. Use exact rational quantity/pack data; do not use floating point for price eligibility.
- Preserve canonical products, retailer listings, observations, offers, watches, alert episodes and delivery attempts as separate records.
- Treat stale, unknown stock, uncertain identity, member terms, coupons and multibuy baskets according to their explicit contracts. No inferred discount may be labelled a verified promotion.
- Add forward-only migrations with foreign keys, unique constraints, bounded queries, rollback/recovery notes and migration tests.

## HTTP, acquisition and UI

- Only the centralized outbound client may retrieve URLs, and only for assessed, approved, enabled sources. Revalidate scheme, DNS answers and redirects; cap decompressed bytes, time and concurrency.
- SSR remains complete without JavaScript. HTMX may enhance navigation and forms but cannot be the only path to important content.
- All mutations require authentication, authorization and CSRF checks once those controls are implemented. Use visible labels, associated validation, keyboard focus and a narrow-screen layout.
- Do not call a source compatible until the date, policy, fields, negative fixtures and parser maintenance decision are recorded.

## Resource and operational safety

- One ARM64 runtime service by default. The starting hard cgroup limit is 440,000,000 bytes; the whole Pi-hosted system must remain under 500,000,000 attributable bytes after real target-device measurement.
- Bound HTTP connections, SQLite result sizes, outbox attempts, diagnostics and exports. Defer noncritical work under memory pressure.
- Never disable safety limits to make a test pass. Capture actual local commands and results in the corresponding `docs/evidence/<batch>/` record.
