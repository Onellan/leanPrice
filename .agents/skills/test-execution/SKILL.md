---
name: test-execution
description: Preflight and classify leanPrice local test evidence; never confuse context failures with application failures.
---

# test-execution

**Trigger:** planning, running, evaluating or accepting any tests or performance measurements.

The requirement/acceptance matrix decides **what** to prove. The repository's *actually implemented* local Makefile/scripts decide **how**. The technical plan proposes targets such as make test, make test-integration, make test-security and make perf-pi; these are not runnable authority until implemented in B01 and inspected.

Before a test: record exact Git candidate SHA/image digest, AC IDs, target Go package/profile, prerequisites (Pi/ARM64, source approvals, fixtures, ports), toolchain version and local command. Preflight command availability and test selector; a command running zero matching Go tests is INVOCATION_FAIL, not PASS.

Failure types: TEST_FAIL (valid behavior failure), INVOCATION_FAIL, ENV_FAIL, TOOL_FAIL, FIXTURE_FAIL, PREREQUISITE_FAIL, TIMEOUT (investigate), POSTCONDITION_FAIL. Only validated TEST_FAIL supports modifying product code. A PASS requires actual execution against the candidate; synthetic-only results do not prove live retailer feasibility.

Report targeted command, context, reason, measured output summary and status. Use local Pi cgroup/host measurements for NFR-001; GitHub Actions/hosted CI results do not substitute local acceptance.

**Adapted from:** Onellan/tockrPlatform test-execution and TockrCTRL validation discipline.
