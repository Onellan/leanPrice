package integration

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/alerts"
	"github.com/Onellan/leanPrice/internal/storage"
)

func TestAlertJourneySurvivesRestartDedupesAndRearms(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leanprice.sqlite")
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	store := openJourneyStore(t, path)
	seedJourneyCatalog(t, store, now)

	for _, listing := range []struct {
		id     string
		amount int64
	}{{"listing-a", 3200}, {"listing-b", 3200}} {
		persistJourneyOffer(t, store, "baseline-"+listing.id, listing.id, now, listing.amount)
	}
	target := int64(3000)
	if err := store.CreateWatch(ctx, "watch-journey", "product-journey", "", true,
		storage.WatchRule{TargetMinor: &target, ConditionOperator: "any", NotificationChannel: "webhook"}, now); err != nil {
		t.Fatal(err)
	}

	firstDrop := now.Add(time.Minute)
	persistJourneyOffer(t, store, "drop-a", "listing-a", firstDrop, 3200)
	persistJourneyOffer(t, store, "drop-b", "listing-b", firstDrop, 2999)
	engine := alerts.Engine{Repository: store, Now: func() time.Time { return firstDrop }}
	summary, err := engine.Evaluate(ctx)
	if err != nil || summary.Evaluated != 2 || summary.Created != 1 {
		t.Fatalf("first evaluation = %+v, %v; want two listings and one episode", summary, err)
	}
	finishOneDelivery(t, ctx, store, firstDrop)
	if episodes, outbox := journeyCounts(t, store); episodes != 1 || outbox != 1 {
		t.Fatalf("after first alert: episodes=%d outbox=%d, want one each", episodes, outbox)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openJourneyStore(t, path)
	engine.Repository = store
	for i := 0; i < 10; i++ {
		recheckAt := firstDrop.Add(time.Duration(i+1) * time.Minute)
		engine.Now = func() time.Time { return recheckAt }
		summary, err = engine.Evaluate(ctx)
		if err != nil || summary.Created != 0 {
			t.Fatalf("unchanged recheck %d = %+v, %v; want no duplicate episode", i+1, summary, err)
		}
	}
	if episodes, outbox := journeyCounts(t, store); episodes != 1 || outbox != 1 {
		t.Fatalf("after restart/rechecks: episodes=%d outbox=%d, want one each", episodes, outbox)
	}

	recoveryAt := firstDrop.Add(11 * time.Minute)
	persistJourneyOffer(t, store, "recovery-b", "listing-b", recoveryAt, 3500)
	engine.Now = func() time.Time { return recoveryAt }
	summary, err = engine.Evaluate(ctx)
	if err != nil || summary.Recovered != 1 || summary.Created != 0 {
		t.Fatalf("recovery evaluation = %+v, %v; want one recovered episode", summary, err)
	}

	secondDrop := recoveryAt.Add(time.Minute)
	persistJourneyOffer(t, store, "second-drop-b", "listing-b", secondDrop, 2700)
	engine.Now = func() time.Time { return secondDrop }
	summary, err = engine.Evaluate(ctx)
	if err != nil || summary.Created != 1 {
		t.Fatalf("second drop evaluation = %+v, %v; want one re-armed episode", summary, err)
	}
	finishOneDelivery(t, ctx, store, secondDrop)
	if episodes, outbox := journeyCounts(t, store); episodes != 2 || outbox != 2 {
		t.Fatalf("after re-arm: episodes=%d outbox=%d, want two each", episodes, outbox)
	}
	var episodeNumbers []int
	db := journeyDB(t, store)
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT episode_number FROM alert_episodes ORDER BY episode_number")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var number int
		if err := rows.Scan(&number); err != nil {
			t.Fatal(err)
		}
		episodeNumbers = append(episodeNumbers, number)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(episodeNumbers) != 2 || episodeNumbers[0] != 1 || episodeNumbers[1] != 2 {
		t.Fatalf("episode sequence = %v, want [1 2]", episodeNumbers)
	}
	var delivered int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM delivery_outbox WHERE status='delivered'").Scan(&delivered); err != nil || delivered != 2 {
		t.Fatalf("delivered notification count = %d, %v; want two", delivered, err)
	}
}

func TestRestartDoesNotTreatOldCachedPriceAsFreshAlertEvidence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leanprice.sqlite")
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	store, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	seedJourneyCatalog(t, store, now)
	target := int64(3000)
	if err := store.CreateWatch(ctx, "watch-stale", "product-journey", "", true,
		storage.WatchRule{TargetMinor: &target, ConditionOperator: "any", NotificationChannel: "webhook"}, now); err != nil {
		t.Fatal(err)
	}
	persistJourneyOffer(t, store, "old-cached-offer", "listing-a", now.Add(-25*time.Hour), 2000)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	engine := alerts.Engine{Repository: store, Now: func() time.Time { return now }}
	summary, err := engine.Evaluate(ctx)
	if err != nil || summary.Evaluated != 1 || summary.Created != 0 {
		t.Fatalf("post-restart evaluation = %+v, %v; stale cached offer must not alert", summary, err)
	}
	if episodes, outbox := journeyCounts(t, store); episodes != 0 || outbox != 0 {
		t.Fatalf("post-restart stale offer produced episodes=%d outbox=%d; want none", episodes, outbox)
	}
}

func openJourneyStore(t *testing.T, path string) *storage.Store {
	t.Helper()
	store, err := storage.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func journeyDB(t *testing.T, store *storage.Store) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func seedJourneyCatalog(t *testing.T, store *storage.Store, now time.Time) {
	t.Helper()
	db := journeyDB(t, store)
	defer db.Close()
	ctx := context.Background()
	for _, source := range []struct {
		id       string
		retailer string
		host     string
	}{{"source-a", "retailer-a", "shop-a.example"}, {"source-b", "retailer-b", "shop-b.example"}} {
		if _, err := db.ExecContext(ctx, "INSERT INTO retailers(id,name) VALUES(?,?)", source.retailer, source.retailer); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO sources(id,retailer_id,source_key,base_host,assessment_state,assessment_ref,assessment_at,enabled,created_at,updated_at) VALUES(?,?,?,?,'approved','synthetic-fixture',?,1,?,?)",
			source.id, source.retailer, source.id, source.host, now.UnixMilli(), now.UnixMilli(), now.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO canonical_products(id,display_name,quantity_numerator,quantity_denominator,quantity_dimension,quantity_unit,created_at,updated_at) VALUES('product-journey','Synthetic household product',1,1,'count','item',?,?)",
		now.UnixMilli(), now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	for _, listing := range []struct {
		id     string
		source string
		host   string
	}{{"listing-a", "source-a", "shop-a.example"}, {"listing-b", "source-b", "shop-b.example"}} {
		url := "https://" + listing.host + "/item/1"
		if _, err := db.ExecContext(ctx, "INSERT INTO listings(id,source_id,canonical_product_id,url,url_key,match_state,first_seen_at,created_at) VALUES(?,?, 'product-journey',?,?,'exact',?,?)",
			listing.id, listing.source, url, url, now.UnixMilli(), now.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
}

func persistJourneyOffer(t *testing.T, store *storage.Store, observationID, listingID string, observedAt time.Time, amount int64) {
	t.Helper()
	host := "shop-a.example"
	if listingID == "listing-b" {
		host = "shop-b.example"
	}
	err := store.PersistObservation(context.Background(), storage.Observation{
		ID: observationID, ListingID: listingID, ObservedAt: observedAt.UnixMilli(),
		FetchStatus: "success", ObservedStock: "in_stock", SourceURL: "https://" + host + "/item/1", ValidationState: "valid",
	}, []storage.ObservedOffer{{ID: "offer-" + observationID, Kind: "standard", Currency: "ZAR", CurrencyScale: 2,
		AmountMinor: amount, MinimumQty: 1, Evidence: "fixture:price", Eligibility: "public"}})
	if err != nil {
		t.Fatal(err)
	}
}

func finishOneDelivery(t *testing.T, ctx context.Context, store *storage.Store, now time.Time) {
	t.Helper()
	delivery, err := store.ClaimDelivery(ctx, "journey-worker", now, time.Minute)
	if err != nil || delivery.Status != "leased" || delivery.AttemptCount != 1 {
		t.Fatalf("claim delivery = %+v, %v", delivery, err)
	}
	if err := store.FinishDelivery(ctx, delivery.ID, "journey-worker", storage.DeliveryResult{Status: "delivered", HTTPStatus: 204}, now); err != nil {
		t.Fatal(err)
	}
}

func journeyCounts(t *testing.T, store *storage.Store) (int, int) {
	t.Helper()
	db := journeyDB(t, store)
	defer db.Close()
	var episodes, outbox int
	if err := db.QueryRow("SELECT count(*) FROM alert_episodes").Scan(&episodes); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM delivery_outbox").Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	return episodes, outbox
}
