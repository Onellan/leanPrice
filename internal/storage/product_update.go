package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/internal/match"
)

const MaxListingsToReReviewOnProductEdit = 500

// UpdateCanonicalProduct records an auditable identity edit and demotes prior
// exact listings to possible so old matches cannot continue ranking silently.
func (s *Store) UpdateCanonicalProduct(ctx context.Context, adminID string, product ProductSummary, expectedUpdatedAt int64, rationale string, now time.Time) error {
	if strings.TrimSpace(adminID) == "" || strings.TrimSpace(product.ID) == "" || expectedUpdatedAt <= 0 ||
		strings.TrimSpace(rationale) == "" || len(rationale) > 900 || strings.ContainsRune(rationale, '\x00') {
		return errors.New("storage: administrator, product, expected version and bounded rationale are required")
	}
	if strings.TrimSpace(product.Name) == "" || len(product.Name) > 240 || len(product.Brand) > 120 || len(product.ProductLine) > 120 ||
		len(product.Variant) > 160 || product.PackCount < 1 || product.PackCount > 10000 {
		return errors.New("storage: product identity fields exceed their limits")
	}
	quantityFields := strings.Fields(product.Quantity)
	if len(quantityFields) == 2 && quantityFields[1] != product.QuantityUnit || len(quantityFields) < 1 || len(quantityFields) > 2 {
		return errors.New("storage: quantity display unit must match the canonical unit")
	}
	parts := strings.Split(quantityFields[0], "/")
	if len(parts) != 2 {
		return errors.New("storage: quantity must be canonical numerator/denominator")
	}
	numerator, numeratorErr := strconv.ParseInt(parts[0], 10, 64)
	denominator, denominatorErr := strconv.ParseInt(parts[1], 10, 64)
	if numeratorErr != nil || denominatorErr != nil || numerator <= 0 || denominator <= 0 {
		return errors.New("storage: quantity must use positive integer numerator and denominator")
	}
	dimension := ""
	switch product.QuantityUnit {
	case "L":
		dimension = "volume"
	case "kg":
		dimension = "mass"
	case "item":
		dimension = "count"
	default:
		return errors.New("storage: supported quantity units are L, kg and item")
	}
	if product.GTIN != "" {
		if normalized, valid := match.NormalizeGTIN(product.GTIN); !valid || normalized != product.GTIN {
			return errors.New("storage: GTIN must be a valid 8, 12, 13 or 14 digit code")
		}
	}
	nowMS := now.UTC().UnixMilli()
	if nowMS <= 0 {
		return errors.New("storage: product timestamp is invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin product identity edit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var oldName, oldBrand, oldLine, oldVariant, oldUnit, oldGTIN string
	var oldNumerator, oldDenominator, oldPackCount, oldUpdatedAt int64
	err = tx.QueryRowContext(ctx, `SELECT display_name,COALESCE(brand,''),COALESCE(product_line,''),COALESCE(variant,''),
		quantity_numerator,quantity_denominator,quantity_unit,pack_count,COALESCE(gtin,''),updated_at
		FROM canonical_products WHERE id=? AND archived=0`, product.ID).
		Scan(&oldName, &oldBrand, &oldLine, &oldVariant, &oldNumerator, &oldDenominator, &oldUnit, &oldPackCount, &oldGTIN, &oldUpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("storage: read product before edit: %w", err)
	}
	if oldUpdatedAt != expectedUpdatedAt {
		return ErrConflict
	}
	product.Name = strings.TrimSpace(product.Name)
	product.Brand = strings.TrimSpace(product.Brand)
	product.ProductLine = strings.TrimSpace(product.ProductLine)
	product.Variant = strings.TrimSpace(product.Variant)
	changed := oldName != product.Name || oldBrand != product.Brand || oldLine != product.ProductLine || oldVariant != product.Variant ||
		oldNumerator != numerator || oldDenominator != denominator || oldUnit != product.QuantityUnit || oldPackCount != int64(product.PackCount) || oldGTIN != product.GTIN
	if !changed {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM listings WHERE canonical_product_id=? AND match_state='exact' ORDER BY id LIMIT ?`, product.ID, MaxListingsToReReviewOnProductEdit+1)
	if err != nil {
		return fmt.Errorf("storage: select exact listings affected by identity edit: %w", err)
	}
	listingIDs := make([]string, 0, 8)
	for rows.Next() {
		var listingID string
		if err := rows.Scan(&listingID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("storage: scan listing affected by identity edit: %w", err)
		}
		listingIDs = append(listingIDs, listingID)
	}
	rowsErr := rows.Err()
	closeErr := rows.Close()
	if rowsErr != nil || closeErr != nil {
		return fmt.Errorf("storage: read listings affected by identity edit: %w", errors.Join(rowsErr, closeErr))
	}
	if len(listingIDs) > MaxListingsToReReviewOnProductEdit {
		return errors.New("storage: too many exact listings to safely re-review in one edit")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE canonical_products SET display_name=?,brand=NULLIF(?,''),product_line=NULLIF(?,''),
		variant=NULLIF(?,''),quantity_numerator=?,quantity_denominator=?,quantity_dimension=?,quantity_unit=?,pack_count=?,gtin=NULLIF(?,''),updated_at=?
		WHERE id=? AND updated_at=?`, product.Name, product.Brand, product.ProductLine, product.Variant, numerator, denominator,
		dimension, product.QuantityUnit, product.PackCount, product.GTIN, nowMS, product.ID, expectedUpdatedAt); err != nil {
		return fmt.Errorf("storage: update canonical product identity: %w", err)
	}
	if len(listingIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE listings SET match_state='possible',match_reason='canonical_identity_changed',match_algorithm='identity-v1'
			WHERE canonical_product_id=? AND match_state='exact'`, product.ID); err != nil {
			return fmt.Errorf("storage: demote listings after identity edit: %w", err)
		}
		for _, listingID := range listingIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO match_reviews(listing_id,admin_user_id,previous_state,new_state,rationale,created_at)
				VALUES (?,?,'exact','possible',?,?)`, listingID, adminID, "Canonical product identity changed: "+strings.TrimSpace(rationale), nowMS); err != nil {
				return fmt.Errorf("storage: audit listing re-review %q: %w", listingID, err)
			}
		}
	}
	detail, err := json.Marshal(struct {
		PreviousName string `json:"previous_name"`
		CurrentName  string `json:"current_name"`
		Listings     int    `json:"listings_requiring_review"`
	}{oldName, product.Name, len(listingIDs)})
	if err != nil {
		return fmt.Errorf("storage: encode product identity audit: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_audit(admin_user_id,action_code,object_type,object_id,detail_json,created_at)
		VALUES (?,'product.identity_updated','canonical_product',?,?,?)`, adminID, product.ID, string(detail), nowMS); err != nil {
		return fmt.Errorf("storage: audit product identity edit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit product identity edit: %w", err)
	}
	return nil
}
