# Components and Systems

Load this for design-system, token, component, form, table, navigation, or state-heavy work.

## Component decision ladder

Before creating presentation code:

```text
1. Reuse an existing component unchanged.
2. Use/configure an existing supported variant.
3. Compose existing components.
4. Extend an existing component API without breaking its semantic boundary.
5. Create a new reusable component.
6. Create a one-off implementation only when reuse would be dishonest or harmful.
```

Do not add a new component pattern, spacing rule, token, visual treatment, or interaction vocabulary when an adequate existing one already exists.

## Visual debt rule

When the incumbent system is inconsistent, explicitly choose one:

- preserve local convention;
- repair within authorized scope;
- create an approved system-level correction.

Never create a third accidental variant.

## Extraction rule

Promote a pattern to a shared abstraction when:

- it appears in roughly three meaningful places and serves the same semantic purpose; or
- interaction/accessibility complexity makes centralization valuable earlier.

Visual similarity is not enough. Three grey boxes do not automatically justify `GenericGrayBox`.

## Layering

Prefer a system shape like:

```text
tokens
→ primitives
→ reusable semantic components
→ product/domain components
→ pages/screens
```

Keep ownership clear:

- primitives should not own business rules;
- product components should not recreate primitive behaviour;
- presentation components should not become authorization or domain authority.

## Forms

Forms should favour successful completion over cleverness.

Use:

- visible labels;
- appropriate native/platform controls;
- concise help where needed;
- clear required/optional meaning;
- validation near the affected field;
- predictable keyboard order;
- preservation of safe entered values after errors where appropriate.

Avoid placeholder-only labels, vague `Submit` actions when a specific outcome is clearer, and critical validation hidden only in toasts.

## Tables and dense data

Keep tabular information tabular when rows/columns aid comparison.

Consider:

- column hierarchy;
- numerical alignment;
- sorting/filtering;
- row selection/actions;
- status treatment;
- empty/loading states;
- narrow-screen strategy.

For narrow screens choose deliberately among horizontal scroll, priority columns, expandable detail, or an alternate representation justified by the task. Accidental overflow is not responsive design.

## Navigation

Navigation should answer:

```text
Where am I?
What context am I in?
Where can I go?
What happens if I switch context?
```

Use consistent active states, labels, icons, breadcrumbs where useful, and context-switching patterns. Do not confuse context switching with normal navigation or hide major functionality behind ambiguous icons merely to reduce visual density.

## Cards and containers

Use a card when a unit genuinely benefits from separate grouping, state, or actions. Do not use cards as default scaffolding. Avoid nested cards used merely to simulate hierarchy.

Use border/elevation intentionally; redundant border + shadow treatments often add noise without information.

## Actions

Establish clear hierarchy: primary, secondary, subtle, destructive, navigation/link. Normally one action should be visually dominant inside one decision context.

Icon-only controls need accessible names and should be understandable without guessing.

## States

For each affected interactive component, consider applicable states:

```text
default | hover | focus | active | selected | disabled
loading | error | success
```

For data/screen surfaces also consider empty, not-found, permission-denied, degraded/offline, and validation states where applicable.

## Overlays

Use dialogs for genuinely bounded decisions/tasks requiring protected focus. Do not use a modal as the first answer to every secondary flow. Consider inline disclosure, sheets/drawers, or dedicated pages for larger workflows.

Ensure overlays handle focus, dismissal, viewport constraints, and stacking/clipping correctly.

## Motion

Motion should communicate state, causality, feedback, or spatial relationship. In routine product UI, prefer short functional transitions. Avoid decorative page-load sequences and repeated entrance effects.

## UX copy

Name things in the user's vocabulary, not implementation vocabulary. Keep action names consistent through the flow. Empty states should explain what is absent and what can actually be done next. Errors should say what happened and how to recover where possible.