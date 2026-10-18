# Review and Visual QA

Load this for material UI review, visual critique, final polish, or substantial migration verification.

## Review dimensions

Review across five dimensions.

### Product and UX

- primary task is clear;
- information hierarchy is correct;
- copy matches the user's mental model;
- important states exist;
- interactions are predictable for the surface mode;
- no product/security/domain authority moved into presentation accidentally.

### Visual design

- direction is intentional and appropriate to mode;
- typography hierarchy is coherent;
- spacing and density reveal relationships;
- colours have explicit roles;
- action hierarchy is clear;
- decoration serves the brief rather than generic aesthetics;
- adjacent screens/components use a consistent vocabulary.

### System quality

- component decision ladder was followed;
- existing tokens/components are reused correctly;
- abstractions live at the right semantic layer;
- no duplicate styling/component system was added;
- no new visual-debt variant was introduced without authority.

### Accessibility and responsiveness

- semantics, labels, keyboard, focus, contrast;
- dialog/overlay behaviour;
- zoom/reflow where relevant;
- narrow/wide layout behaviour;
- touch targets where relevant;
- important content/actions remain reachable.

### Technical quality

- real states are implemented;
- assets/fonts/JavaScript are proportionate;
- no avoidable layout shift/overflow;
- interaction failures are handled;
- implementation is maintainable within the project's architecture.

Visual preference alone is not an engineering finding. A finding should identify a concrete usability, accessibility, system-consistency, performance, technical, or approved-brief problem.

## Batched visual inspection

When rendering/browser/image evidence is available, inspect representative states together rather than making endless screenshot loops.

Round 1 should normally cover, as applicable:

- wide/desktop;
- narrow/mobile;
- primary populated state;
- one material empty/error/loading state;
- changed interactive controls;
- keyboard/focus path.

Inspect:

- hierarchy;
- spacing/alignment;
- typography/wrapping;
- density;
- overflow/clipping;
- component consistency;
- contrast/state communication;
- navigation/context;
- realistic copy/data.

Batch the concrete findings, fix them coherently, then run one confirmation round.

## Stop rule

After confirmation, stop polishing unless a named unresolved defect remains.

Do not continue because:

- a slightly different radius might also work;
- another colour could be interesting;
- the page could be made more "premium";
- aesthetic restlessness suggests a new direction.

If the direction itself is wrong, classify that explicitly as redesign/brief mismatch rather than endless micro-polish.

## Completion questions

Before passing the frontend aspect of a change, answer:

- Does the surface's purpose read within seconds?
- Is novelty calibrated to mode?
- Does the UI belong to this product/subject rather than a generic template?
- For Operate surfaces, are standard interactions familiar unless difference is justified?
- Is there any removable decoration competing with information?
- Are applicable non-happy states intentional?
- Is keyboard/focus behaviour sound?
- Does the composition structurally adapt?
- Did visual ambition create avoidable runtime cost?
- Did the change create another design-system variant rather than resolving/reusing one?

## Evidence discipline

Automated tests and rendered evidence prove different things. Do not substitute one for the other.

- tests can prove behaviour/semantics/invariants;
- rendered/browser evidence can prove composition, wrapping, visual hierarchy, overflow, focus visibility, and interaction presentation;
- accessibility automation can catch classes of issues but does not replace keyboard/semantic reasoning;
- screenshots prove only the captured state.

Report only material findings and evidence; do not restate the skill checklist.