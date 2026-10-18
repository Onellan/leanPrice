package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/fetch"
	"github.com/Onellan/leanPrice/internal/match"
	"github.com/Onellan/leanPrice/internal/pricing"
	"github.com/Onellan/leanPrice/internal/storage"
)

const refreshProduct = `{"@context":"https://schema.org","@type":"Product","name":"Domestos Lavender 750 ml","brand":{"@type":"Brand","name":"Domestos"},"productLine":"Domestos bleach","color":"Lavender","size":"750 ml","gtin13":"4006381333931","offers":{"@type":"Offer","price":"29.99","priceCurrency":"ZAR","availability":"https://schema.org/InStock"}}`

type memoryRepository struct {
	listing storage.ListingForRefresh
	err     error
	obs     []storage.Observation
	offers  [][]storage.ObservedOffer
	checked int64
	events  []string
}

func (m *memoryRepository) ListingForRefresh(context.Context, string) (storage.ListingForRefresh, error) {
	return m.listing, m.err
}
func (m *memoryRepository) PersistObservation(_ context.Context, observation storage.Observation, offers []storage.ObservedOffer) error {
	m.obs = append(m.obs, observation)
	m.offers = append(m.offers, offers)
	return nil
}
func (m *memoryRepository) MarkListingChecked(_ context.Context, _ string, at int64) error {
	m.checked = at
	return nil
}
func (m *memoryRepository) RecordSourceEvent(_ context.Context, _, _, code, _ string, _ time.Time) error {
	m.events = append(m.events, code)
	return nil
}

type memoryFetcher struct {
	result fetch.Result
	err    error
	calls  int
}

func (m *memoryFetcher) Fetch(_ context.Context, _ string) (fetch.Result, error) {
	m.calls++
	return m.result, m.err
}

func TestRefreshPersistsExactProductWithUnknownOfferTerms(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	body := []byte("<script type=\"application/ld+json\">" + refreshProduct + "</script>")
	digest := sha256.Sum256(body)
	repo := &memoryRepository{listing: storage.ListingForRefresh{
		ID: "listing1", SourceID: "source1", URL: "https://retailer.example/product?token=secret", MatchState: "exact",
		CanonicalProductID: "product1",
		CanonicalIdentity: match.Identity{Brand: "Domestos", Line: "Domestos bleach", Variant: "Lavender", VariantKnown: true,
			Quantity: &pricing.Quantity{Numerator: 3, Denominator: 4, Dimension: pricing.DimensionVolume, PackCount: 1}, GTIN: "4006381333931"},
	}}
	fetcher := &memoryFetcher{result: fetch.Result{StatusCode: 200, ContentType: "text/html", RetrievedAt: now,
		Digest: hex.EncodeToString(digest[:]), Body: body}}
	id := 0
	refresher := Refresher{Repository: repo, Fetcher: fetcher, Now: func() time.Time { return now }, NewID: func() string { id++; return "test-id-" + string(rune('0'+id)) }}
	result, err := refresher.Refresh(context.Background(), "listing1")
	if err != nil {
		t.Fatalf("Refresh(): %v", err)
	}
	if result.Match.Status != match.StatusExact || len(repo.obs) != 1 || len(repo.offers) != 1 || len(repo.offers[0]) != 1 {
		t.Fatalf("refresh did not persist exact lineage: result=%#v observations=%#v offers=%#v", result, repo.obs, repo.offers)
	}
	if repo.obs[0].ValidationState != "valid" || repo.obs[0].SourceURL != "https://retailer.example/product" || repo.obs[0].EvidenceDigest != hex.EncodeToString(digest[:]) {
		t.Fatalf("observation has unsafe or incomplete provenance: %#v", repo.obs[0])
	}
	if repo.obs[0].ObservedBrand != "Domestos" || repo.obs[0].ObservedProductLine != "Domestos bleach" || repo.obs[0].ObservedVariant != "Lavender" || !strings.Contains(repo.obs[0].FieldProvenanceJSON, "brand") {
		t.Fatalf("observation did not retain bounded field provenance: %#v", repo.obs[0])
	}
	offer := repo.offers[0][0]
	if offer.Currency != "ZAR" || offer.CurrencyScale != 2 || offer.AmountMinor != 2999 || offer.Kind != "unknown" || offer.Eligibility != "unknown" {
		t.Fatalf("generic offer was not stored fail-closed: %#v", offer)
	}
	if fetcher.calls != 1 || repo.checked != now.UnixMilli() {
		t.Fatalf("refresh calls=%d checked=%d", fetcher.calls, repo.checked)
	}
}

func TestRefreshRejectsVariantMismatchWithoutPersistingOffer(t *testing.T) {
	now := time.Now().UTC()
	body := []byte(`<script type="application/ld+json">` + stringsReplace(refreshProduct, "Lavender", "Original") + `</script>`)
	digest := sha256.Sum256(body)
	repo := validRepository()
	fetcher := &memoryFetcher{result: fetch.Result{StatusCode: 200, RetrievedAt: now, Digest: hex.EncodeToString(digest[:]), Body: body}}
	refresher := Refresher{Repository: repo, Fetcher: fetcher, Now: func() time.Time { return now }, NewID: func() string { return "id" }}
	result, err := refresher.Refresh(context.Background(), "listing1")
	if err != nil {
		t.Fatalf("Refresh(): %v", err)
	}
	if result.Match.Status != match.StatusRejected || len(repo.offers) != 1 || len(repo.offers[0]) != 0 || repo.obs[0].ValidationState != "invalid" {
		t.Fatalf("mismatched product produced a current offer: %#v %#v", result, repo.offers)
	}
}

func TestRefreshPersistsFailureAndRecordsClassifiedSourceEvent(t *testing.T) {
	now := time.Now().UTC()
	repo := validRepository()
	fetcher := &memoryFetcher{err: &fetch.FetchError{Kind: fetch.KindRateLimited, StatusCode: 429}}
	refresher := Refresher{Repository: repo, Fetcher: fetcher, Now: func() time.Time { return now }, NewID: func() string { return "id" }}
	result, err := refresher.Refresh(context.Background(), "listing1")
	if err == nil {
		t.Fatal("Refresh() did not return fetch failure")
	}
	if result.Observation.FetchStatus != "rate_limited" || result.Observation.FailureCode != "rate_limited" || len(repo.obs) != 1 || repo.checked != now.UnixMilli() || len(repo.events) != 1 {
		t.Fatalf("rate limit was not recorded: result=%#v repo=%#v", result, repo)
	}
}

func TestRefreshDoesNotFetchUnapprovedListing(t *testing.T) {
	repo := validRepository()
	repo.err = storage.ErrSourceUnavailable
	fetcher := &memoryFetcher{}
	refresher := Refresher{Repository: repo, Fetcher: fetcher}
	if _, err := refresher.Refresh(context.Background(), "listing1"); !errors.Is(err, storage.ErrSourceUnavailable) {
		t.Fatalf("Refresh() error = %v", err)
	}
	if fetcher.calls != 0 {
		t.Fatalf("unapproved source was contacted %d times", fetcher.calls)
	}
}

func validRepository() *memoryRepository {
	return &memoryRepository{listing: storage.ListingForRefresh{
		ID: "listing1", SourceID: "source1", URL: "https://retailer.example/product", MatchState: "exact", CanonicalProductID: "product1",
		CanonicalIdentity: match.Identity{Brand: "Domestos", Line: "Domestos bleach", Variant: "Lavender", VariantKnown: true,
			Quantity: &pricing.Quantity{Numerator: 3, Denominator: 4, Dimension: pricing.DimensionVolume, PackCount: 1}, GTIN: "4006381333931"},
	}}
}

func stringsReplace(value, old, new string) string {
	return strings.ReplaceAll(value, old, new)
}
