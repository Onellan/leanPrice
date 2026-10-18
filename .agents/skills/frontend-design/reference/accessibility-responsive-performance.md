# Accessibility, Responsive Design, Performance, and Content Robustness

Load this when `A11Y`, `RESP`, `PERF`, `I18N`, or production-hardening risk is present.

## Accessibility

Target WCAG 2.2 AA unless project authority requires a different standard.

Check applicable concerns:

- semantic HTML/platform semantics;
- heading hierarchy;
- accessible names and labels;
- keyboard operation and logical focus order;
- visible focus and focus restoration;
- dialog/overlay semantics;
- error association and announcements;
- status not conveyed by colour alone;
- colour contrast;
- reduced-motion preferences;
- 200% zoom/reflow where applicable;
- meaningful link/button text;
- table semantics.

Practical contrast floor unless stronger authority applies:

- normal text: at least 4.5:1;
- large text: at least 3:1.

Prefer native/platform semantics over recreating standard behaviour with generic elements and JavaScript.

## Responsive design

Responsive design is structural adaptation, not desktop shrinkage.

Choose representative widths/device classes appropriate to the product. At narrow sizes verify:

- primary context remains visible;
- important actions remain reachable;
- navigation remains usable;
- forms remain understandable;
- tables use an explicit strategy;
- overlays remain within the viewport;
- touch targets remain usable where relevant;
- content does not overflow accidentally.

Breakpoints should respond to composition needs rather than arbitrary device labels. Do not use fluid typography as a substitute for structural layout changes.

## Real-content robustness

Test realistic extremes when relevant:

- long names and labels;
- long translations;
- missing values;
- large/negative numbers;
- currencies and localized formats;
- timestamps/time zones;
- pluralization;
- RTL layouts where supported;
- validation/error copy longer than the happy path.

Do not optimize layouts solely around short English placeholder text.

## Performance

Visual quality includes runtime quality. Inspect for proportionate risks such as:

- unnecessary JavaScript;
- oversized/unoptimized images;
- excessive font families/weights/files;
- render-blocking assets;
- cumulative layout shift;
- expensive blur/filter effects;
- animation on layout-heavy properties;
- very large SVG/DOM trees;
- duplicate CSS/component frameworks;
- redundant dependencies;
- avoidable repeated rendering or network work.

A visually ambitious result that feels slow is not high-quality frontend design.

## Loading/degraded states

Choose feedback based on scope:

- local skeleton/progress for local work;
- progressive rendering where useful;
- optimistic updates only when failure semantics are safe;
- explicit progress for longer operations;
- degraded/offline messaging where the product can encounter it.

Avoid a global spinner when only one small region is busy. Skeletons should resemble the actual content structure rather than generic grey decoration.

## Platform/browser details

The shipped interface includes details models often overlook: autofill, selection, caret, focus rings, underline treatment, numeric formatting, safe areas, native validation, scroll behaviour, and browser/platform theme.

Customize these only when doing so improves coherence or usability. In operational software, familiar platform behaviour often beats branded novelty.