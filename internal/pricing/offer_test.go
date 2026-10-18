package pricing

import (
	"testing"
	"time"
)

func TestRankOffersExcludesStaleConditionalAndUnavailablePrices(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	valid := func(minor int64, stock StockState) Offer {
		return Offer{
			Money: Money{Minor: minor, Currency: "ZAR", Scale: 2}, Type: OfferStandard, Eligibility: EligibilityPublic,
			MinimumQty: 1, TermsKnown: true, Stock: stock, ObservedAt: now.Add(-time.Hour),
			SourceApproved: true, ExactMatch: true, ObservationValid: true,
		}
	}
	stale := valid(2500, StockInStock)
	stale.ObservedAt = now.Add(-25 * time.Hour)
	member := valid(2400, StockInStock)
	member.Type, member.Eligibility = OfferMember, EligibilityMember
	multibuy := valid(5000, StockInStock)
	multibuy.Type, multibuy.MinimumQty = OfferMultibuy, 2
	coupon := valid(1000, StockInStock)
	coupon.Type, coupon.TermsKnown = OfferCoupon, false
	wrongMatch := valid(1000, StockInStock)
	wrongMatch.ExactMatch = false
	outOfStock := valid(2000, StockOutOfStock)

	groups := RankOffers([]Offer{
		valid(3100, StockInStock), valid(2700, StockUnknown), outOfStock,
		stale, member, multibuy, coupon, wrongMatch,
	}, now, 24*time.Hour)
	if len(groups) != 2 {
		t.Fatalf("ranking groups = %#v, want in-stock and unknown-stock public groups", groups)
	}
	if groups[0].Stock != StockInStock || len(groups[0].Offers) != 1 || groups[0].Offers[0].Money.Minor != 3100 {
		t.Fatalf("in-stock ranking = %#v", groups[0])
	}
	if groups[1].Stock != StockUnknown || len(groups[1].Offers) != 1 || groups[1].Offers[0].Money.Minor != 2700 {
		t.Fatalf("unknown-stock ranking = %#v", groups[1])
	}
}

func TestRankOffersSeparatesCurrenciesAndOfferQuantities(t *testing.T) {
	now := time.Now().UTC()
	base := Offer{Type: OfferStandard, Eligibility: EligibilityPublic, MinimumQty: 1, TermsKnown: true, Stock: StockInStock,
		ObservedAt: now, SourceApproved: true, ExactMatch: true, ObservationValid: true}
	zar := base
	zar.Money = Money{Minor: 1000, Currency: "ZAR", Scale: 2}
	usd := base
	usd.Money = Money{Minor: 100, Currency: "USD", Scale: 2}
	groups := RankOffers([]Offer{zar, usd}, now, time.Minute)
	if len(groups) != 2 || groups[0].Currency == groups[1].Currency {
		t.Fatalf("cross-currency offers were combined: %#v", groups)
	}
}
