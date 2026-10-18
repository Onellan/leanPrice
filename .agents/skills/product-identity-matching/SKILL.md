---
name: product-identity-matching
description: Match exact retailer listings to canonical products without conflating variant, pack, barcode or units.
---

# product-identity-matching

**Trigger:** product ingestion, normalization, candidate search, exact grouping, fuzzy match or per-unit comparison.

Canonical identity = brand, product line, explicit variant (e.g. scent), quantity and dimension, count/pack, GTIN/remote SKU when reliable. A retailer listing belongs to a source/context and may be EXACT, POSSIBLE, REJECTED or SUSPENDED. Hard conflicting brand, variant, unit dimension or pack count overrides any numeric text-similarity score; GTIN agreement does not override contradictory physical properties.

Normalize Unicode, punctuation, unit spellings, number/decimal locale and per-pack expressions deterministically. A 750ml single bottle != 1L or 2×750ml. Prefer exact high-confidence match with explainable field evidence; ambiguous candidate needs explicit human confirmation and retains original decision audit. Per-litre value comparisons may be shown as an **alternative** view but never redefine same-SKU equality.

Tests must include Domestos Lavender vs Original; 750ml vs 1L; single vs multi-pack; GTIN missing/colliding; ambiguous retailer abbreviations; locality-specific SKU; conflicting title and JSON-LD. Output match reason/version used by downstream price/alert eligibility. Never trigger an alert on a POSSIBLE candidate.

**Derived for leanPrice from:** Tockr domain-modeling and ActualPlus correctness/evidence principles, using leanPrice FR-002–008 and AC-M*.
