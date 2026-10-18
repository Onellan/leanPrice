GO ?= go
DIST_DIR ?= dist

.PHONY: help bootstrap fmt lint test test-integration test-security test-contract test-fixtures backup-drill pi-baseline release-check vuln asset-audit license-report build build-arm64 image-arm64 backup restore

help:
	@printf '%s\n' 'bootstrap        verify pinned Go and download modules' 'fmt              format Go source' 'lint             run go vet and pinned staticcheck' 'test             run all Go tests' 'test-integration run HTTP integration tests' 'test-security    run security-header and rendering tests' 'test-contract    run implemented domain contract tests' 'test-fixtures    require and run fixture-backed tests' 'backup-drill     run local SQLite backup and restore recovery tests' 'pi-baseline      read target Pi metrics; set PI_SSH_TARGET=user@host' 'release-check    run local gates and report external certification blockers' 'vuln             run pinned govulncheck' 'asset-audit      audit the pinned frontend build dependencies' 'license-report   report Go module dependency licenses' 'build-arm64      cross-compile the ARM64 binary' 'image-arm64      build the ARM64 container locally'
	@printf '%s\n' 'backup           create a live SQLite snapshot; set BACKUP=/data/backups/name.sqlite' 'restore          stop, validate, restore and readiness-check; set BACKUP=/data/backups/name.sqlite'

bootstrap:
	./scripts/bootstrap.sh "$(GO)"

fmt:
	$(GO) fmt ./...

lint:
	$(GO) vet ./...
	$(GO) run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...

test:
	$(GO) test ./...

test-integration:
	$(GO) test -count=1 -run '^(TestHomeRendersFullPageAndHTMXFragment|TestHealthEndpointsReflectReadiness)$$' ./internal/httpui

test-security:
	$(GO) test -count=1 -run '^TestSecurityHeaders$$' ./internal/httpui

test-contract:
	GO="$(GO)" ./scripts/test-contract.sh

test-fixtures:
	GO="$(GO)" ./scripts/test-fixtures.sh

backup-drill:
	GO="$(GO)" ./scripts/backup-drill.sh

pi-baseline:
	./scripts/pi-baseline.sh

release-check:
	GO="$(GO)" ./scripts/release-check.sh

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

asset-audit:
	docker run --rm --mount type=bind,source="$(CURDIR)",target=/src -w /src node:24-alpine@sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f npm audit --audit-level=high

license-report:
	$(GO) run github.com/google/go-licenses@v1.6.0 report ./...

build:
	mkdir -p "$(DIST_DIR)"
	CGO_ENABLED=0 $(GO) build -trimpath -o "$(DIST_DIR)/leanprice" ./cmd/leanprice

build-arm64:
	mkdir -p "$(DIST_DIR)"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags='-s -w' -o "$(DIST_DIR)/leanprice-linux-arm64" ./cmd/leanprice

image-arm64:
	docker buildx build --platform linux/arm64 --load -t leanprice:local .

backup:
	@test -n "$(BACKUP)" || (echo 'set BACKUP=/data/backups/name.sqlite' >&2; exit 2)
	docker compose exec leanprice /app/leanprice backup "$(BACKUP)"

restore:
	@test -n "$(BACKUP)" || (echo 'set BACKUP=/data/backups/name.sqlite' >&2; exit 2)
	./scripts/restore.sh "$(BACKUP)"
