# B01 — Toolchain and foundation evidence

**Assessment date:** 2026-10-18  
**Slices:** LP-P0-B01-S01 through LP-P0-B01-S04  
**Status:** implemented; workstation checks pass; independent and Pi acceptance remain open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Base SHA:** `03356fdc22cd8be40006dddc5ab51012080a5796`

## Authority and scope

- Product: FR-035, FR-039, NFR-003, NFR-006, NFR-012, NFR-013.
- Acceptance: AC-O01, AC-R01, AC-X06, AC-X10.
- UX: U-06, U-07.
- ADR: ADR-001 (single process), ADR-004 (source fetching remains disabled).
- Implemented: pinned local Go setup, Chi server lifecycle/configuration, structured process logs, request IDs/deadlines, liveness/readiness, complete SSR shell, HTMX fragments, local compiled CSS and JS, ARM64 OCI image, Compose resource limits, local quality and evidence docs.
- Not implemented in B01: SQLite, authentication, source fetching, product persistence, price/watch/notification behavior. The UI is deliberately explicit about empty and unimplemented areas.

## Candidate files

`go.mod`, `go.sum`, `cmd/leanprice/main.go`, `internal/app/*`, `internal/httpui/*`, `web/*`, `Dockerfile`, `compose.yaml`, `.dockerignore`, `.gitignore`, `.env.example`, `package.json`, `package-lock.json`, `Makefile`, `scripts/*`, `docs/CONTRIBUTING.md`, `docs/technical/CODE-STANDARDS.md`, `docs/technical/THIRD-PARTY-NOTICES.md`, `.github/PULL_REQUEST_TEMPLATE.md`, and this record.

## Pinned inputs

| Tool/dependency | Pin or evidence |
|---|---|
| Go | 1.27.2; local Linux AMD64 archive SHA-256 `ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5`; ARM64 SHA-256 `94f3e30b8e374bc285e7dadc11e0865726b9bc6e85b841ccceaabc0214c6b7c8` |
| Chi | `github.com/go-chi/chi/v5` v5.5.0 |
| HTMX | v2.0.11; local asset SHA-256 `d6fdc75f204e6bdefa99b69bf1e6d4ac69b8a364f77929f45c13476b4000f717`; 0BSD notice embedded beside asset |
| Tailwind | CLI/CSS v4.3.0; exact npm lockfile; MIT notice embedded beside generated CSS |
| Docker builders | Node 24 Alpine index digest `sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f`; Go 1.27.2 Alpine index digest `sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d` |
| Static/security tools | Staticcheck v0.8.2; govulncheck v1.9.0; go-licenses v1.6.0 |

Tailwind 4.3.3 was rejected after `npm audit` reported four high findings in its dependency tree. Tailwind 4.3.0 plus the updated lockfile was built and audited with zero vulnerabilities; the image Dockerfile runs this audit as a build gate.

## Commands actually run

| Command | Result |
|---|---|
| `make bootstrap GO=/tmp/leanprice-toolchains/go1.27.2/bin/go` | PASS; Go 1.27.2 verified and modules downloaded |
| `make fmt lint test test-integration test-security build-arm64 GO=/tmp/leanprice-toolchains/go1.27.2/bin/go` | PASS; `go fmt`, `go vet`, Staticcheck, all Go tests, HTTP route tests, security-header tests and Linux ARM64 cross-build |
| `go test -race ./...` | PASS on Linux AMD64 workstation |
| `make vuln GO=/tmp/leanprice-toolchains/go1.27.2/bin/go` | PASS; no known Go vulnerabilities |
| `make asset-audit GO=/tmp/leanprice-toolchains/go1.27.2/bin/go` | PASS; `npm audit` found zero vulnerabilities |
| `make license-report GO=/tmp/leanprice-toolchains/go1.27.2/bin/go` | Command exits 0 and reports Chi MIT, but reports all leanPrice packages as `Unknown`; project license decision is open, so licensing is **BLOCKED**, not passed |
| `make image-arm64 GO=/tmp/leanprice-toolchains/go1.27.2/bin/go` | PASS; buildx produced `linux/arm64`, image index `sha256:d98e77cb5faa594faf2ec2ae511f1f7354099310f810bc1635bb7db59184ac57` |
| `docker image inspect leanprice:local` | PASS; image OS `linux`, architecture `arm64`, runtime user `65532:65532` |
| `docker compose config --quiet` | PASS |
| AMD64 smoke container with `--memory 440000000 --memory-swap 440000000 --pids-limit 64 --read-only --user 65532:65532` | PASS; liveness readiness and both local assets returned HTTP 200; Docker health became `healthy`; inspect showed Memory=440000000, MemorySwap=440000000, PidsLimit=64, ReadOnlyRoot=true, OOMKilled=false; one workstation idle sample was 3.727 MiB |

The AMD64 smoke is container configuration/startup evidence only. The observed idle memory is not Pi evidence and cannot close NFR-001/AC-R01. The ARM64 image was built but not run on target hardware.

## Negative cases and limitations

- Config tests reject port 0, empty data path and malformed timeout; default values are bounded.
- HTTP tests verify readiness returns 503 until ready, live health returns 200, and HTMX receives the content fragment while ordinary GET receives a complete document.
- Security tests verify local-only CSP, `nosniff`, referrer policy and a request ID.
- Static asset tests verify HTMX and Tailwind output are embedded and served locally.
- No browser is installed; rendered wide/narrow screenshot inspection and manual keyboard/screen-reader walkthrough remain open.
- No Pi is attached; cgroup v2 path, effective ARM64 limits, host memory delta, soak and OOM counters remain open.
- No independent B01 reviewer or acceptance tester is recorded. This evidence is implementer-run.
- Contract and source fixture targets are present but intentionally exit nonzero until B04/B07 domain tests and fixtures exist.
- No retailer request was made. All source-specific retrieval remains disabled.

## Security/resource and rollback notes

- Compose binds the host port to loopback by default. The shell has no authentication yet; do not expose it on a LAN until B03 is complete.
- Runtime is scratch-based, non-root, read-only except `/data` and a bounded `/tmp`, drops Linux capabilities, disables privilege escalation, caps PIDs at 64, and sets matching 440,000,000-byte memory/swap limits. Those values are configuration evidence, not measured Pi certification.
- Replace by retaining the previous image tag and stopping it before starting the next instance. There is no database migration in B01.
- No source, account, notification, household or production credentials are present.

## Review and open gates

- Independent reviewer: not assigned/performed.
- Independent acceptance tester: not assigned/performed.
- Project license: owner decision requested; `Unknown` license blocks distribution acceptance.
- Raspberry Pi: unavailable in this workspace; B00-S03/B01-S03 Pi run remains blocked.
- Visual QA: source-level responsive/accessibility review done; browser-rendered review remains blocked by missing browser.
