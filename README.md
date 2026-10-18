# leanPrice

A lightweight, self-hosted product discovery, price comparison, historical price tracking and sale-alert application for South African retailers, designed for Raspberry Pi ARM64.

**Status:** Product definition (v1.0) drafted. No application implementation has been released.

**Non-negotiable:** the complete Pi-hosted runtime, including searches, scraping and notifications, must remain below **500,000,000 bytes RAM** during supported workloads. The single container's enforced limit will be smaller to preserve margin. Remote search/browser services, if adopted, remain optional and are not part of the Pi deployment.

## Product contract

1. [Product Requirements Document](docs/product/PRD.md) — vision, scope, requirements, release gates, risk and pricing semantics.
2. [UX and Interaction Specification](docs/product/UX-SPEC.md) — IA, workflows, visual system, states and accessibility.
3. [Acceptance Matrix](docs/product/ACCEPTANCE.md) — positive and negative scenarios, retailer feasibility and mandatory memory tests.

## Delivery policy

Product specification is merged to `main` **before** the technical plan is authored against it. All validation is run locally; no GitHub Actions or hosted CI required. The app has not been built, tested or deployed yet.
