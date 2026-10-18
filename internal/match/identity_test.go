package match

import (
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
)

func quantity(numerator, denominator, packs int64) *pricing.Quantity {
	return &pricing.Quantity{Numerator: numerator, Denominator: denominator, Dimension: pricing.DimensionVolume, PackCount: packs}
}

func domestosLavender750() Identity {
	return Identity{Brand: "Domestos", Line: "Power Fresh", Variant: "Lavender", VariantKnown: true, Quantity: quantity(750, 1000, 1)}
}

func TestExactMatchRequiresPhysicalIdentityAndUnicodeNormalization(t *testing.T) {
	canonical := domestosLavender750()
	listing := Identity{Brand: "DOMESTOS", Line: "Power-Fresh", Variant: "LAVENDER", VariantKnown: true, Quantity: quantity(3, 4, 1)}
	decision := Evaluate(canonical, listing)
	if decision.Status != StatusExact || decision.Algorithm != AlgorithmVersion {
		t.Fatalf("equivalent identity decision = %#v", decision)
	}

	composed := NormalizeText("Café Creme")
	decomposed := NormalizeText("Cafe\u0301 Creme")
	if composed != decomposed {
		t.Fatalf("Unicode normalization differs: %q != %q", composed, decomposed)
	}
}

func TestPhysicalMismatchesOverrideGTINAgreement(t *testing.T) {
	canonical := domestosLavender750()
	canonical.GTIN = "4006381333931"
	cases := []struct {
		name   string
		mutate func(*Identity)
		reason string
	}{
		{name: "different scent", mutate: func(i *Identity) { i.Variant = "Original" }, reason: "variant_mismatch"},
		{name: "one litre", mutate: func(i *Identity) { i.Quantity = quantity(1, 1, 1) }, reason: "quantity_or_pack_mismatch"},
		{name: "twin pack", mutate: func(i *Identity) { i.Quantity = quantity(750, 1000, 2) }, reason: "quantity_or_pack_mismatch"},
		{name: "brand", mutate: func(i *Identity) { i.Brand = "Other" }, reason: "brand_mismatch"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			listing := canonical
			test.mutate(&listing)
			decision := Evaluate(canonical, listing)
			if decision.Status != StatusRejected || decision.Reason != test.reason {
				t.Fatalf("mismatch decision = %#v", decision)
			}
		})
	}
}

func TestGTINChecksumAndPossibleMatch(t *testing.T) {
	if normalized, ok := NormalizeGTIN("4006 3813 33931"); !ok || normalized != "4006381333931" {
		t.Fatalf("valid EAN-13 normalization = %q, %v", normalized, ok)
	}
	if _, ok := NormalizeGTIN("4006381333932"); ok {
		t.Fatal("invalid EAN-13 check digit accepted")
	}
	canonical := domestosLavender750()
	listing := canonical
	listing.VariantKnown = false
	decision := Evaluate(canonical, listing)
	if decision.Status != StatusPossible {
		t.Fatalf("unknown variant was auto-linked: %#v", decision)
	}
}

func TestManualConfirmationIsAuditedAndDoesNotMutatePreviousDecision(t *testing.T) {
	previous := Decision{Status: StatusPossible, Reason: "insufficient_identity_fields", Algorithm: AlgorithmVersion}
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	review, err := ApplyManualConfirmation(previous, "owner", at, "Checked package and scent against listing", "decision-1")
	if err != nil {
		t.Fatalf("ApplyManualConfirmation(): %v", err)
	}
	if review.Before.Status != StatusPossible || review.After.Status != StatusExact || review.Actor != "owner" || review.PreviousID != "decision-1" {
		t.Fatalf("review does not retain audit lineage: %#v", review)
	}
	if previous.Status != StatusPossible || previous.ReviewedBy != "" {
		t.Fatalf("previous decision was mutated: %#v", previous)
	}
	if _, err := ApplyManualConfirmation(Decision{Status: StatusRejected}, "owner", at, "override", "decision-2"); err == nil {
		t.Fatal("hard-rejected product identity was manually accepted")
	}
}
