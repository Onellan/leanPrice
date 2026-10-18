package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestListProductPageUsesStableKeysetAndRejectsInvalidCursor(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	for i := 0; i < 3; i++ {
		name := []string{"Alpha", "Bravo", "Charlie"}[i]
		if err := store.CreateCanonicalProduct(ctx, ProductSummary{ID: fmt.Sprintf("product-%d", i), Name: name, Quantity: "1/1", QuantityUnit: "item", PackCount: 1}, time.Unix(int64(i+1), 0)); err != nil {
			t.Fatal(err)
		}
	}
	first, more, err := store.ListProductPage(ctx, "", "", "", 2)
	if err != nil || !more || len(first) != 2 || first[0].Name != "Alpha" || first[1].Name != "Bravo" {
		t.Fatalf("first page = %+v, more=%v, err=%v", first, more, err)
	}
	second, more, err := store.ListProductPage(ctx, "", first[1].SortName, first[1].ID, 2)
	if err != nil || more || len(second) != 1 || second[0].Name != "Charlie" {
		t.Fatalf("second page = %+v, more=%v, err=%v", second, more, err)
	}
	if _, _, err := store.ListProductPage(ctx, "", "alpha", "", 2); err == nil {
		t.Fatal("incomplete cursor was accepted")
	}
	matched, more, err := store.ListProductPage(ctx, "brav", "", "", 2)
	if err != nil || more || len(matched) != 1 || matched[0].Name != "Bravo" {
		t.Fatalf("search page = %+v, more=%v, err=%v", matched, more, err)
	}
}

func TestListProductHistoryPageDoesNotRepeatEqualTimestampRows(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-history-page", "product-history-page", "listing-history-page")
	observedAt := time.Now().UTC().UnixMilli()
	for _, item := range []struct{ observation, offer string }{{"obs-a", "offer-a"}, {"obs-b", "offer-b"}} {
		observation := Observation{ID: item.observation, ListingID: "listing-history-page", ObservedAt: observedAt,
			FetchStatus: "success", ObservedStock: "in_stock", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
		offer := ObservedOffer{ID: item.offer, Kind: "standard", Currency: "ZAR", CurrencyScale: 2,
			AmountMinor: 1200, MinimumQty: 1, Evidence: "fixture:price", Eligibility: "public"}
		if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
			t.Fatal(err)
		}
	}
	first, more, err := store.ListProductHistoryPage(ctx, "product-history-page", nil, 1)
	if err != nil || !more || len(first) != 1 {
		t.Fatalf("first history page = %+v, more=%v, err=%v", first, more, err)
	}
	cursor := HistoryCursor{ObservedAt: first[0].ObservedAt, ObservationID: first[0].ObservationID, OfferID: first[0].OfferID}
	second, more, err := store.ListProductHistoryPage(ctx, "product-history-page", &cursor, 1)
	if err != nil || more || len(second) != 1 || second[0].ObservationID == first[0].ObservationID {
		t.Fatalf("second history page = %+v, more=%v, err=%v", second, more, err)
	}
	if _, _, err := store.ListProductHistoryPage(ctx, "product-history-page", &HistoryCursor{ObservedAt: observedAt}, 1); err == nil {
		t.Fatal("incomplete history cursor was accepted")
	}
}

func TestListActivityPageUsesStableKeysetAtEqualTimes(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source-activity-page")
	at := time.Now().UTC()
	for _, id := range []string{"event-a", "event-b"} {
		if err := store.RecordSourceEvent(ctx, id, "source-activity-page", "check_ok", "fixture", at); err != nil {
			t.Fatal(err)
		}
	}
	first, more, err := store.ListActivityPage(ctx, nil, 1)
	if err != nil || !more || len(first) != 1 {
		t.Fatalf("first activity page = %+v, more=%v, err=%v", first, more, err)
	}
	cursor := ActivityCursor{At: first[0].At, EventKey: first[0].EventKey}
	second, more, err := store.ListActivityPage(ctx, &cursor, 1)
	if err != nil || more || len(second) != 1 || second[0].EventKey == first[0].EventKey {
		t.Fatalf("second activity page = %+v, more=%v, err=%v", second, more, err)
	}
	if _, _, err := store.ListActivityPage(ctx, &ActivityCursor{At: at.UnixMilli()}, 1); err == nil {
		t.Fatal("incomplete activity cursor was accepted")
	}
}

func TestSetSourceEnabledRequiresApprovalAndAuditsOperator(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source-toggle")
	if _, err := store.db.ExecContext(ctx, `INSERT INTO admin_users(id,username,password_hash,created_at,updated_at)
		VALUES ('admin-toggle','owner','this-is-a-long-enough-test-password-hash',1,1)`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSourceEnabled(ctx, "admin-toggle", "source-clicks", "event-rejected-enable", "audit-rejected-enable", true, time.Now()); err != ErrSourceNotApproved {
		t.Fatalf("enable unapproved source error = %v, want %v", err, ErrSourceNotApproved)
	}
	at := time.Now().UTC()
	if err := store.SetSourceEnabled(ctx, "admin-toggle", "source-toggle", "event-pause", "audit-pause", false, at); err != nil {
		t.Fatalf("pause approved source: %v", err)
	}
	if err := store.SetSourceEnabled(ctx, "admin-toggle", "source-toggle", "event-enable", "audit-enable", true, at.Add(time.Second)); err != nil {
		t.Fatalf("enable approved source: %v", err)
	}
	var enabled, events, audits int
	if err := store.db.QueryRowContext(ctx, "SELECT enabled FROM sources WHERE id='source-toggle'").Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM source_events WHERE source_id='source-toggle'").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM admin_audit WHERE object_id='source-toggle'").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 || events != 2 || audits != 2 {
		t.Fatalf("source enabled=%d, events=%d, audits=%d; want 1,2,2", enabled, events, audits)
	}
}
