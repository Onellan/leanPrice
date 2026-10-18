// Package catalog coordinates one bounded listing refresh without holding a
// database transaction during network access or parsing.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Onellan/leanPrice/internal/extract"
	"github.com/Onellan/leanPrice/internal/fetch"
	"github.com/Onellan/leanPrice/internal/match"
	"github.com/Onellan/leanPrice/internal/storage"
)

type Fetcher interface {
	Fetch(context.Context, string) (fetch.Result, error)
}

type Repository interface {
	ListingForRefresh(context.Context, string) (storage.ListingForRefresh, error)
	PersistObservation(context.Context, storage.Observation, []storage.ObservedOffer) error
	MarkListingChecked(context.Context, string, int64) error
	RecordSourceEvent(context.Context, string, string, string, string, time.Time) error
}

type Refresher struct {
	Repository Repository
	Fetcher    Fetcher
	Now        func() time.Time
	NewID      func() string
}

type RefreshResult struct {
	Observation storage.Observation
	Match       match.Decision
	Extraction  extract.Extraction
}

func (r Refresher) Refresh(ctx context.Context, listingID string) (RefreshResult, error) {
	if r.Repository == nil || r.Fetcher == nil || strings.TrimSpace(listingID) == "" {
		return RefreshResult{}, errors.New("catalog: refresher dependencies and listing ID are required")
	}
	now := r.now()
	newID := r.id
	listing, err := r.Repository.ListingForRefresh(ctx, listingID)
	if err != nil {
		return RefreshResult{}, err
	}
	response, fetchErr := r.Fetcher.Fetch(ctx, listing.URL)
	if fetchErr != nil {
		observation := storage.Observation{
			ID: newID(), ListingID: listing.ID, ObservedAt: response.RetrievedAt.UTC().UnixMilli(),
			FetchStatus: fetchStatus(fetchErr), HTTPStatus: statusPointer(response.StatusCode),
			ObservedStock: "unknown", FailureCode: fetchFailureCode(fetchErr), SourceURL: extract.SanitizeURL(listing.URL),
			ValidationState: "uncertain",
		}
		if observation.ObservedAt <= 0 {
			observation.ObservedAt = now.UnixMilli()
		}
		if err := r.persist(ctx, observation, nil); err != nil {
			return RefreshResult{}, err
		}
		if err := r.Repository.RecordSourceEvent(ctx, newID(), listing.SourceID, observation.FailureCode, "", now); err != nil {
			return RefreshResult{Observation: observation}, fmt.Errorf("catalog: record source failure: %w", err)
		}
		return RefreshResult{Observation: observation}, fetchErr
	}
	if response.RetrievedAt.IsZero() {
		response.RetrievedAt = now
	}
	parsed, parseErr := extract.Parse(response.Body, extract.Input{
		SourceURL: listing.URL, ObservedAt: response.RetrievedAt, Parser: extract.ParserVersion, BodyDigest: response.Digest,
	})
	observation := storage.Observation{
		ID: newID(), ListingID: listing.ID, ObservedAt: response.RetrievedAt.UTC().UnixMilli(),
		FetchStatus: "success", HTTPStatus: statusPointer(response.StatusCode), ParserVersion: extract.ParserVersion,
		EvidenceDigest: response.Digest, ObservedStock: "unknown", SourceURL: extract.SanitizeURL(listing.URL),
		ValidationState: "invalid",
	}
	result := RefreshResult{Observation: observation, Extraction: parsed}
	if parseErr != nil {
		result.Observation.FailureCode = "parse_error"
		result.Observation.ValidationState = "invalid"
		if err := r.persist(ctx, result.Observation, nil); err != nil {
			return result, err
		}
		return result, parseErr
	}
	result.Observation.EvidenceDigest = parsed.EvidenceDigest
	result.Observation.SourceURL = parsed.SourceURL
	result.Observation.ObservedTitle = parsed.Title
	result.Observation.ObservedBrand = parsed.Brand
	result.Observation.ObservedProductLine = parsed.Line
	result.Observation.ObservedVariant = parsed.Variant
	result.Observation.ObservedGTIN = parsed.GTIN
	provenanceJSON, provenanceErr := json.Marshal(parsed.FieldOrigins)
	if provenanceErr == nil && len(provenanceJSON) <= 4096 {
		result.Observation.FieldProvenanceJSON = string(provenanceJSON)
	}
	result.Observation.ObservedStock = string(parsed.Stock)
	if parsed.Quantity != nil {
		quantityJSON, marshalErr := json.Marshal(parsed.Quantity)
		if marshalErr != nil {
			return result, fmt.Errorf("catalog: encode observed quantity: %w", marshalErr)
		}
		if len(quantityJSON) <= 256 {
			result.Observation.ObservedQuantityJSON = string(quantityJSON)
		}
	}
	if parsed.Status == extract.StatusConflict {
		result.Observation.ValidationState = "uncertain"
		result.Observation.FailureCode = "extraction_conflict"
		if err := r.persist(ctx, result.Observation, nil); err != nil {
			return result, err
		}
		return result, nil
	}
	if parsed.Status != extract.StatusParsed {
		result.Observation.FailureCode = "product_not_found"
		if err := r.persist(ctx, result.Observation, nil); err != nil {
			return result, err
		}
		return result, nil
	}
	identity := match.Identity{Brand: parsed.Brand, Line: parsed.Line,
		Variant: parsed.Variant, VariantKnown: parsed.Variant != "", Quantity: parsed.Quantity, GTIN: parsed.GTIN}
	decision := match.Evaluate(listing.CanonicalIdentity, identity)
	result.Match = decision
	if decision.Status != match.StatusExact {
		result.Observation.ValidationState = "uncertain"
		result.Observation.FailureCode = decision.Reason
		if decision.Status == match.StatusRejected {
			result.Observation.ValidationState = "invalid"
		}
		if err := r.persist(ctx, result.Observation, nil); err != nil {
			return result, err
		}
		return result, nil
	}
	offers := make([]storage.ObservedOffer, 0, len(parsed.Offers))
	for _, extractedOffer := range parsed.Offers {
		if !extractedOffer.Valid {
			continue
		}
		eligibility := string(extractedOffer.Offer.Eligibility)
		if eligibility == "member" {
			eligibility = "conditional"
		}
		offers = append(offers, storage.ObservedOffer{
			ID: newID(), Kind: string(extractedOffer.Offer.Type), Currency: extractedOffer.Offer.Money.Currency,
			CurrencyScale: extractedOffer.Offer.Money.Scale, AmountMinor: extractedOffer.Offer.Money.Minor,
			MinimumQty: int(extractedOffer.Offer.MinimumQty), Evidence: extractedOffer.Origin.Path, Eligibility: eligibility,
		})
	}
	if len(offers) == 0 {
		result.Observation.FailureCode = "missing_valid_offer"
		if err := r.persist(ctx, result.Observation, nil); err != nil {
			return result, err
		}
		return result, nil
	}
	result.Observation.ValidationState = "valid"
	if err := r.persist(ctx, result.Observation, offers); err != nil {
		return result, err
	}
	return result, nil
}

func (r Refresher) persist(ctx context.Context, observation storage.Observation, offers []storage.ObservedOffer) error {
	if err := r.Repository.PersistObservation(ctx, observation, offers); err != nil {
		return fmt.Errorf("catalog: persist observation: %w", err)
	}
	if err := r.Repository.MarkListingChecked(ctx, observation.ListingID, observation.ObservedAt); err != nil {
		return fmt.Errorf("catalog: update last checked time: %w", err)
	}
	return nil
}

func (r Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r Refresher) id() string {
	if r.NewID != nil {
		return r.NewID()
	}
	return uuid.NewString()
}

func statusPointer(code int) *int {
	if code < 100 || code > 599 {
		return nil
	}
	return &code
}

func fetchStatus(err error) string {
	var fetchErr *fetch.FetchError
	if !errors.As(err, &fetchErr) {
		return "network_error"
	}
	switch fetchErr.Kind {
	case fetch.KindBlocked, fetch.KindPolicy:
		return "blocked"
	case fetch.KindRateLimited:
		return "rate_limited"
	case fetch.KindTimeout:
		return "timeout"
	case fetch.KindOversized:
		return "oversized"
	case fetch.KindInvalid:
		return "parse_error"
	case fetch.KindUnavailable:
		if fetchErr.StatusCode != 0 {
			return "http_error"
		}
		return "network_error"
	default:
		return "network_error"
	}
}

func fetchFailureCode(err error) string {
	var fetchErr *fetch.FetchError
	if errors.As(err, &fetchErr) {
		return string(fetchErr.Kind)
	}
	return "network_error"
}
