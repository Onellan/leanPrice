---
name: domain-modeling
description: Protect leanPrice canonical product, retailer listing, price observation, promotion eligibility and alert domain meaning.
---

# domain-modeling

**Trigger:** model/schema changes, product matching, promotion logic, history, alerts, source data mapping.

Read PRD sections 6–8 and DATA-CONTRACTS.md. Model explicit canonical product, source listing and match state, immutable observation, conditional offer, watch rule version, alert episode and delivery outbox. Treat these entities separately; never silently reclassify an old observation as validated or rewrite confirmed historical facts without audited correction.

Invariants: integer positive cents + explicit currency, size/variant/pack hard-mismatch prevents exact match, stock unknown is not in stock, public/member/multibuy not interchangeable, stale observations never create current best or new alert, retailer historical crossed-out price is a claim not observed saving.

Identify authoritative writer and all readers for any field addition; make migrations, backfill, evaluation and UI evidence consistent. Do not leak raw retailer HTML into the domain or infer missing promo terms. Cover boundary fixtures and observability reasons for exclusion.

**Adapted from:** Onellan/tockrPlatform domain-modeling, shaped by ActualPlus traceability/immutability principles.
