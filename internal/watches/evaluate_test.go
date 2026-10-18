package watches

import (
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
)

func publicOffer(now time.Time, minor int64) pricing.Offer {
	return pricing.Offer{Money: pricing.Money{Minor: minor, Currency: "ZAR", Scale: 2}, Type: pricing.OfferStandard,
		Eligibility: pricing.EligibilityPublic, MinimumQty: 1, TermsKnown: true, Stock: pricing.StockInStock,
		ObservedAt: now.Add(-time.Minute), SourceApproved: true, ExactMatch: true, ObservationValid: true}
}

func TestEvaluateThresholdDoesNotCallOrdinaryPriceASale(t *testing.T) {
	now := time.Now().UTC()
	target := pricing.Money{Minor: 3000, Currency: "ZAR", Scale: 2}
	offer := publicOffer(now, 2999)
	offer.RetailerSaleClaim = false
	result := Evaluate(Rule{Version: 1, Target: &target, Operator: "any"}, offer, nil, now)
	if !result.Eligible || !result.Triggered || result.ReasonCode != "threshold_met" || result.Reasons[0] == "sale" {
		t.Fatalf("threshold evaluation = %#v", result)
	}
}

func TestEvaluateEnforcesIdentityFreshnessStockAndEligibility(t *testing.T) {
	now := time.Now().UTC()
	target := pricing.Money{Minor: 3000, Currency: "ZAR", Scale: 2}
	base := publicOffer(now, 2500)
	cases := []struct {
		name string
		edit func(*pricing.Offer, *Rule)
		want string
	}{
		{name: "weak match", edit: func(o *pricing.Offer, _ *Rule) { o.ExactMatch = false }, want: "offer_not_verified"},
		{name: "unapproved source", edit: func(o *pricing.Offer, _ *Rule) { o.SourceApproved = false }, want: "offer_not_verified"},
		{name: "stale", edit: func(o *pricing.Offer, _ *Rule) { o.ObservedAt = now.Add(-25 * time.Hour) }, want: "stale_offer"},
		{name: "unknown terms", edit: func(o *pricing.Offer, _ *Rule) { o.TermsKnown = false }, want: "offer_terms_unknown"},
		{name: "out of stock", edit: func(o *pricing.Offer, _ *Rule) { o.Stock = pricing.StockOutOfStock }, want: "out_of_stock"},
		{name: "member default", edit: func(o *pricing.Offer, _ *Rule) {
			o.Type, o.Eligibility = pricing.OfferMember, pricing.EligibilityMember
		}, want: "member_offer_not_enabled"},
		{name: "coupon", edit: func(o *pricing.Offer, _ *Rule) {
			o.Type, o.Eligibility = pricing.OfferCoupon, pricing.EligibilityUnknown
		}, want: "coupon_not_supported"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			offer, rule := base, Rule{Version: 1, Target: &target}
			test.edit(&offer, &rule)
			result := Evaluate(rule, offer, nil, now)
			if result.Triggered || result.SuppressedReason != test.want {
				t.Fatalf("evaluation = %#v", result)
			}
		})
	}
}

func TestEvaluateUnknownStockHasHonestPriceOnlyDisclaimer(t *testing.T) {
	now := time.Now().UTC()
	target := pricing.Money{Minor: 3000, Currency: "ZAR", Scale: 2}
	offer := publicOffer(now, 2900)
	offer.Stock = pricing.StockUnknown
	result := Evaluate(Rule{Version: 1, Target: &target}, offer, nil, now)
	if !result.Triggered || !result.StockDisclaimer {
		t.Fatalf("unknown stock alert lacks disclaimer: %#v", result)
	}
}

func TestEvaluatePercentageDropRequiresComparableBaselineAndHandlesOverflow(t *testing.T) {
	now := time.Now().UTC()
	bps := 1000
	offer := publicOffer(now, 9_000_000_000_000_000_000)
	offer.ListingID = "listing-1"
	baselineOffer := publicOffer(now.Add(-time.Hour), 9_100_000_000_000_000_000)
	baselineOffer.ListingID = "listing-1"
	baseline := &Baseline{ListingID: "listing-1", Offer: baselineOffer, SourceApproved: true, ExactMatch: true, Valid: true}
	result := Evaluate(Rule{Version: 1, DropBasisPoints: &bps}, offer, baseline, now)
	if !result.Eligible || result.Triggered || result.SuppressedReason != "conditions_not_met" {
		t.Fatalf("sub-threshold decrease evaluation = %#v", result)
	}
	offer.Money.Minor = 8_000_000_000_000_000_000
	result = Evaluate(Rule{Version: 1, DropBasisPoints: &bps}, offer, baseline, now)
	if !result.Triggered || result.ReasonCode != "observed_decrease" {
		t.Fatalf("large decrease evaluation = %#v", result)
	}
	missing := Evaluate(Rule{Version: 1, DropBasisPoints: &bps}, offer, nil, now)
	if missing.Triggered || missing.Reasons[0] != "baseline_missing_or_incomparable" {
		t.Fatalf("missing baseline evaluation = %#v", missing)
	}
}

func TestEvaluateRequiresSameListingBaseline(t *testing.T) {
	now := time.Now().UTC()
	bps := 1000
	offer := publicOffer(now, 800)
	offer.ListingID = "listing-a"
	baselineOffer := publicOffer(now.Add(-time.Hour), 1000)
	baselineOffer.ListingID = "listing-b"
	baseline := &Baseline{ListingID: "listing-b", Offer: baselineOffer, SourceApproved: true, ExactMatch: true, Valid: true}
	result := Evaluate(Rule{Version: 1, DropBasisPoints: &bps}, offer, baseline, now)
	if result.Triggered || result.Reasons[0] != "baseline_missing_or_incomparable" {
		t.Fatalf("cross-listing baseline evaluation = %#v", result)
	}
}

func TestEvaluateDoesNotTreatMissingRelativeHistoryAsRecovery(t *testing.T) {
	now := time.Now().UTC()
	target := pricing.Money{Minor: 5000, Currency: "ZAR", Scale: 2}
	bps := 1000
	result := Evaluate(Rule{Version: 1, Target: &target, DropBasisPoints: &bps, Operator: "any"}, publicOffer(now, 6000), nil, now)
	if result.Triggered || result.RecoveryConfirmed {
		t.Fatalf("missing baseline was treated as a recovery: %#v", result)
	}
}

func TestEvaluateMultibuyUsesCompleteBasketTerms(t *testing.T) {
	now := time.Now().UTC()
	target := pricing.Money{Minor: 5000, Currency: "ZAR", Scale: 2}
	offer := publicOffer(now, 5000)
	offer.Type, offer.MinimumQty = pricing.OfferMultibuy, 2
	result := Evaluate(Rule{Version: 1, Target: &target}, offer, nil, now)
	if result.Triggered || result.SuppressedReason != "multibuy_not_enabled" {
		t.Fatalf("multibuy was not opt-in: %#v", result)
	}
	result = Evaluate(Rule{Version: 1, Target: &target, AllowMultibuy: true}, offer, nil, now)
	if !result.Triggered || result.BasketQuantity != 2 || result.Amount.Minor != 5000 {
		t.Fatalf("multibuy basket evaluation = %#v", result)
	}
}
