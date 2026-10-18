---
name: resolving-merge-conflicts
description: Resolve authorised Git merge conflicts while preserving leanPrice contracts and unrelated work.
---

# resolving-merge-conflicts

**Trigger:** merge conflict during local development or user-authorised PR merge.

Inspect both parents/merge base, affected slices, tests, migrations and authoritative PRD/architecture/ADRs. Prefer semantic reconciliation over picking ours/theirs wholesale. Never destroy an unrelated user change, alter terminal sign-off history or silently drop a failure test. Price semantics, source URL security, SQLite migrations and Docker memory limits require explicit review for conflicting edits.

Resolve minimal files, run appropriate local checks and migration/fixture tests, inspect combined diff and produce a conflict-resolution rationale. If uncertain product policy or a divergent migration cannot be reconciled safely, report BLOCKED rather than selecting arbitrarily. GitHub Actions is not a required validation route.

**Adapted from:** Onellan/tockrPlatform resolving-merge-conflicts.
