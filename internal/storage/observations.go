package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Onellan/leanPrice/internal/pricing"
)

// Observation is an immutable fetch attempt. Persisted prices are separate
// rows and are committed with their observation as one unit.
type Observation struct {
	ID                   string
	ListingID            string
	JobID                string
	ObservedAt           int64
	FetchStatus          string
	HTTPStatus           *int
	ParserVersion        string
	EvidenceDigest       string
	EvidenceLocation     string
	ObservedTitle        string
	ObservedBrand        string
	ObservedProductLine  string
	ObservedVariant      string
	ObservedGTIN         string
	ObservedQuantityJSON string
	FieldProvenanceJSON  string
	ObservedStock        string
	FailureCode          string
	SourceURL            string
	ValidationState      string
}

// ObservedOffer contains only integer-minor-unit amounts; no float money is
// accepted at the persistence boundary.
type ObservedOffer struct {
	ID                 string
	Kind               string
	Currency           string
	CurrencyScale      int
	AmountMinor        int64
	OriginalClaimMinor *int64
	MinimumQty         int
	MembershipLabel    string
	CouponCondition    string
	ValidFrom          *int64
	ValidUntil         *int64
	Evidence           string
	Eligibility        string
}

// PersistObservation stores a fetch attempt and all extracted offers in one
// transaction. A failure or invalid extraction cannot leave orphan offers.
func (s *Store) PersistObservation(ctx context.Context, observation Observation, offers []ObservedOffer) error {
	if err := validateObservation(observation, offers); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin observation persistence: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO observations
		(id, listing_id, job_id, observed_at, fetch_status, http_status, parser_version, evidence_digest,
		 evidence_location, observed_title, observed_brand, observed_product_line, observed_variant,
		 observed_gtin, observed_quantity_json, field_provenance_json, observed_stock,
		 failure_code, source_url, validation_state)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
		        NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?)`,
		observation.ID, observation.ListingID, observation.JobID, observation.ObservedAt,
		observation.FetchStatus, nullableInt(observation.HTTPStatus), observation.ParserVersion, observation.EvidenceDigest,
		observation.EvidenceLocation, observation.ObservedTitle, observation.ObservedBrand, observation.ObservedProductLine,
		observation.ObservedVariant, observation.ObservedGTIN, observation.ObservedQuantityJSON,
		defaultProvenance(observation.FieldProvenanceJSON),
		defaultStock(observation.ObservedStock), observation.FailureCode, observation.SourceURL, observation.ValidationState)
	if err != nil {
		return fmt.Errorf("storage: insert observation: %w", err)
	}
	for _, offer := range offers {
		_, err := tx.ExecContext(ctx, `INSERT INTO observed_offers
			(id, observation_id, offer_kind, currency, currency_scale, amount_minor, original_claim_minor, minimum_qty,
			 membership_label, coupon_condition, valid_from, valid_until, offer_evidence, eligibility_state)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)`,
			offer.ID, observation.ID, offer.Kind, offer.Currency, offer.CurrencyScale, offer.AmountMinor, offer.OriginalClaimMinor,
			offer.MinimumQty, offer.MembershipLabel, offer.CouponCondition, offer.ValidFrom, offer.ValidUntil, offer.Evidence, offer.Eligibility)
		if err != nil {
			return fmt.Errorf("storage: insert observed offer %q: %w", offer.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit observation: %w", err)
	}
	return nil
}

func validateObservation(observation Observation, offers []ObservedOffer) error {
	if strings.TrimSpace(observation.ID) == "" || strings.TrimSpace(observation.ListingID) == "" {
		return errors.New("storage: observation and listing IDs are required")
	}
	if observation.ObservedAt <= 0 || strings.TrimSpace(observation.SourceURL) == "" {
		return errors.New("storage: observation time and source URL are required")
	}
	if len(observation.ObservedTitle) > 240 || len(observation.ObservedBrand) > 120 || len(observation.ObservedProductLine) > 120 ||
		len(observation.ObservedVariant) > 160 || len(observation.ObservedGTIN) > 14 || len(observation.ObservedQuantityJSON) > 256 ||
		len(observation.FieldProvenanceJSON) > 4096 {
		return errors.New("storage: observation identity or provenance exceeds its limit")
	}
	if observation.FieldProvenanceJSON != "" && !json.Valid([]byte(observation.FieldProvenanceJSON)) {
		return errors.New("storage: field provenance must be valid JSON")
	}
	switch observation.FetchStatus {
	case "success", "blocked", "rate_limited", "timeout", "http_error", "network_error", "oversized", "parse_error", "resource_deferred":
	default:
		return fmt.Errorf("storage: invalid observation fetch status %q", observation.FetchStatus)
	}
	switch observation.ValidationState {
	case "valid", "invalid", "uncertain":
	default:
		return fmt.Errorf("storage: invalid observation validation state %q", observation.ValidationState)
	}
	if observation.FetchStatus != "success" && observation.ValidationState == "valid" {
		return errors.New("storage: failed fetch cannot have a valid observation")
	}
	if observation.HTTPStatus != nil && (*observation.HTTPStatus < 100 || *observation.HTTPStatus > 599) {
		return errors.New("storage: invalid HTTP status")
	}
	if observation.ObservedStock == "" {
		observation.ObservedStock = "unknown"
	}
	switch observation.ObservedStock {
	case "in_stock", "out_of_stock", "unknown":
	default:
		return errors.New("storage: invalid stock state")
	}
	if len(offers) > 0 && (observation.FetchStatus != "success" || observation.ValidationState != "valid") {
		return errors.New("storage: offers require a successful, valid observation")
	}
	seen := make(map[string]struct{}, len(offers))
	for _, offer := range offers {
		if strings.TrimSpace(offer.ID) == "" || strings.TrimSpace(offer.Evidence) == "" {
			return errors.New("storage: offer ID and evidence are required")
		}
		if _, ok := seen[offer.ID]; ok {
			return fmt.Errorf("storage: duplicate offer ID %q", offer.ID)
		}
		seen[offer.ID] = struct{}{}
		scale, supportedCurrency := pricing.MinorUnitScale(offer.Currency)
		if offer.AmountMinor <= 0 || !supportedCurrency || scale != offer.CurrencyScale || offer.Currency != strings.ToUpper(offer.Currency) || offer.MinimumQty <= 0 {
			return fmt.Errorf("storage: offer %q has invalid monetary terms", offer.ID)
		}
		if offer.Kind != "multibuy" && offer.MinimumQty != 1 {
			return fmt.Errorf("storage: offer %q quantity must be one unless multibuy", offer.ID)
		}
		if offer.Kind == "coupon" && offer.Eligibility == "public" {
			return fmt.Errorf("storage: coupon offer %q cannot be public", offer.ID)
		}
		if offer.ValidFrom != nil && *offer.ValidFrom < 0 || offer.ValidUntil != nil && *offer.ValidUntil < 0 {
			return fmt.Errorf("storage: offer %q has invalid validity timestamp", offer.ID)
		}
		if offer.ValidFrom != nil && offer.ValidUntil != nil && *offer.ValidUntil < *offer.ValidFrom {
			return fmt.Errorf("storage: offer %q validity window is inverted", offer.ID)
		}
	}
	return nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func defaultStock(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func defaultProvenance(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}
