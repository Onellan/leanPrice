package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
)

const MaxWatchEvaluations = 500

type WatchEvaluation struct {
	WatchID       string
	ProductID     string
	ListingID     string
	ObservationID string
	OfferID       string
	Rule          WatchRule
	Current       pricing.Offer
	Baseline      *pricing.Offer
}

// ListWatchEvaluations returns bounded latest exact offers for active watches.
// A relative baseline is selected from the same listing and comparable offer
// terms only; another retailer's history is never substituted.
func (s *Store) ListWatchEvaluations(ctx context.Context, now time.Time, maxAge time.Duration, limit int) ([]WatchEvaluation, error) {
	if now.IsZero() || maxAge <= 0 || maxAge > 365*24*time.Hour {
		return nil, errors.New("storage: watch evaluation time and freshness window are required")
	}
	if limit <= 0 || limit > MaxWatchEvaluations {
		limit = MaxWatchEvaluations
	}
	rows, err := s.db.QueryContext(ctx, `SELECT w.id,p.id,l.id,ob.id,o.id,w.current_rule_version,rv.target_minor,
		rv.drop_basis_points,rv.condition_operator,rv.allow_member,rv.allow_multibuy,COALESCE(rv.notification_channel,''),
		ob.observed_at,ob.fetch_status,ob.validation_state,ob.observed_stock,
		o.offer_kind,o.currency,o.currency_scale,o.amount_minor,o.minimum_qty,o.eligibility_state,
		o.valid_from,o.valid_until,s.assessment_state,s.enabled,l.match_state
		FROM watches w JOIN canonical_products p ON p.id=w.canonical_product_id
		JOIN watch_rule_versions rv ON rv.watch_id=w.id AND rv.version=w.current_rule_version
		JOIN listings l ON l.canonical_product_id=p.id AND (w.listing_id IS NULL OR w.listing_id=l.id) AND l.match_state='exact'
		JOIN sources s ON s.id=l.source_id AND s.assessment_state='approved' AND s.enabled=1
		JOIN observations ob ON ob.listing_id=l.id AND ob.id=(SELECT ob2.id FROM observations ob2
			WHERE ob2.listing_id=l.id ORDER BY ob2.observed_at DESC,ob2.id DESC LIMIT 1)
		JOIN observed_offers o ON o.observation_id=ob.id
		WHERE w.active=1 AND o.currency='ZAR' AND o.currency_scale=2 AND o.offer_kind!='coupon'
		ORDER BY ob.observed_at DESC,w.id,l.id,o.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: select watch evaluation offers: %w", err)
	}
	items := make([]WatchEvaluation, 0, limit)
	for rows.Next() {
		var item WatchEvaluation
		var target, drop, validFrom, validUntil sql.NullInt64
		var allowMember, allowMulti, enabled int
		var kind, currency, eligibility, fetchStatus, validation, stock, assessment, matchState, operator string
		var scale int
		var amount int64
		var minimumQty int
		var observedAt int64
		if err := rows.Scan(&item.WatchID, &item.ProductID, &item.ListingID, &item.ObservationID, &item.OfferID,
			&item.Rule.Version, &target, &drop, &operator, &allowMember, &allowMulti, &item.Rule.NotificationChannel,
			&observedAt, &fetchStatus, &validation, &stock, &kind, &currency, &scale, &amount, &minimumQty,
			&eligibility, &validFrom, &validUntil, &assessment, &enabled, &matchState); err != nil {
			return nil, fmt.Errorf("storage: scan watch evaluation offer: %w", err)
		}
		if target.Valid {
			value := target.Int64
			item.Rule.TargetMinor = &value
		}
		if drop.Valid {
			value := int(drop.Int64)
			item.Rule.DropBasisPoints = &value
		}
		item.Rule.ConditionOperator = operator
		item.Rule.AllowMember, item.Rule.AllowMultibuy = allowMember == 1, allowMulti == 1
		item.Current = pricing.Offer{
			ListingID: item.ListingID, Money: pricing.Money{Minor: amount, Currency: currency, Scale: scale},
			Type: pricing.OfferType(kind), Eligibility: toEligibility(eligibility), MinimumQty: int64(minimumQty),
			TermsKnown: kind != "unknown" && eligibility != "unknown", Stock: pricing.StockState(stock),
			ObservedAt: time.UnixMilli(observedAt).UTC(), SourceApproved: assessment == "approved" && enabled == 1,
			ExactMatch: matchState == "exact", ObservationValid: fetchStatus == "success" && validation == "valid",
		}
		if validFrom.Valid {
			value := time.UnixMilli(validFrom.Int64).UTC()
			item.Current.ValidFrom = &value
		}
		if validUntil.Valid {
			value := time.UnixMilli(validUntil.Int64).UTC()
			item.Current.ValidUntil = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("storage: read watch evaluation offers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("storage: close watch evaluation rows: %w", err)
	}
	for index := range items {
		if items[index].Rule.DropBasisPoints == nil {
			continue
		}
		baseline, found, baselineErr := s.previousComparableOffer(ctx, items[index], items[index].Current.ObservedAt.UnixMilli(), maxAge)
		if baselineErr != nil {
			return nil, baselineErr
		}
		if found {
			items[index].Baseline = &baseline
		}
	}
	return items, nil
}

func (s *Store) previousComparableOffer(ctx context.Context, current WatchEvaluation, currentAt int64, maxAge time.Duration) (pricing.Offer, bool, error) {
	var offer pricing.Offer
	var kind, currency, eligibility, stock string
	var scale, minimumQty int
	var amount, observedAt int64
	var validFrom, validUntil sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT o.offer_kind,o.currency,o.currency_scale,o.amount_minor,o.minimum_qty,
		o.eligibility_state,ob.observed_at,ob.observed_stock,o.valid_from,o.valid_until
		FROM observations ob JOIN observed_offers o ON o.observation_id=ob.id
		JOIN listings l ON l.id=ob.listing_id JOIN sources s ON s.id=l.source_id
		WHERE l.id=? AND l.match_state='exact' AND s.assessment_state='approved' AND s.enabled=1
		AND ob.fetch_status='success' AND ob.validation_state='valid' AND ob.observed_at<=? AND ob.observed_at>=?
		AND o.currency=? AND o.currency_scale=? AND o.eligibility_state=? AND o.minimum_qty=? AND o.offer_kind=?
		AND ob.id!=? AND ob.observed_stock!='out_of_stock'
		AND (o.valid_from IS NULL OR o.valid_from<=ob.observed_at) AND (o.valid_until IS NULL OR o.valid_until>ob.observed_at)
		ORDER BY ob.observed_at DESC,ob.id DESC,o.id LIMIT 1`, current.ListingID, currentAt-1, currentAt-maxAge.Milliseconds(),
		current.Current.Money.Currency, current.Current.Money.Scale, eligibilityValue(current.Current.Eligibility),
		current.Current.MinimumQty, string(current.Current.Type), current.ObservationID).
		Scan(&kind, &currency, &scale, &amount, &minimumQty, &eligibility, &observedAt, &stock, &validFrom, &validUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return pricing.Offer{}, false, nil
	}
	if err != nil {
		return pricing.Offer{}, false, fmt.Errorf("storage: find comparable watch baseline: %w", err)
	}
	offer = pricing.Offer{ListingID: current.ListingID, Money: pricing.Money{Minor: amount, Currency: currency, Scale: scale},
		Type: pricing.OfferType(kind), Eligibility: toEligibility(eligibility), MinimumQty: int64(minimumQty),
		TermsKnown: kind != "unknown" && eligibility != "unknown", Stock: pricing.StockState(stock),
		ObservedAt: time.UnixMilli(observedAt).UTC(), SourceApproved: true, ExactMatch: true, ObservationValid: true}
	if validFrom.Valid {
		value := time.UnixMilli(validFrom.Int64).UTC()
		offer.ValidFrom = &value
	}
	if validUntil.Valid {
		value := time.UnixMilli(validUntil.Int64).UTC()
		offer.ValidUntil = &value
	}
	return offer, true, nil
}

func toEligibility(value string) pricing.Eligibility {
	switch value {
	case "public":
		return pricing.EligibilityPublic
	case "conditional":
		return pricing.EligibilityMember
	default:
		return pricing.EligibilityUnknown
	}
}

func eligibilityValue(value pricing.Eligibility) string {
	switch value {
	case pricing.EligibilityPublic:
		return "public"
	case pricing.EligibilityMember:
		return "conditional"
	default:
		return "unknown"
	}
}
