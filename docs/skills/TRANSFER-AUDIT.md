# leanPrice — Cross-repository skill transfer audit
**Date:** 2026-10-18  
**Target:** `Onellan/leanPrice`, branch `chore/import-reusable-agent-skills`  
**Scope:** connected GitHub account repository defaults accessible during this review (26 repositories). This records a path/tree inventory and targeted source-content review, **not a deep audit of all source files or historical branches**.

## 1. Inventory
The following repository default-branch trees were queried using authenticated GitHub access:

| Repository | Branch | Skill directory finding |
|---|---|---|
| git-study | master | No `SKILL.md` agent library |
| starter-web | master | None |
| materials | master | None |
| ha.integration.load_shedding | master | None |
| speedtest_exporter | master | None |
| BunkaraHomeHub | main | None |
| person_address_sensor | main | None |
| home_sensor_notifications | main | None |
| OpenBid | main | None |
| **tockrctrl** | main | **14 shared agent skills** |
| dpgpay | main | None |
| geyserwala-ha | main | None |
| ha_tuya_ble | main | None |
| localtuya | master | None |
| gopro-plus | master | None |
| gmail-observability | master | None |
| STP | main | None |
| job-hunter | main | No SKILL.md library; `AGENTS.md` contains scraper/deployment risk guidance |
| JobSpy | main | None |
| kolaboSolutions | main | None |
| invoice-analytics | main | None |
| AgentDelivery | main | No SKILL.md library; `AGENTS.md` contains durable agent execution guidance |
| **tockrims** | main | **16 shared agent skills** |
| **tockrPlatform** | main | **17 shared agent skills** |
| **ActualPlus** | main | **large agent library**: shared workflow/engineering + finance/SA tax/accounting specialties (59 SKILL.md files in tree; counted by discovered paths) |
| leanPrice | main | No agent skills before transfer |

Inventory numbers are source-tree `SKILL.md` occurrences under `.agents/skills/*/`; source trees were not truncated in their API responses. No private-repo AGENTS.md or domain-only skill is copied verbatim.

## 2. Transfer decision matrix
**Selected reusable capabilities (17):**
- Workflow: `plan-backlog-item`, `implement-backlog-plan`, `deliver-backlog-item`, `deliver-slice`, `code-review`, `test-backlog-implementation`, `test-execution`, `audit-codebase`, `resolving-merge-conflicts`.
- Engineering: `architecture-health-review`, `codebase-design`, `coding-standards`, `diagnosing-bugs`, `domain-modeling`, `frontend-design`, `tdd`, `evaluate-agent-efficiency`.

**Transfer modes:**
- **Verbatim from public Onellan/tockrPlatform:** `frontend-design/SKILL.md` and 5 linked `reference/*.md` files (general-purpose by design). Its source blob SHA for SKILL.md was `a5b9486aa57252f8623f7bb039441f9b84545d28`. Supporting refs imported as part of same scoped skill, not loose policies.
- **Adapted to leanPrice:** remaining 16 shared skills, based on patterns/skill contents from `Onellan/tockrPlatform`, `Onellan/tockrctrl`, `Onellan/tockrims`, with applicable correctness/recovery norms from `Onellan/ActualPlus`. They explicitly replace Tockr ownership/terminal-ledger/CI commands with leanPrice authority and local evidence, not blind string substitution.
- **New leanPrice-specific skills (6):** `retailer-source-feasibility`, `safe-http-scraping`, `product-identity-matching`, `price-and-promotion-integrity`, `sqlite-job-and-outbox-integrity`, `pi-memory-certification`. These specialize patterns from prior repos and the already-merged leanPrice product/technical plan; no unsupported source claims.

**Explicit exclusions:**
- ActualPlus finance, SARS/VAT, accounting/GL, budget reconciliation and tax evidence skill directories are **not relevant** to product-price watching. Only generic precision/immutability/transaction safety principles inform new skills.
- Tockr Platform tenant/workspace entitlements, document assertion mechanics and rollout-specific instructions are not leanPrice product requirements.
- Tockr/ActualPlus Codex reasoning configuration, CI scripts, validation registry references, agent runtime files and delivery checkpoints are **not copied** because leanPrice does not yet implement those paths and uses local-only validation.
- No private source secrets, financial records, system credentials, enterprise assumptions or production data were moved.

## 3. Reconciliation and risk
- Source skill headers commonly refer to `docs/implementation/IMPLEMENTED.md`, `scripts/validate.py`, `.codex/...` or platform-specific tenancy. Those references were deliberately **not** reproduced as working commands or authority. leanPrice will define its own local gate scripts in batch `LP-P0-B01`.
- `frontend-design` is framework-agnostic; use alongside leanPrice UX and SSR/HTMX architecture, which outrank visual generic guidance. All five linked refs have been imported.
- For HTTP collection, import general engineering approaches but **not JobSpy scraper implementations or source restrictions bypasses**; each retailer must be assessed.
- Existing product/technical plans are the source of truth; skill changes cannot create a new feature, declare successful acceptance or relax Pi RAM limits.
- Because source is a mix of public and private projects, provenance is documentation only: do not auto-sync private repositories or expose their code by replacing adapted files wholesale.

## 4. Result and counts
- **26 repositories inspected at default-branch file-tree level.**
- **17 reusable skills** transferred (1 portable verbatim + references, 16 adapted).
- **6 leanPrice-specific skills** newly authored.
- **23 total skill directories** under `.agents/skills/`.
- Root `AGENTS.md` and skill registry define discoverability, authority, local-only quality gates and task-based skill routing.

## 5. Verification and ongoing updates
**Documentation validation planned in this PR:** enumerate actual target `SKILL.md` paths, check YAML names/description, check linked reference paths, assert no duplicate names, unexpected product names, fake runnable gates, exposed secrets or hosted-CI requirements; review diff before PR merge.

**Not performed merely by transfer:** Go compilation, agent CLI discovery on a local workstation, source scraping permission checks, Pi profiling, delivery script execution, live adapter validations. These remain later implementation/release tasks.

Before importing future repo additions: compare default branch and source history, check license/private-data implications, map to leanPrice's authoritative domain, copy only applicable references, update this audit and independently review conflicts.
