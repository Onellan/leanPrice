# B03 — Authentication and outbound security boundaries
**Status:** implemented locally; independent security acceptance remains open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-030; NFR-009/011; AC-X01–X14; ADR source and notification policies.

## Implemented scope
- One-use setup, bounded Argon2id hashing, login throttling, persisted session lifecycle, logout, password rotation/reset and audit history (`internal/auth/`, `cmd/leanprice/`).
- Session middleware validates origin and CSRF on every mutation; CSP/security headers and forwarded HTTPS trust only configured proxy CIDRs (`internal/httpui/`).
- One hardened retailer fetch client enforces exact host approval, public DNS answers, safe redirect revalidation, bounded time/body/decompression and rate slots (`internal/fetch/`, `internal/storage/sources.go`).
- Webhook destination policy is separate from retailer fetch; exact configured endpoint, private CIDR opt-in, pinned DNS result, no redirects, bounded request/response and redacted transport errors (`internal/notify/client.go`).

## Local negative coverage and result
- `go test -count=1 ./internal/auth ./internal/fetch ./internal/notify ./internal/httpui`: PASS.
- Tests cover unsafe schemes/private and mixed DNS answers, rebinding/redirect host changes, query-string redaction from fetch errors, weak credentials, one-use setup/session/CSRF, proxy trust and security headers.
- Webhook test route uses an explicit authenticated CSRF-protected POST and fixed test payload. Mock-server test asserts payload and target-policy behavior; UI error test verifies transport detail is not shown.
- No independent adversarial security review, deployed proxy test, Pi run, or production webhook test was conducted.

## Review boundary
Implementer-run tests, not security certification. No credentials or live retailer requests were used.
