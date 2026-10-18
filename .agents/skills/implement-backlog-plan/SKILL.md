---
name: implement-backlog-plan
description: Implement approved leanPrice work packages with bounded scope, behavior-first proof and local gates.
---

# implement-backlog-plan

**Trigger:** executing an authorised active plan or exact delegated slice.

Confirm the plan refers to latest main product authority and current code. Read only triggered skills: frontend-design for UI, domain-modeling/price-and-promotion-integrity for pricing, safe-http-scraping for HTTP, sqlite-job-and-outbox-integrity for persistence, pi-memory-certification for resource-sensitive changes.

At each slice: preserve unrelated changes; implement smallest cohesive domain seam; add deterministic fixture/negative test; inspect security and allocation; run existing local tests with a preflight that validates target and test selection; classify failures properly. Do not change product code because an invocation, fixture, tool or environment is broken. Keep release-parity tests for high-risk slices.

Never update certified historical acceptance retroactively. A defect in previous delivered code is a forward repair with its own slice and regression test. Keep source/network calls outside SQLite transactions and external side effects separate from observation/event commits.

Only report actual commands/results and exact SHA. Implementer does not self-certify independent engineering review or product acceptance. Multiple compatible slices may be implemented in a single run, but retain separate scope/test evidence.

**Adapted from:** Onellan/tockrPlatform, TockrIMS, TockrCTRL and ActualPlus implement-backlog-plan.
