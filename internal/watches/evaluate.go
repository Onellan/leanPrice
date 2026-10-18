// Package watches evaluates fresh, exact, policy-approved offers against
// immutable watch rules. It contains no storage or network side effects.
package watches

import (
	"math/big"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
)

const DefaultFreshness = 24 * time.Hour
const DefaultBaselineAge = 90 * 24 * time.Hour

type Rule struct {
	Version         int
	Target          *pricing.Money
	DropBasisPoints *int
	Operator        string
	AllowMember     bool
	AllowMultibuy   bool
	Freshness       time.Duration
	BaselineAge     time.Duration
}

type Baseline struct {
	ListingID      string
	Offer          pricing.Offer
	SourceApproved bool
	ExactMatch     bool
	Valid          bool
}

type Evaluation struct {
	Eligible          bool
	Triggered         bool
	ReasonCode        string
	Reasons           []string
	SuppressedReason  string
	StockDisclaimer   bool
	RecoveryConfirmed bool
	BasketQuantity    int64
	Amount            pricing.Money
}

func Evaluate(rule Rule, current pricing.Offer, baseline *Baseline, now time.Time) Evaluation {
	result := Evaluation{BasketQuantity: current.MinimumQty, Amount: current.Money}
	if !validRule(rule) {
		result.SuppressedReason = "invalid_rule"
		return result
	}
	if err := current.Validate(); err != nil || !current.SourceApproved || !current.ExactMatch || !current.ObservationValid {
		result.SuppressedReason = "offer_not_verified"
		return result
	}
	freshness := rule.Freshness
	if freshness <= 0 {
		freshness = DefaultFreshness
	}
	if now.Before(current.ObservedAt) || now.Sub(current.ObservedAt) > freshness {
		result.SuppressedReason = "stale_offer"
		return result
	}
	if current.ValidFrom != nil && now.Before(*current.ValidFrom) || current.ValidUntil != nil && !now.Before(*current.ValidUntil) {
		result.SuppressedReason = "offer_outside_validity"
		return result
	}
	if current.Stock == pricing.StockOutOfStock {
		result.SuppressedReason = "out_of_stock"
		return result
	}
	if !current.TermsKnown {
		result.SuppressedReason = "offer_terms_unknown"
		return result
	}
	switch current.Type {
	case pricing.OfferStandard, pricing.OfferPublicSpecial:
		if current.Eligibility != pricing.EligibilityPublic {
			result.SuppressedReason = "offer_not_public"
			return result
		}
	case pricing.OfferMember:
		if !rule.AllowMember || current.Eligibility != pricing.EligibilityMember {
			result.SuppressedReason = "member_offer_not_enabled"
			return result
		}
	case pricing.OfferMultibuy:
		if !rule.AllowMultibuy || current.Eligibility != pricing.EligibilityPublic {
			result.SuppressedReason = "multibuy_not_enabled"
			return result
		}
	case pricing.OfferCoupon:
		result.SuppressedReason = "coupon_not_supported"
		return result
	default:
		result.SuppressedReason = "offer_terms_unknown"
		return result
	}
	result.Eligible = true
	result.StockDisclaimer = current.Stock == pricing.StockUnknown

	var targetMet, dropMet, targetComparable, dropComparable bool
	conditions := 0
	if rule.Target != nil {
		conditions++
		comparison, err := current.Money.Compare(*rule.Target)
		if err != nil {
			result.Eligible = false
			result.SuppressedReason = "currency_mismatch"
			return result
		}
		targetComparable = true
		targetMet = comparison <= 0
		if targetMet {
			result.Reasons = append(result.Reasons, "threshold_met")
		}
	}
	if rule.DropBasisPoints != nil {
		conditions++
		if baseline != nil && comparableBaseline(current, *baseline, now, rule) {
			dropComparable = true
			dropMet = meetsDrop(current.Money, baseline.Offer.Money, *rule.DropBasisPoints)
			if dropMet {
				result.Reasons = append(result.Reasons, "observed_decrease")
			}
		} else {
			result.Reasons = append(result.Reasons, "baseline_missing_or_incomparable")
		}
	}
	if conditions == 1 {
		result.Triggered = targetMet || dropMet
	} else if rule.Operator == "all" {
		result.Triggered = targetMet && dropMet
	} else {
		result.Triggered = targetMet || dropMet
	}
	if !result.Triggered {
		result.RecoveryConfirmed = (rule.Target == nil || targetComparable) && (rule.DropBasisPoints == nil || dropComparable)
		result.SuppressedReason = "conditions_not_met"
		return result
	}
	if len(result.Reasons) == 1 {
		result.ReasonCode = result.Reasons[0]
	} else {
		result.ReasonCode = "multiple_conditions_met"
	}
	return result
}

func validRule(rule Rule) bool {
	if rule.Version < 1 || rule.Target == nil && rule.DropBasisPoints == nil {
		return false
	}
	if rule.Target != nil && rule.Target.Validate() != nil {
		return false
	}
	if rule.DropBasisPoints != nil && (*rule.DropBasisPoints < 1 || *rule.DropBasisPoints > 9900) {
		return false
	}
	operator := rule.Operator
	if operator == "" {
		operator = "any"
	}
	if operator != "any" && operator != "all" || operator == "all" && (rule.Target == nil || rule.DropBasisPoints == nil) {
		return false
	}
	return rule.Freshness >= 0 && rule.BaselineAge >= 0
}

func comparableBaseline(current pricing.Offer, baseline Baseline, now time.Time, rule Rule) bool {
	if current.ListingID == "" || baseline.ListingID == "" || current.ListingID != baseline.ListingID ||
		!baseline.Valid || !baseline.SourceApproved || !baseline.ExactMatch || baseline.Offer.Validate() != nil {
		return false
	}
	previous := baseline.Offer
	if previous.ListingID != current.ListingID || previous.Eligibility != current.Eligibility || previous.MinimumQty != current.MinimumQty ||
		previous.Money.Currency != current.Money.Currency || previous.Type == pricing.OfferCoupon || previous.Type == pricing.OfferMultibuy && !rule.AllowMultibuy {
		return false
	}
	maxAge := rule.BaselineAge
	if maxAge <= 0 {
		maxAge = DefaultBaselineAge
	}
	return !now.Before(previous.ObservedAt) && now.Sub(previous.ObservedAt) <= maxAge
}

func meetsDrop(current, previous pricing.Money, basisPoints int) bool {
	comparison, err := current.Compare(previous)
	if err != nil || comparison >= 0 || current.Currency != previous.Currency || current.Scale != previous.Scale {
		return false
	}
	delta := new(big.Int).Sub(big.NewInt(previous.Minor), big.NewInt(current.Minor))
	left := new(big.Int).Mul(delta, big.NewInt(10000))
	right := new(big.Int).Mul(big.NewInt(previous.Minor), big.NewInt(int64(basisPoints)))
	return left.Cmp(right) >= 0
}
