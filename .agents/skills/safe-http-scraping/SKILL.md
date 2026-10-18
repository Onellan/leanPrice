---
name: safe-http-scraping
description: Build bounded, permitted browserless Go scraping with Colly/goquery and hard SSRF/memory controls.
---

# safe-http-scraping

**Trigger:** all source fetch, extraction, discovery, URL normalization, redirects, HTTP retry and parser work.

Only the central fetch package may open retailer network connections. Validate HTTPS source allowlist, hostname/IP, DNS resolutions, IPv4/IPv6 public ranges, IDNA, ports and every redirect; do not allow loopback, RFC1918/ULA, link-local or metadata endpoints. Protect against rebinding by dialing validated IP and verifying host/TLS identity correctly. Distinguish outbound notification endpoint policy from retailer fetch policy.

Use context deadlines, one in-flight global fetch initially, per-source cadence/rate policy, Retry-After, cancellation and bounded retry. Stream and cap *decompressed* response bytes before DOM creation (design starting cap 2MiB); limit JSON blocks, page/candidate totals and recursion depth. Store source hash/parser version/field evidence rather than full HTML. Request handler must enqueue work, not block on arbitrary remote fetch.

Extraction precedence: permitted official feed/API > public JSON-LD Product/Offer > known publicly exposed JSON > goquery CSS selectors. Colly is optional orchestration; it must use hardened transport. Treat conflicting fields, malformed money, missing promotion terms, blocked pages and changes in markup as **invalid/unknown** rather than choosing the lowest number. No JavaScript evaluation, Selenium/Chromium on Pi, anti-bot evasion or raw arbitrary URL crawling.

Fixtures: 2MiB/oversize, decompression bomb, Unicode price, location-dependent page, 403/429/5xx, redirect-to-private, DNS rebinding, conflicting JSON-LD vs visible offer and missing price. Profile allocations under Pi limit.

**Derived for leanPrice from:** Tockr codebase-design/diagnosing-bugs and Job Hunter scraper-risk rules, aligned with leanPrice ARCHITECTURE and VALIDATION.
