# Contributing

leanPrice is built for a single, resource-bounded household instance. Product truth, permitted source access, data safety and the local Raspberry Pi memory ceiling take precedence over coverage or speed.

## Local setup

Install Docker with Buildx, `curl`, `sha256sum`, `make`, and a Linux shell. Run `make bootstrap`; it installs Go 1.27.2 into the ignored `.tools/` directory when the pinned version is not already available, verifies the official archive checksum, and downloads Go modules. Use `make help` to see implemented local targets. Tailwind CSS is compiled only in the Docker build stage; Node is not a runtime service.

## Change workflow

1. Read `AGENTS.md`, the relevant product acceptance cases, technical contracts, and the active slice in `docs/plans/`.
2. Make a behavior-first change with deterministic negative tests. Do not make live retailer requests in tests.
3. Run `make fmt lint test build-arm64` and the targeted security, contract, fixture or image checks that apply. Record commands that actually ran; a missing target, compiler, reviewer or Raspberry Pi is not a pass.
4. Keep source adapters disabled until a dated source assessment and approval exists. Never add anti-bot circumvention, private retailer data, secrets, or raw captured pages.
5. Keep SQLite write transactions short and free of network side effects. Use forward-only migrations and a validated backup for recovery.

Do not add GitHub Actions or claim release certification from workstation checks. Independent review, acceptance testing and Raspberry Pi certification are separate gates.

## Branches and releases

Use focused implementation branches. Merge and deployment authority remains with the repository owner. Do not rewrite or remove another contributor's work. Follow the selected repository license once the owner records that decision in the repository.
