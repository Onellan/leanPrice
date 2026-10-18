package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const CurrentOfferFreshness = 36 * time.Hour

// ListProductOffers returns only fresh observations from the latest check for
// each approved, enabled exact listing. Conditional terms remain visible and
// are not promoted to the public verified-deal list.
func (s *Store) ListProductOffers(ctx context.Context, productID string, now time.Time, limit int) ([]ProductOfferSummary, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" || now.IsZero() {
		return nil, fmt.Errorf("storage: product and current time are required for offer listing")
	}
	limit = boundedLimit(limit)
	current := now.UTC().UnixMilli()
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,r.name,s.base_host,l.url,o.amount_minor,o.currency,o.currency_scale,ob.observed_stock,
		ob.observed_at,o.offer_kind,o.eligibility_state,o.minimum_qty
		FROM listings l
		JOIN sources s ON s.id=l.source_id
		JOIN retailers r ON r.id=s.retailer_id
		JOIN observations ob ON ob.id=(SELECT latest.id FROM observations latest WHERE latest.listing_id=l.id
			ORDER BY latest.observed_at DESC,latest.id DESC LIMIT 1)
		JOIN observed_offers o ON o.observation_id=ob.id
		WHERE l.canonical_product_id=? AND l.match_state='exact'
		AND s.assessment_state='approved' AND s.enabled=1
		AND ob.fetch_status='success' AND ob.validation_state='valid' AND ob.observed_at<=? AND ob.observed_at>=?
		AND (o.valid_from IS NULL OR o.valid_from<=?) AND (o.valid_until IS NULL OR o.valid_until>?)
		ORDER BY o.currency,o.currency_scale,o.amount_minor,ob.observed_at DESC,l.id,o.id LIMIT ?`, productID, current,
		now.Add(-CurrentOfferFreshness).UTC().UnixMilli(), current, current, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: query product offers: %w", err)
	}
	defer rows.Close()
	items := make([]ProductOfferSummary, 0, limit)
	for rows.Next() {
		var item ProductOfferSummary
		if err := rows.Scan(&item.ListingID, &item.Retailer, &item.SourceHost, &item.ListingURL, &item.AmountMinor,
			&item.Currency, &item.Scale, &item.Stock, &item.ObservedAt, &item.OfferKind, &item.Eligibility, &item.MinimumQty); err != nil {
			return nil, fmt.Errorf("storage: scan product offer: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: read product offers: %w", err)
	}
	return items, nil
}

// ProductObservedPriceRange summarizes only independently recorded, fresh-at-
// observation public single-pack prices with known in-stock state. It reports
// a sampled range and never claims retailer savings or a universal market low.
func (s *Store) ProductObservedPriceRange(ctx context.Context, productID string, from, through time.Time) (*ProductPriceRange, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" || from.IsZero() || through.IsZero() || !from.Before(through) || through.Sub(from) > 365*24*time.Hour {
		return nil, fmt.Errorf("storage: product price range window must be positive and at most one year")
	}
	var minimum, maximum sql.NullInt64
	var summary ProductPriceRange
	err := s.db.QueryRowContext(ctx, `SELECT MIN(o.amount_minor),MAX(o.amount_minor),COUNT(DISTINCT ob.id),COUNT(DISTINCT r.id),
		COALESCE(MIN(ob.observed_at),0),COALESCE(MAX(ob.observed_at),0),COALESCE(MAX(o.currency),''),COALESCE(MAX(o.currency_scale),0)
		FROM observed_offers o JOIN observations ob ON ob.id=o.observation_id
		JOIN listings l ON l.id=ob.listing_id JOIN sources s ON s.id=l.source_id JOIN retailers r ON r.id=s.retailer_id
		WHERE l.canonical_product_id=? AND l.match_state='exact' AND s.assessment_state='approved' AND s.enabled=1
		AND ob.fetch_status='success' AND ob.validation_state='valid' AND ob.observed_stock='in_stock'
		AND ob.observed_at>=? AND ob.observed_at<=? AND o.currency='ZAR' AND o.currency_scale=2
		AND o.eligibility_state='public' AND o.offer_kind IN ('standard','public_special') AND o.minimum_qty=1
		AND (o.valid_from IS NULL OR o.valid_from<=ob.observed_at)
		AND (o.valid_until IS NULL OR o.valid_until>ob.observed_at)`, productID,
		from.UTC().UnixMilli(), through.UTC().UnixMilli()).Scan(&minimum, &maximum, &summary.ObservationCount,
		&summary.RetailerCount, &summary.FirstObservedAt, &summary.LastObservedAt, &summary.Currency, &summary.CurrencyScale)
	if err != nil {
		return nil, fmt.Errorf("storage: summarize observed product prices: %w", err)
	}
	if summary.ObservationCount == 0 || !minimum.Valid || !maximum.Valid {
		return nil, nil
	}
	summary.MinimumMinor, summary.MaximumMinor = minimum.Int64, maximum.Int64
	return &summary, nil
}

// ListProductPriceSamples returns a bounded chronological sample of exact,
// validated public ZAR prices with known in-stock state. The sample is for
// display only and never feeds price eligibility or alert decisions.
func (s *Store) ListProductPriceSamples(ctx context.Context, productID string, from, through time.Time, limit int) ([]ProductPriceSample, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" || from.IsZero() || through.IsZero() || !from.Before(through) || through.Sub(from) > 365*24*time.Hour {
		return nil, fmt.Errorf("storage: product price sample window must be positive and at most one year")
	}
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT observed_at,retailer,amount_minor,currency,currency_scale FROM (
		SELECT ob.id,ob.observed_at,r.name AS retailer,MIN(o.amount_minor) AS amount_minor,
			o.currency,o.currency_scale
		FROM observations ob JOIN listings l ON l.id=ob.listing_id
		JOIN sources s ON s.id=l.source_id JOIN retailers r ON r.id=s.retailer_id
		JOIN observed_offers o ON o.observation_id=ob.id
		WHERE l.canonical_product_id=? AND l.match_state='exact' AND s.assessment_state='approved' AND s.enabled=1
		AND ob.fetch_status='success' AND ob.validation_state='valid' AND ob.observed_stock='in_stock'
		AND ob.observed_at>=? AND ob.observed_at<=? AND o.currency='ZAR' AND o.currency_scale=2
		AND o.eligibility_state='public' AND o.offer_kind IN ('standard','public_special') AND o.minimum_qty=1
		AND (o.valid_from IS NULL OR o.valid_from<=ob.observed_at)
		AND (o.valid_until IS NULL OR o.valid_until>ob.observed_at)
		GROUP BY ob.id,ob.observed_at,r.name,o.currency,o.currency_scale
		ORDER BY ob.observed_at DESC,ob.id DESC LIMIT ?
	) samples ORDER BY observed_at,retailer`, productID, from.UTC().UnixMilli(), through.UTC().UnixMilli(), limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list product price samples: %w", err)
	}
	defer rows.Close()
	samples := make([]ProductPriceSample, 0, limit)
	for rows.Next() {
		var sample ProductPriceSample
		if err := rows.Scan(&sample.ObservedAt, &sample.Retailer, &sample.AmountMinor, &sample.Currency, &sample.Scale); err != nil {
			return nil, fmt.Errorf("storage: scan product price sample: %w", err)
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}
