---
name: plan-backlog-item
description: Plan an active leanPrice Priority → Batch → Slice without implementing it.
---

# plan-backlog-item

**Trigger:** an approved feature/defect/architecture backlog item needs executable planning.

Read the merged PRD, UX, acceptance matrix, technical implementation plan and relevant code/test state. Select an active slice ID LP-P*-B*-S* (or propose a new distinct forward slice). Do not edit an already certified historical slice to absorb a new fix; product contract changes require the product document first.

The plan MUST include: purpose and authority map (FR/NFR/AC/U), observed repository state and blockers, in/out scope, dependencies, affected modules/SQL tables/routes, interface and migration contracts, retailer terms and URL safety if fetching, exact-match/offer provenance impact, security/privacy risk, RAM/allocation/concurrency impact, executable local test intent, expected failure cases, rollback, reviewer/tester boundary and Definition of Done.

Use classification R=bounded routine, E=elevated, H=high, plus risk codes DATA/AUTH/PRICE/SOURCE/SEC/CONC/OPS/API/UI/PERF/DEPLOY/DOC. High-risk price, source safety, authentication, history, database and Pi-memory work remains high review effort. Avoid prescribing commands that are absent; distinguish planned targets from implemented local Makefile/scripts. No GitHub Actions validations.

Keep plan output narrowly scoped, refer to stable requirement IDs rather than repeating large docs, and do **not** implement while planning. Report genuine unresolved source/legal/product decisions as blockers.

**Adapted from:** Onellan/tockrPlatform, TockrCTRL and ActualPlus plan-backlog-item.
