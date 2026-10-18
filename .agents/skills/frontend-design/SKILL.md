---
name: frontend-design
description: Framework- and application-agnostic frontend design and UI engineering guidance for planning, implementing, migrating, critiquing, and reviewing user-facing interfaces. Use for material UI/presentation work; classify the surface, preserve product authority, follow the incumbent design system where appropriate, and load only the triggered reference guidance.
metadata:
  version: 3.0.0
---

# Frontend Design & UI Engineering

Use this skill for material user-facing interface work. It governs **how** frontend design is reasoned about and executed; it does not define product requirements or prescribe a framework, renderer, styling system, component library, or client/server architecture.

## 1. Trigger

Load this skill when work materially creates, changes, migrates, critiques, or reviews any of the following:

- pages, screens, layouts, shells, navigation, forms, tables, dashboards, settings, onboarding, or empty states;
- visual hierarchy, typography, spacing, colour, icons, imagery, tokens, or component variants;
- responsive behaviour, accessibility, frontend interaction, motion, loading/error/validation states, or UX copy;
- frontend design-system/component architecture;
- a presentation migration or visual redesign.

Do not load it for backend-only work with no material UI consequence. For mixed work, apply it only to the presentation/interaction surface while preserving the owning system authority.

## 2. Authority

Use the project's own authority first. In general:

1. explicit user/client brief;
2. product/domain/security requirements;
3. approved design/brand specification;
4. approved implementation plan;
5. repository/application architecture and engineering standards;
6. established design system and incumbent visual language;
7. this skill;
8. existing implementation where higher authority is silent;
9. generic industry practice.

The brief wins on intentionally pinned visual choices. Visual preference must never weaken product behaviour, security, authorization, privacy, data integrity, domain meaning, platform conventions, or other higher authority.

## 3. Classify before designing

For material frontend work, form this compact routing signature before loading references or making design decisions:

```text
UI: Mode=[Persuade|Operate|Read|Experience]
    Change=[Refine|Extend|Redesign|Greenfield|Migrate]
    Scope=[Component|Surface|System]
    Authority=[Established|Partial|None]
    Risk=[A11Y|RESP|INTERACTION|PERF|DESIGN-SYSTEM|CONTENT|I18N|NONE...]
```

Use it internally for implementation/review and record it in a plan when it materially affects execution or validation.

- **Persuade**: the visitor should believe, decide, or act.
- **Operate**: the user is completing a task.
- **Read**: the reader is trying to understand information.
- **Experience**: the artifact/experience itself leads.

Load `reference/modes.md` when mode-specific design decisions matter.

## 4. Determine what is already true

Before editing an existing interface, inspect representative screens/code, tokens, components, assets, and any design authority.

- **Refine**: preserve the incumbent identity and behaviour; correct bounded problems.
- **Extend**: inherit the established surface/system; solve only the new purpose, states, hierarchy, and interaction.
- **Redesign**: preserve product/content truth and constraints, but replace the visual world rather than splitting the difference.
- **Greenfield**: no meaningful visual authority exists; establish a deliberate system.
- **Migrate**: change implementation architecture while preserving approved behaviour unless the plan explicitly authorizes visual/interaction changes.

Missing design documentation does not automatically mean greenfield. Coherent incumbent code and shipped UI are evidence of visual authority.

## 5. Core MUST rules

Frontend work MUST:

- preserve higher product/security/domain authority;
- inspect incumbent visual truth before changing an established interface;
- classify the surface and change type for material work;
- reuse the established design system/architecture unless change is explicitly authorized;
- cover applicable interactive and system states, not only the populated happy path;
- provide deliberate responsive behaviour for affected compositions;
- preserve or improve keyboard/focus semantics and accessible naming;
- use real or representative content rather than designing only around ideal placeholders;
- visually inspect material UI changes when rendering/browser/image evidence is available;
- use bounded QA: inspect once in a batched pass, fix findings coherently, confirm once, then stop unless a concrete defect remains;
- avoid silently expanding visual debt.

## 6. Core SHOULD rules

Frontend work SHOULD:

- derive aesthetic decisions from the subject, audience, task, environment, and existing identity;
- use visual structure to encode information rather than decorate;
- keep sustained prose around 65–75ch where appropriate and generally below about 80 characters per line;
- use semantic tokens for recurring colour/spacing/type/state decisions where a design system exists;
- keep operational interfaces familiar, efficient, and consistent;
- spend visual boldness in one or two deliberate places rather than everywhere;
- make controls name their outcome and keep vocabulary consistent through a flow;
- prefer native/platform semantics over rebuilding standard behaviour without a reason;
- keep motion purposeful and user-state-driven, especially on Operate surfaces;
- treat performance as part of visual quality.

These are defaults, not authority over a clear brief.

## 7. Defaults to challenge

When the brief/system leaves an axis open, do not fall automatically into generic generated-design habits such as:

- every section in same-sized rounded cards;
- nested cards used only to create hierarchy;
- decorative gradient washes or gradient text;
- glass/blur/glow without a subject-specific reason;
- giant metric + small label + supporting stat as a default hero;
- eyebrow labels above every heading;
- all-caps metadata and decorative monospace used as generic "technical" styling;
- meaningless `01 / 02 / 03` numbering;
- pill shapes for every control;
- identical entrance animations on every section;
- generic abstract geometric backgrounds unrelated to the subject;
- modal-first interaction design;
- novelty navigation in routine task software;
- card-list replacements for genuinely tabular desktop data.

These are not blanket bans. A coherent brief may legitimately require them. The failure is using them without deciding.

## 8. Distinctiveness by mode

Do not apply the same novelty target to every surface.

```text
Persuade   → high opportunity for distinctive visual identity
Experience → high opportunity for distinctive visual identity
Read       → moderate; comprehension and reading comfort dominate
Operate    → low-to-moderate; earned familiarity and task fluency dominate
```

A conventional table with excellent hierarchy can be the superior design for operational software. A marketing surface that looks like a generic admin dashboard may be a failure. See `reference/modes.md`.

## 9. Visual direction

For a substantial new surface or redesign, establish a compact direction before implementation:

```text
Purpose    = audience + primary job + desired outcome
Mode       = Persuade | Operate | Read | Experience
Thesis     = one specific sentence describing the visual idea
Typography = family/roles/scale/density
Colour     = foundational surfaces + accent + semantic roles
Layout     = composition/grid/alignment/density/responsive strategy
Signature  = one characteristic visual or interaction idea
```

Then run the generic-design test:

> Could this design belong to an unrelated product by changing only its logo and copy?

If yes, revise the free design axes before building. For established Operate surfaces, also run the familiarity test: did a standard interaction become strange without purpose?

Load `reference/visual-direction.md` for greenfield/redesign, major typography/colour/layout work, or a bland/generic result.

## 10. Design-system and component discipline

Before creating presentation code, use this decision ladder:

```text
1. Reuse an existing component unchanged.
2. Use/configure an existing supported variant.
3. Compose existing components.
4. Extend an existing component API without breaking its semantic boundary.
5. Create a new reusable component.
6. Create a one-off implementation only when reuse would be dishonest or harmful.
```

Do not add a new token, spacing convention, visual treatment, interaction vocabulary, or component pattern when an adequate existing one exists.

If the incumbent system is inconsistent, choose one explicitly:

```text
preserve the local convention
repair inconsistency within authorized scope
create an approved system-level correction
```

Never add a third accidental variation.

### Extraction rule

Promote a repeated pattern to a shared abstraction when either:

- it appears in roughly three meaningful places **and** shares the same semantic purpose; or
- interaction/accessibility complexity justifies centralization earlier.

Visual similarity alone is not semantic identity.

Load `reference/components-and-systems.md` for component, form, table, navigation, state, or design-system work.

## 11. Production quality floor

For affected surfaces, deliberately consider all applicable states:

```text
default | hover | focus | active | selected | disabled
loading | empty | validation-error | system-error
permission-denied | not-found | degraded/offline | success
```

Not every component needs every state; every applicable state needs intentional treatment.

For substantial work, check:

- hierarchy, spacing, alignment, density, typography, wrapping, overflow;
- labels, errors, action naming, empty/loading guidance;
- responsive structure and touch targets where applicable;
- semantic markup/platform semantics, keyboard operation, focus, contrast, reduced motion;
- real content edge cases, localization/long strings where applicable;
- unnecessary JavaScript/assets/fonts/layout shift/render cost.

Load `reference/accessibility-responsive-performance.md` when those risks are present.

## 12. Interaction and copy

Actions should describe outcomes (`Save changes`, `Create project`, `Try again`) rather than generic mechanics (`Submit`, `Proceed`) unless the generic term is genuinely clearest.

Keep the same vocabulary through a flow. Errors should state what happened and, where possible, what the user can do. Empty states should explain the situation and offer an action only when the user can actually take one.

Motion should communicate state, causality, feedback, or spatial relationship. In routine task interfaces, prefer short functional transitions and avoid page-load choreography. Respect reduced-motion preferences.

## 13. Migration discipline

For implementation-only frontend migration:

```text
existing behaviour proven
        ↓
inventory current UI contract
        ↓
map concepts to target architecture
        ↓
migrate
        ↓
visual/responsive/accessibility inspection
        ↓
existing behaviour proven again
```

Use green-preserving refactor evidence when behaviour is already covered. Do not manufacture a failing test for a pure refactor. Use RED → GREEN for genuinely new behaviour or a reproducible regression. Remove legacy presentation paths only after active callers are proven absent.

## 14. Verify → repair → confirm → stop

For material visual changes when rendering evidence is available:

```text
build/complete the scoped change
        ↓
inspection round 1
wide + narrow + key states together
        ↓
fix concrete findings in one coherent batch
        ↓
inspection round 2
confirm repairs
        ↓
STOP
```

Additional rounds require a named unresolved defect, not aesthetic restlessness.

A UI is not proven correct merely because it compiles or tests pass; nor is a screenshot sufficient to prove product behaviour. Both engineering evidence and rendered evidence matter according to scope.

Load `reference/review-and-qa.md` for substantial implementation review, visual critique, or pre-release polish.

## 15. Planner / Implementer / Reviewer use

### Planner

For UI-bearing work, identify the routing signature, affected surfaces, incumbent authority, important states, component/token impacts, responsive/accessibility/interaction/performance risks, and required rendered evidence. Avoid vague requirements such as "modernize the UI."

### Implementer

Inspect adjacent surfaces and reusable system pieces before editing. Follow the component ladder. Preserve higher authority. Build applicable states, run relevant automated evidence, visually inspect material changes when possible, batch-fix concrete defects, and report only material UI decisions/evidence.

### Reviewer

Review product/UX, visual hierarchy, design-system integrity, accessibility/responsiveness, interaction states, and technical quality. Visual preference alone is not an engineering finding. Findings should name a concrete usability, accessibility, consistency, system, performance, or approved-brief problem.

## 16. Reference router

Load only the references triggered by the current `UI:` signature and work:

| Trigger | Reference |
| --- | --- |
| Mode materially affects design | `reference/modes.md` |
| Greenfield/redesign/major visual direction or generic-looking result | `reference/visual-direction.md` |
| Components, forms, tables, navigation, states, tokens, extraction | `reference/components-and-systems.md` |
| A11Y/RESP/PERF/I18N risk or production hardening | `reference/accessibility-responsive-performance.md` |
| Material UI review, visual critique, final QA/polish | `reference/review-and-qa.md` |

Do not pre-read every reference. Load a newly triggered reference later if evidence exposes a new risk.

## 17. Completion and output discipline

Frontend work is complete only when the affected scope has evidence, as applicable, for:

```text
purpose understood
+ higher authority preserved
+ design direction intentional
+ information hierarchy clear
+ design-system use coherent
+ applicable states covered
+ responsive/accessibility behaviour deliberate
+ interaction technically sound
+ performance proportionate
+ rendered result inspected
```

Do not restate this skill in reports. Report only the routing signature when useful, surfaces changed, reused/new abstractions, material design/accessibility/responsive/interaction decisions, validation performed, and deviations/blockers.