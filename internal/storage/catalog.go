package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/internal/match"
	"github.com/Onellan/leanPrice/internal/pricing"
)

var ErrSourceNotApproved = errors.New("storage: source is not approved for enablement")

var (
	ErrSourceUnavailable = errors.New("storage: source is not approved and enabled")
	ErrCanonicalProduct  = errors.New("storage: listing has no canonical product")
)

type ListingForRefresh struct {
	ID                 string
	SourceID           string
	URL                string
	MatchState         string
	CanonicalProductID string
	CanonicalIdentity  match.Identity
}

func (s *Store) ListingForRefresh(ctx context.Context, listingID string) (ListingForRefresh, error) {
	var result ListingForRefresh
	var canonicalID sql.NullString
	var brand, line, variant, gtin sql.NullString
	var variantKnown int
	var quantity pricing.Quantity
	var dimension string
	var assessment string
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT l.id, l.source_id, l.url, l.match_state, l.canonical_product_id,
		p.brand, p.product_line, p.variant, p.quantity_numerator, p.quantity_denominator,
		p.quantity_dimension, p.pack_count, p.gtin,
		CASE WHEN p.variant IS NULL THEN 0 ELSE 1 END,
		s.assessment_state, s.enabled
		FROM listings l
	JOIN sources s ON s.id = l.source_id
	LEFT JOIN canonical_products p ON p.id = l.canonical_product_id
	WHERE l.id = ?`, listingID).Scan(&result.ID, &result.SourceID, &result.URL, &result.MatchState, &canonicalID,
		&brand, &line, &variant, &quantity.Numerator, &quantity.Denominator, &dimension, &quantity.PackCount, &gtin,
		&variantKnown, &assessment, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return ListingForRefresh{}, ErrNotFound
	}
	if err != nil {
		return ListingForRefresh{}, fmt.Errorf("storage: read listing for refresh: %w", err)
	}
	if assessment != "approved" || enabled != 1 {
		return ListingForRefresh{}, ErrSourceUnavailable
	}
	if !canonicalID.Valid {
		return ListingForRefresh{}, ErrCanonicalProduct
	}
	result.CanonicalProductID = canonicalID.String
	if brand.Valid {
		result.CanonicalIdentity.Brand = brand.String
	}
	if line.Valid {
		result.CanonicalIdentity.Line = line.String
	}
	if variant.Valid {
		result.CanonicalIdentity.Variant = variant.String
	}
	result.CanonicalIdentity.VariantKnown = variantKnown == 1
	if gtin.Valid {
		result.CanonicalIdentity.GTIN = gtin.String
	}
	switch dimension {
	case "volume":
		quantity.Dimension = pricing.DimensionVolume
	case "mass":
		quantity.Dimension = pricing.DimensionMass
	case "count":
		quantity.Dimension = pricing.DimensionCount
	default:
		return ListingForRefresh{}, errors.New("storage: canonical product has invalid quantity dimension")
	}
	result.CanonicalIdentity.Quantity = &quantity
	return result, nil
}

func (s *Store) MarkListingChecked(ctx context.Context, listingID string, checkedAt int64) error {
	if strings.TrimSpace(listingID) == "" || checkedAt <= 0 {
		return errors.New("storage: listing and positive check time are required")
	}
	result, err := s.db.ExecContext(ctx, "UPDATE listings SET last_checked_at = ? WHERE id = ?", checkedAt, listingID)
	if err != nil {
		return fmt.Errorf("storage: update listing check time: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RecordSourceEvent(ctx context.Context, id, sourceID, code, detail string, at time.Time) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(sourceID) == "" || at.IsZero() || !validEventCode(code) || len(detail) > 512 || strings.ContainsRune(detail, '\x00') {
		return errors.New("storage: invalid source event")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO source_events(id, source_id, event_code, detail_redacted, created_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?)`, id, sourceID, code, detail, at.UTC().UnixMilli()); err != nil {
		return fmt.Errorf("storage: record source event: %w", err)
	}
	return nil
}

// SetSourceEnabled changes only the operational switch. It never changes the
// source assessment and records the operator action in the same transaction.
func (s *Store) SetSourceEnabled(ctx context.Context, adminID, sourceID, eventID, auditID string, enabled bool, at time.Time) error {
	if strings.TrimSpace(adminID) == "" || strings.TrimSpace(sourceID) == "" || strings.TrimSpace(eventID) == "" || strings.TrimSpace(auditID) == "" || at.IsZero() {
		return errors.New("storage: source action requires an administrator, source, event, audit ID and timestamp")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin source action: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	value, action, event := 0, "source_paused", "operator_paused"
	if enabled {
		value, action, event = 1, "source_enabled", "operator_enabled"
	}
	query := "UPDATE sources SET enabled=?,updated_at=? WHERE id=?"
	if enabled {
		query += " AND assessment_state='approved' AND assessment_ref IS NOT NULL AND assessment_at IS NOT NULL"
	}
	result, err := tx.ExecContext(ctx, query, value, at.UTC().UnixMilli(), sourceID)
	if err != nil {
		return fmt.Errorf("storage: update source operational state: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: read source action result: %w", err)
	}
	if changed != 1 {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sources WHERE id=?", sourceID).Scan(&exists); err != nil {
			return fmt.Errorf("storage: inspect source action result: %w", err)
		}
		if exists == 0 {
			return ErrNotFound
		}
		if enabled {
			return ErrSourceNotApproved
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO source_events(id,source_id,event_code,detail_redacted,created_at)
		VALUES (?,?,?,'operator action',?)`, eventID, sourceID, event, at.UTC().UnixMilli()); err != nil {
		return fmt.Errorf("storage: record source event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_audit(id,admin_user_id,action_code,object_type,object_id,detail_json,created_at)
		VALUES (?,?,?,'source',?,'{}',?)`, auditID, adminID, action, sourceID, at.UTC().UnixMilli()); err != nil {
		return fmt.Errorf("storage: record source admin audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit source action: %w", err)
	}
	return nil
}

func validEventCode(value string) bool {
	if len(value) < 1 || len(value) > 80 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
