# leanPrice agent skills — registry

This directory contains reusable skill guidance ported/adapted from the owner's Tockr/ActualPlus repositories, an imported framework-agnostic frontend skill with supporting references, and leanPrice-specific skills derived from the project's product and technical contracts. The skill taxonomy follows `SKILL.md` + YAML frontmatter.

**Skills do not supersede** [leanPrice PRD](../../docs/product/PRD.md), [acceptance](../../docs/product/ACCEPTANCE.md), [architecture](../../docs/technical/ARCHITECTURE.md) or [implementation plan](../../docs/technical/IMPLEMENTATION-PLAN.md). Read only skills triggered by the current task.

## Workflow skills (9)
| Skill | When to use |
|---|---|
| [plan-backlog-item](plan-backlog-item/SKILL.md) | Scope one active backlog item and ordered slices |
| [implement-backlog-plan](implement-backlog-plan/SKILL.md) | Execute authorised planned Go/UI/data work with local proof |
| [deliver-backlog-item](deliver-backlog-item/SKILL.md) | Orchestrate plan → implement → independent review/test → local certification |
| [deliver-slice](deliver-slice/SKILL.md) | One authorised slice, local checks and independent handoff |
| [code-review](code-review/SKILL.md) | Read-only engineering review on exact SHA |
| [test-backlog-implementation](test-backlog-implementation/SKILL.md) | Independent read-only acceptance |
| [test-execution](test-execution/SKILL.md) | Preflight tests and classify valid failure evidence |
| [audit-codebase](audit-codebase/SKILL.md) | Explicit whole-repository architecture/security audit |
| [resolving-merge-conflicts](resolving-merge-conflicts/SKILL.md) | Safe semantic conflict resolution |

## General engineering skills (8)
| Skill | When to use |
|---|---|
| [architecture-health-review](architecture-health-review/SKILL.md) | Package boundary and resource architecture health |
| [codebase-design](codebase-design/SKILL.md) | Deep modules, interfaces and clean seams |
| [coding-standards](coding-standards/SKILL.md) | Go, SQLite, SSR, security and RAM baseline |
| [diagnosing-bugs](diagnosing-bugs/SKILL.md) | Root-cause intermittent and complex failures |
| [domain-modeling](domain-modeling/SKILL.md) | Product/listing/offer/watch historical meaning |
| [frontend-design](frontend-design/SKILL.md) | UI/UX, accessibility, responsive layouts and visual review |
| [tdd](tdd/SKILL.md) | Behaviour-first tests and negative fixtures |
| [evaluate-agent-efficiency](evaluate-agent-efficiency/SKILL.md) | Compare agent workflow cost without quality regression |

The portable `frontend-design` skill includes upstream `reference/` material: modes, visual direction, components/systems, accessibility/responsive/performance and QA.

## leanPrice-specific domain/operations skills (6)
| Skill | When to use |
|---|---|
| [retailer-source-feasibility](retailer-source-feasibility/SKILL.md) | Assess retailer permissions, format and truthful capability |
| [safe-http-scraping](safe-http-scraping/SKILL.md) | Bounded HTTP, Colly/goquery extraction, SSRF/rate controls |
| [product-identity-matching](product-identity-matching/SKILL.md) | Variant, size, GTIN and pack correctness |
| [price-and-promotion-integrity](price-and-promotion-integrity/SKILL.md) | Integer-cents pricing, specials, member/multibuy rules |
| [sqlite-job-and-outbox-integrity](sqlite-job-and-outbox-integrity/SKILL.md) | Migrations, leases, outbox, crash/backup recovery |
| [pi-memory-certification](pi-memory-certification/SKILL.md) | Entire 500 MB Pi limit, cgroup/soak resource gates |

## Routing
- UI work: `frontend-design`; add `price-and-promotion-integrity` if price-related copy/eligibility changes.
- Scraping/source: `retailer-source-feasibility` → `safe-http-scraping` → `product-identity-matching` / `price-and-promotion-integrity`.
- Storage/background: `sqlite-job-and-outbox-integrity` plus `tdd`.
- Memory/runtime: `pi-memory-certification`, `codebase-design`, independent `test-execution`.
- Planning/delivery: use the corresponding workflow skill, then load only affected domain skills.
- Release: `code-review`, `test-backlog-implementation`, `test-execution`, `pi-memory-certification`.

## Workflow contract
Use three lanes: implementation, continuous lightweight boundary/quality guard, independent batch/final review. Multiple dependency-safe slices in one implementation run are allowed; record each slice's exact result. Tests run **locally** only and against a fixed candidate; no GitHub Actions. Planned `make` targets in documentation must not be represented as available until implemented. Never claim an app is running or any retailer is supported because a skill file exists.

## Provenance / maintenance
See [Transfer Audit](../../docs/skills/TRANSFER-AUDIT.md). Generic frontend files were copied from **public** `Onellan/tockrPlatform`; other shared skills are rewritten/adapted for leanPrice, not copied wholesale from private repos. The six domain/ops skills above are newly authored. When upstream evolves, compare semantic rules and update leanPrice consciously rather than replacing its product authority with upstream project instructions.
