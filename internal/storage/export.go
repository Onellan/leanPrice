package storage

import (
	"context"
	"fmt"
)

const (
	MaxHistoryExportPage = 100
	MaxHistoryExportRows = 10000
)

// ExportedHistory contains only validated offer history from exact matches.
// It deliberately excludes source URLs, credentials and internal identifiers.
type ExportedHistory struct {
	ProductName string
	Brand       string
	ProductLine string
	Variant     string
	Quantity    string
	PackCount   int
	Retailer    string
	AmountMinor int64
	Currency    string
	Scale       int
	Stock       string
	ObservedAt  int64
	Validation  string
	OfferKind   string
	MinimumQty  int
	OfferID     string
}

// ExportValidatedHistoryPage returns a keyset-paginated, bounded page of
// successful validated public-price history. Source policy and exact-match
// status are rechecked at export time.
func (s *Store) ExportValidatedHistoryPage(ctx context.Context, afterObservedAt int64, afterOfferID string, limit int) ([]ExportedHistory, error) {
	if afterObservedAt < 0 || len(afterOfferID) > 80 || limit < 1 || limit > MaxHistoryExportPage {
		return nil, fmt.Errorf("storage: invalid history export cursor or page size")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.display_name,COALESCE(p.brand,''),COALESCE(p.product_line,''),
		COALESCE(p.variant,''),p.quantity_numerator || '/' || p.quantity_denominator || ' ' || p.quantity_unit,
		p.pack_count,r.name,o.amount_minor,o.currency,o.currency_scale,ob.observed_stock,ob.observed_at,
		ob.validation_state,o.offer_kind,o.minimum_qty,o.id
		FROM observed_offers o
		JOIN observations ob ON ob.id=o.observation_id
		JOIN listings l ON l.id=ob.listing_id
		JOIN canonical_products p ON p.id=l.canonical_product_id
		JOIN sources s ON s.id=l.source_id
		JOIN retailers r ON r.id=s.retailer_id
		WHERE ob.fetch_status='success' AND ob.validation_state='valid' AND l.match_state='exact'
		AND s.assessment_state='approved' AND s.enabled=1
		AND o.eligibility_state='public' AND o.offer_kind IN ('standard','public_special')
		AND (?=0 OR ob.observed_at>? OR (ob.observed_at=? AND o.id>?))
		ORDER BY ob.observed_at,o.id LIMIT ?`, afterObservedAt, afterObservedAt, afterObservedAt, afterOfferID, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: query validated history export: %w", err)
	}
	defer rows.Close()
	items := make([]ExportedHistory, 0, limit)
	for rows.Next() {
		var item ExportedHistory
		if err := rows.Scan(&item.ProductName, &item.Brand, &item.ProductLine, &item.Variant, &item.Quantity,
			&item.PackCount, &item.Retailer, &item.AmountMinor, &item.Currency, &item.Scale, &item.Stock,
			&item.ObservedAt, &item.Validation, &item.OfferKind, &item.MinimumQty, &item.OfferID); err != nil {
			return nil, fmt.Errorf("storage: scan validated history export: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: read validated history export: %w", err)
	}
	return items, nil
}
