---
name: deliver-backlog-item
description: Orchestrate one leanPrice active backlog item through planning, implementation, independent review, testing and local release gate.
---

# deliver-backlog-item

**Trigger:** complete end-to-end delivery of one authorised batch item.

Flow:
1. Identify active backlog authority and product acceptance; distinguish planned/implemented/certified.
2. Use plan-backlog-item to define risk-tagged slices and dependency order.
3. Implement dependency-ready slices using implement-backlog-plan; allow multiple compatible slices in one run, but preserve discrete evidence.
4. Independent read-only code-review returns R1 blockers and R2 findings.
5. Independent test-backlog-implementation validates acceptance on exact candidate SHA; failures classified using test-execution.
6. Route valid product failure to implementation repair, test-context failures to harness/prerequisites, then re-review and re-test changed candidate.
7. Perform local release-parity and Pi cgroup certification when triggered; **never substitute GitHub CI**.
8. Publish/merge only after actual approvals and authorised Git writes; mark complete only after evidence exists.

Critical: source permissions, 500 MB maximum and no false price alert are not waivable. A blocked test is not a pass. Reviewer/tester must remain read-only and independent from implementer. Retain a concise checkpoint (next slice, evidence IDs, findings, SHA); do not replay certified work.

**Adapted from:** Onellan/tockrPlatform deliver-backlog-item; aligned with Tockr delivery three-lane workflow. Hosted-CI phase intentionally replaced with local validation.
