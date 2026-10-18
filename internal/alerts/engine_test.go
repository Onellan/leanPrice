package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
	"github.com/Onellan/leanPrice/internal/storage"
)

type fakeRepository struct {
	rows        []storage.WatchEvaluation
	alert       storage.AlertDelivery
	created     bool
	createCalls int
	recovered   int
}

func (r *fakeRepository) ListWatchEvaluations(context.Context, time.Time, time.Duration, int) ([]storage.WatchEvaluation, error) {
	return r.rows, nil
}
func (r *fakeRepository) CreateAlertEpisode(_ context.Context, alert storage.AlertDelivery) (string, bool, error) {
	r.alert, r.createCalls = alert, r.createCalls+1
	return alert.EpisodeID, r.created, nil
}
func (r *fakeRepository) CloseRecoveredAlertEpisode(context.Context, storage.AlertRecovery) (bool, error) {
	r.recovered++
	return true, nil
}

func TestEngineCreatesDurableInAppEpisodeFromFreshExactEvidence(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	target := int64(2000)
	repo := &fakeRepository{created: true, rows: []storage.WatchEvaluation{{
		WatchID: "watch-1", ProductID: "product-1", ListingID: "listing-1", OfferID: "offer-1",
		Rule: storage.WatchRule{Version: 2, TargetMinor: &target, ConditionOperator: "any"},
		Current: pricing.Offer{ListingID: "listing-1", Money: pricing.Money{Minor: 1900, Currency: "ZAR", Scale: 2}, Type: pricing.OfferStandard,
			Eligibility: pricing.EligibilityPublic, MinimumQty: 1, TermsKnown: true, Stock: pricing.StockUnknown,
			ObservedAt: now.Add(-time.Minute), SourceApproved: true, ExactMatch: true, ObservationValid: true},
	}}}
	engine := Engine{Repository: repo, Now: func() time.Time { return now }}
	summary, err := engine.Evaluate(context.Background())
	if err != nil || summary.Evaluated != 1 || summary.Created != 1 || repo.createCalls != 1 {
		t.Fatalf("Evaluate() = %+v, %v; calls=%d", summary, err, repo.createCalls)
	}
	if repo.alert.DestinationKey != "in_app" || repo.alert.ReasonCode != "threshold_met" || repo.alert.RuleVersion != 2 || repo.alert.OfferID != "offer-1" {
		t.Fatalf("alert episode lost rule/evidence lineage: %+v", repo.alert)
	}
}

func TestEngineUsesWatchNotificationPreferenceOverInstanceFallback(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	target := int64(2000)
	repo := &fakeRepository{created: true, rows: []storage.WatchEvaluation{{
		WatchID: "watch-webhook", ProductID: "product-1", ListingID: "listing-1", OfferID: "offer-1",
		Rule: storage.WatchRule{Version: 1, TargetMinor: &target, ConditionOperator: "any", NotificationChannel: "webhook"},
		Current: pricing.Offer{ListingID: "listing-1", Money: pricing.Money{Minor: 1900, Currency: "ZAR", Scale: 2}, Type: pricing.OfferStandard,
			Eligibility: pricing.EligibilityPublic, MinimumQty: 1, TermsKnown: true, Stock: pricing.StockInStock,
			ObservedAt: now.Add(-time.Minute), SourceApproved: true, ExactMatch: true, ObservationValid: true},
	}}}
	engine := Engine{Repository: repo, Now: func() time.Time { return now }, DestinationKey: "in_app"}
	if _, err := engine.Evaluate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.alert.DestinationKey != "webhook" {
		t.Fatalf("alert destination = %q, want watch preference webhook", repo.alert.DestinationKey)
	}
}

func TestEngineDoesNotCloseEpisodeWithoutComparableRecoveryEvidence(t *testing.T) {
	now := time.Now().UTC()
	target, drop := int64(5000), 1000
	repo := &fakeRepository{rows: []storage.WatchEvaluation{{
		WatchID: "watch-1", ListingID: "listing-1", OfferID: "offer-1",
		Rule: storage.WatchRule{Version: 1, TargetMinor: &target, DropBasisPoints: &drop, ConditionOperator: "any"},
		Current: pricing.Offer{ListingID: "listing-1", Money: pricing.Money{Minor: 6000, Currency: "ZAR", Scale: 2}, Type: pricing.OfferStandard,
			Eligibility: pricing.EligibilityPublic, MinimumQty: 1, TermsKnown: true, Stock: pricing.StockInStock,
			ObservedAt: now.Add(-time.Minute), SourceApproved: true, ExactMatch: true, ObservationValid: true},
	}}}
	engine := Engine{Repository: repo, Now: func() time.Time { return now }}
	if _, err := engine.Evaluate(context.Background()); err != nil || repo.recovered != 0 {
		t.Fatalf("Evaluate() error=%v, recovery calls=%d; missing baseline must not close", err, repo.recovered)
	}
}
