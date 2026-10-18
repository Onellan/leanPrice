---
name: sqlite-job-and-outbox-integrity
description: Protect leanPrice SQLite migrations, observation history, job leases and price-alert delivery across crashes.
---

# sqlite-job-and-outbox-integrity

**Trigger:** SQLite schema/queries/migrations, durable scheduler, job queue, watches, alerts, webhook delivery, backup, restore or retention.

Source of truth is one SQLite database in WAL mode with FK/CHECK/UNIQUE constraints, bounded busy-timeout and short transactions. Each observation is traceable and immutable; failed parsing cannot overwrite last valid history with 0. Queue jobs persist with status/lease/next due and dedupe key; never spawn a goroutine per listing. Claim leases atomically; network fetches and webhooks execute strictly **outside** DB write transactions.

Evaluate rules only from validated fresh exact eligible offers. Persist a unique logical alert episode and delivery-outbox intent atomically; retries remain bounded and reconstructible. External webhook delivery is normally **at least once**, not provably exactly once; send stable event IDs and idempotency headers where supported. Preserve rule version, offer evidence, eligibility and re-arm state through restarts.

Back up via a WAL-consistent online SQLite snapshot, checksum and integrity check; never copy only the main DB while active. Migration tests cover initial install, supported upgrades, failed migrations, restart/kill during lease/commit, concurrent check dedupe, disk-full, backup while active and clean restore. Protect alert evidence from premature retention purges.

**Derived for leanPrice from:** ActualPlus durable mutation/audit principles and Tockr package/queue governance, reconciled with leanPrice DATA-CONTRACTS and AC-O*/AC-A*.
