package pricing

import (
	"errors"
	"sort"
	"time"
)

type OfferType string

const (
	OfferStandard      OfferType = "standard"
	OfferPublicSpecial OfferType = "public_special"
	OfferMember        OfferType = "member"
	OfferMultibuy      OfferType = "multibuy"
	OfferCoupon        OfferType = "coupon"
	OfferUnknown       OfferType = "unknown"
)

type Eligibility string

const (
	EligibilityPublic  Eligibility = "public"
	EligibilityMember  Eligibility = "member"
	EligibilityUnknown Eligibility = "unknown"
)

type StockState string

const (
	StockInStock    StockState = "in_stock"
	StockOutOfStock StockState = "out_of_stock"
	StockUnknown    StockState = "unknown"
)

var ErrInvalidOffer = errors.New("invalid offer")

type Offer struct {
	ListingID            string
	Money                Money
	Type                 OfferType
	Eligibility          Eligibility
	MinimumQty           int64
	TermsKnown           bool
	Stock                StockState
	ValidFrom            *time.Time
	ValidUntil           *time.Time
	ObservedAt           time.Time
	SourceApproved       bool
	ExactMatch           bool
	ObservationValid     bool
	RetailerSaleClaim    bool
	ClaimedPreviousMinor *int64
}

type RankingGroup struct {
	MinimumQty int64
	Currency   string
	Stock      StockState
	Offers     []Offer
}

type rankingKey struct {
	minimumQty int64
	currency   string
	stock      StockState
}

func (o Offer) Validate() error {
	if err := o.Money.Validate(); err != nil || o.MinimumQty <= 0 || o.ObservedAt.IsZero() {
		return ErrInvalidOffer
	}
	switch o.Type {
	case OfferStandard, OfferPublicSpecial, OfferMember, OfferMultibuy, OfferCoupon, OfferUnknown:
	default:
		return ErrInvalidOffer
	}
	switch o.Eligibility {
	case EligibilityPublic, EligibilityMember, EligibilityUnknown:
	default:
		return ErrInvalidOffer
	}
	switch o.Stock {
	case StockInStock, StockOutOfStock, StockUnknown:
	default:
		return ErrInvalidOffer
	}
	if o.Type == OfferMultibuy && o.MinimumQty < 2 {
		return ErrInvalidOffer
	}
	if o.Type != OfferMultibuy && o.Type != OfferCoupon && o.MinimumQty != 1 {
		return ErrInvalidOffer
	}
	if o.ValidFrom != nil && o.ValidUntil != nil && o.ValidUntil.Before(*o.ValidFrom) {
		return ErrInvalidOffer
	}
	if o.ClaimedPreviousMinor != nil && *o.ClaimedPreviousMinor <= 0 {
		return ErrInvalidOffer
	}
	return nil
}

func RankOffers(offers []Offer, now time.Time, maxAge time.Duration) []RankingGroup {
	groups := make(map[rankingKey]*RankingGroup)
	for _, offer := range offers {
		if err := offer.Validate(); err != nil || !offer.SourceApproved || !offer.ExactMatch || !offer.ObservationValid || maxAge <= 0 || now.Before(offer.ObservedAt) || now.Sub(offer.ObservedAt) > maxAge {
			continue
		}
		if offer.Eligibility != EligibilityPublic || offer.Type != OfferStandard && offer.Type != OfferPublicSpecial || !offer.TermsKnown {
			continue
		}
		if offer.Stock == StockOutOfStock || offer.ValidFrom != nil && now.Before(*offer.ValidFrom) || offer.ValidUntil != nil && !now.Before(*offer.ValidUntil) {
			continue
		}
		key := rankingKey{minimumQty: offer.MinimumQty, currency: offer.Money.Currency, stock: offer.Stock}
		group := groups[key]
		if group == nil {
			group = &RankingGroup{MinimumQty: offer.MinimumQty, Currency: offer.Money.Currency, Stock: offer.Stock}
			groups[key] = group
		}
		group.Offers = append(group.Offers, offer)
	}
	result := make([]RankingGroup, 0, len(groups))
	for _, group := range groups {
		sort.SliceStable(group.Offers, func(i, j int) bool {
			if group.Offers[i].Stock != group.Offers[j].Stock {
				return group.Offers[i].Stock == StockInStock
			}
			return group.Offers[i].Money.Minor < group.Offers[j].Money.Minor
		})
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MinimumQty != result[j].MinimumQty {
			return result[i].MinimumQty < result[j].MinimumQty
		}
		if result[i].Currency != result[j].Currency {
			return result[i].Currency < result[j].Currency
		}
		return result[i].Stock < result[j].Stock
	})
	return result
}
