package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestOpenCreatesVersionedWALSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leanprice.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer store.Close()

	var foreignKeys, version int
	var journalMode string
	if err := store.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || !strings.EqualFold(journalMode, "wal") || version != 4 {
		t.Fatalf("foreign_keys=%d journal_mode=%q user_version=%d", foreignKeys, journalMode, version)
	}
	if got := store.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}
	var checksum string
	if err := store.db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = 1").Scan(&checksum); err != nil {
		t.Fatalf("read migration checksum: %v", err)
	}
	if len(checksum) != 64 {
		t.Fatalf("migration checksum length = %d, want 64", len(checksum))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database mode = %04o, want 0600", got)
	}
}

func TestOpenHoldsExclusiveProcessLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leanprice.db")
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), path); err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("second Open() error = %v, want active-process lock", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() after close: %v", err)
	}
	_ = second.Close()
}

func TestMigrationIsIdempotentAndChecksumProtected(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leanprice.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate(): %v", err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE schema_migrations SET checksum = ? WHERE version = 1", strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = Open(ctx, path)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Open() error = %v, want checksum mismatch", err)
	}
}

func TestFutureSchemaVersionIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = Open(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("Open() error = %v, want unsupported future schema", err)
	}
}

func TestSchemaRejectsUnapprovedSourceAndInvalidPrices(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "leanprice.db"))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer store.Close()
	if _, err := store.db.ExecContext(ctx, `INSERT INTO retailers(id, name) VALUES ('r1', 'Example')`); err != nil {
		t.Fatal(err)
	}
	_, err = store.db.ExecContext(ctx, `INSERT INTO sources(id, retailer_id, source_key, base_host, assessment_state, enabled)
		VALUES ('s1', 'r1', 'example', 'shop.example', 'unassessed', 1)`)
	if err == nil {
		t.Fatal("enabled unassessed source insert succeeded")
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO canonical_products
		(id, display_name, quantity_numerator, quantity_denominator, quantity_dimension, quantity_unit, created_at, updated_at)
		VALUES ('p1', 'Example item', 0, 1, 'volume', 'L', 1, 1)`); err == nil {
		t.Fatal("zero-quantity product insert succeeded")
	}
}

func TestSourceHostApprovalIsExactAndFailClosed(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if approved, err := store.HostApproved(ctx, "shop.example"); err != nil || approved {
		t.Fatalf("unconfigured source approval = %v, %v", approved, err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO retailers(id, name) VALUES ('retailer-example', 'Example')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO sources(id, retailer_id, source_key, base_host, assessment_state, assessment_ref, assessment_at, enabled, created_at, updated_at)
		VALUES ('source-disabled', 'retailer-example', 'disabled', 'shop.example', 'approved', 'review', 1, 0, 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if approved, err := store.HostApproved(ctx, "shop.example"); err != nil || approved {
		t.Fatalf("disabled source approval = %v, %v", approved, err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE sources SET enabled=1 WHERE id='source-disabled'`); err != nil {
		t.Fatal(err)
	}
	if approved, err := store.HostApproved(ctx, "SHOP.EXAMPLE."); err != nil || !approved {
		t.Fatalf("normalized exact host approval = %v, %v", approved, err)
	}
	if approved, err := store.HostApproved(ctx, "www.shop.example"); err != nil || approved {
		t.Fatalf("implicit subdomain approval = %v, %v", approved, err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE sources SET assessment_state='review_required' WHERE id='source-disabled'`); err == nil {
		t.Fatal("enabled source was changed to review_required")
	}
}

func TestSourceRateReservationsAndBackoffSurviveRestarts(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source1")
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	wait, err := store.ReserveHost(ctx, "shop.example", now, 10*time.Second)
	if err != nil || wait != 0 {
		t.Fatalf("first ReserveHost() = %v, %v", wait, err)
	}
	wait, err = store.ReserveHost(ctx, "SHOP.EXAMPLE.", now.Add(time.Second), 10*time.Second)
	if err != nil || wait != 9*time.Second {
		t.Fatalf("second ReserveHost() = %v, %v", wait, err)
	}
	backoff := now.Add(time.Hour)
	if err := store.DeferHost(ctx, "shop.example", backoff); err != nil {
		t.Fatalf("DeferHost(): %v", err)
	}
	wait, err = store.ReserveHost(ctx, "shop.example", now.Add(2*time.Second), 10*time.Second)
	if err != nil || wait != backoff.Sub(now.Add(2*time.Second)) {
		t.Fatalf("reservation did not honor durable backoff: %v, %v", wait, err)
	}
	if _, err := store.ReserveHost(ctx, "sub.shop.example", now, 10*time.Second); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("unapproved subdomain reservation error = %v", err)
	}
}

func TestListingRefreshRequiresApprovedSourceAndCanonicalIdentity(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source1", "product1", "listing1")
	listing, err := store.ListingForRefresh(ctx, "listing1")
	if err != nil {
		t.Fatalf("ListingForRefresh(): %v", err)
	}
	if listing.CanonicalProductID != "product1" || listing.CanonicalIdentity.Quantity == nil || listing.CanonicalIdentity.Quantity.Dimension != "count" || listing.URL != "https://shop.example/item/1" {
		t.Fatalf("refresh lineage = %#v", listing)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE sources SET enabled=0 WHERE id='source1'"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListingForRefresh(ctx, "listing1"); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("disabled source refresh error = %v", err)
	}
	if _, err := store.ListingForRefresh(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing listing error = %v", err)
	}
}

func TestRecordSourceEventRejectsUnredactedFreeformCodes(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source1")
	if err := store.RecordSourceEvent(ctx, "event1", "source1", "rate_limited", "", time.Now().UTC()); err != nil {
		t.Fatalf("RecordSourceEvent(): %v", err)
	}
	if err := store.RecordSourceEvent(ctx, "event2", "source1", "https://retailer.example/?token=secret", "", time.Now().UTC()); err == nil {
		t.Fatal("freeform source event code accepted")
	}
}

func TestMigrateReturnsContextError(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "leanprice.db"))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer store.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = store.Migrate(ctx)
	if err == nil || (!errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context")) {
		t.Fatalf("Migrate() error = %v, want canceled context", err)
	}
}

func TestJobQueueDeduplicatesAndChecksSourceAtClaim(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source1")
	now := time.Now().UTC().Truncate(time.Millisecond)
	job := Job{ID: "job1", Kind: "source_probe", SourceID: "source1", DedupeKey: "probe:source1:1", DueAt: now.UnixMilli()}
	id, created, err := store.EnqueueJob(ctx, job)
	if err != nil || !created || id != job.ID {
		t.Fatalf("EnqueueJob() = %q, %v, %v", id, created, err)
	}
	id2, created, err := store.EnqueueJob(ctx, Job{ID: "job2", Kind: job.Kind, SourceID: job.SourceID, DedupeKey: job.DedupeKey, DueAt: job.DueAt})
	if err != nil || created || id2 != job.ID {
		t.Fatalf("duplicate EnqueueJob() = %q, %v, %v", id2, created, err)
	}
	claimed, err := store.ClaimJob(ctx, "worker-a", now, time.Minute)
	if err != nil || claimed.ID != job.ID || claimed.AttemptCount != 1 || claimed.Status != "leased" {
		t.Fatalf("ClaimJob() = %+v, %v", claimed, err)
	}
	if err := store.FinishJob(ctx, job.ID, "worker-b", JobResult{Status: "completed"}, now); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("FinishJob wrong owner error = %v, want ErrLeaseLost", err)
	}
	if err := store.FinishJob(ctx, job.ID, "worker-a", JobResult{Status: "retry_at", NextAttemptAt: now.Add(time.Minute).UnixMilli(), LastErrorCode: "timeout"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJob(ctx, "worker-b", now.Add(30*time.Second), time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("early ClaimJob() error = %v, want ErrNotFound", err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE sources SET enabled = 0 WHERE id = 'source1'"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJob(ctx, "worker-b", now.Add(2*time.Minute), time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim on paused source error = %v, want ErrNotFound", err)
	}
}

func TestDailySchedulingOnlyEnqueuesApprovedWatchedListings(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-schedule", "product-schedule", "listing-schedule")
	target := int64(1000)
	if err := store.CreateWatch(ctx, "watch-schedule", "product-schedule", "", true, WatchRule{TargetMinor: &target}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	created, err := store.ScheduleRefreshes(ctx, now, 24*time.Hour, 100)
	if err != nil || created != 1 {
		t.Fatalf("ScheduleRefreshes() = %d, %v; want one approved listing", created, err)
	}
	created, err = store.ScheduleRefreshes(ctx, now, 24*time.Hour, 100)
	if err != nil || created != 0 {
		t.Fatalf("same-day scheduling = %d, %v; want deduplicated", created, err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE sources SET enabled=0 WHERE id='source-schedule'"); err != nil {
		t.Fatal(err)
	}
	created, err = store.ScheduleRefreshes(ctx, now.Add(25*time.Hour), 24*time.Hour, 100)
	if err != nil || created != 0 {
		t.Fatalf("paused-source scheduling = %d, %v; want none", created, err)
	}
}

func TestJobQueueIsBoundedAndDeadLettersAfterFiveAttempts(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	for index := 0; index < MaxPendingJobs; index++ {
		job := Job{ID: fmt.Sprintf("queue-%04d", index), Kind: "maintenance", DedupeKey: fmt.Sprintf("queue-key-%04d", index), DueAt: now.UnixMilli()}
		if _, _, err := store.EnqueueJob(ctx, job); err != nil {
			t.Fatalf("enqueue %d: %v", index, err)
		}
	}
	if _, _, err := store.EnqueueJob(ctx, Job{ID: "queue-overflow", Kind: "maintenance", DedupeKey: "queue-overflow", DueAt: now.UnixMilli()}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("queue overflow error = %v, want ErrQueueFull", err)
	}
	store2 := openTestStore(t)
	job := Job{ID: "retry-job", Kind: "maintenance", DedupeKey: "retry-key", DueAt: now.UnixMilli()}
	if _, _, err := store2.EnqueueJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= MaxJobAttempts; attempt++ {
		claimed, err := store2.ClaimJob(ctx, "worker", now, time.Minute)
		if err != nil || claimed.AttemptCount != attempt {
			t.Fatalf("claim attempt %d = %+v, %v", attempt, claimed, err)
		}
		if err := store2.FinishJob(ctx, job.ID, "worker", JobResult{Status: "retry_at", NextAttemptAt: now.Add(time.Second).UnixMilli(), LastErrorCode: "failed"}, now); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	var status string
	if err := store2.db.QueryRowContext(ctx, "SELECT status FROM jobs WHERE id=?", job.ID).Scan(&status); err != nil || status != "dead" {
		t.Fatalf("fifth failure status = %q, %v; want dead", status, err)
	}
}

func TestWatchEvaluationLoadsOnlySameListingComparableHistory(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-watch", "product-watch", "listing-watch")
	baseAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Millisecond)
	for _, item := range []struct {
		id     string
		offer  string
		at     time.Time
		amount int64
	}{{"obs-watch-1", "offer-watch-1", baseAt, 1500}, {"obs-watch-2", "offer-watch-2", baseAt.Add(time.Hour), 1200}} {
		observation := Observation{ID: item.id, ListingID: "listing-watch", ObservedAt: item.at.UnixMilli(), FetchStatus: "success", ObservedStock: "in_stock", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
		offer := ObservedOffer{ID: item.offer, Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: item.amount, MinimumQty: 1, Evidence: "fixture:price", Eligibility: "public"}
		if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
			t.Fatal(err)
		}
	}
	target, drop := int64(1300), 1000
	if err := store.CreateWatch(ctx, "watch-eval", "product-watch", "", true, WatchRule{TargetMinor: &target, DropBasisPoints: &drop, ConditionOperator: "any"}, baseAt.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListWatchEvaluations(ctx, baseAt.Add(2*time.Hour), 24*time.Hour, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListWatchEvaluations() = %+v, %v", items, err)
	}
	item := items[0]
	if item.Current.ListingID != "listing-watch" || item.Current.Money.Minor != 1200 || item.Baseline == nil || item.Baseline.ListingID != "listing-watch" || item.Baseline.Money.Minor != 1500 {
		t.Fatalf("watch evidence did not preserve same-listing baseline: %+v", item)
	}
}

func TestProductIdentityEditAuditsAndDemotesPriorExactListings(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-product-edit", "product-product-edit", "listing-product-edit")
	if _, err := store.db.ExecContext(ctx, `INSERT INTO admin_users(id,username,password_hash,created_at,updated_at)
		VALUES ('admin-edit','owner','this-is-a-long-enough-test-password-hash',1,1)`); err != nil {
		t.Fatal(err)
	}
	current, err := store.ProductByID(ctx, "product-product-edit")
	if err != nil {
		t.Fatal(err)
	}
	current.Name = "Example product, corrected variant"
	current.Variant = "New variant"
	if err := store.UpdateCanonicalProduct(ctx, "admin-edit", current, current.UpdatedAt, "Corrected after reviewing the exact item", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var state, reason string
	if err := store.db.QueryRowContext(ctx, "SELECT match_state,match_reason FROM listings WHERE id='listing-product-edit'").Scan(&state, &reason); err != nil || state != "possible" || reason != "canonical_identity_changed" {
		t.Fatalf("listing after identity edit = %q %q, %v", state, reason, err)
	}
	var reviews, audits int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM match_reviews WHERE listing_id='listing-product-edit' AND previous_state='exact' AND new_state='possible'").Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM admin_audit WHERE action_code='product.identity_updated' AND object_id='product-product-edit'").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if reviews != 1 || audits != 1 {
		t.Fatalf("identity edit audit counts: reviews=%d audit=%d", reviews, audits)
	}
	if err := store.UpdateCanonicalProduct(ctx, "admin-edit", current, current.UpdatedAt, "stale editor", time.Now().UTC()); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale product edit error = %v, want ErrConflict", err)
	}
}

func TestValidatedHistoryExportIncludesOnlyApprovedExactPublicPrices(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-export", "product-export", "listing-export")
	baseAt := time.Now().UTC().Truncate(time.Millisecond)
	valid := Observation{
		ID: "obs-export-valid", ListingID: "listing-export", ObservedAt: baseAt.UnixMilli(), FetchStatus: "success",
		ObservedStock: "unknown", SourceURL: "https://shop.example/item/1", ValidationState: "valid",
	}
	public := ObservedOffer{ID: "offer-export-public", Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 1299,
		MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, valid, []ObservedOffer{public}); err != nil {
		t.Fatal(err)
	}
	member := valid
	member.ID = "obs-export-member"
	member.ObservedAt++
	memberOffer := public
	memberOffer.ID = "offer-export-member"
	memberOffer.Kind = "member"
	memberOffer.Eligibility = "conditional"
	if err := store.PersistObservation(ctx, member, []ObservedOffer{memberOffer}); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.ID = "obs-export-invalid"
	invalid.ObservedAt += 2
	invalid.FetchStatus = "parse_error"
	invalid.ValidationState = "invalid"
	invalid.ObservedStock = "unknown"
	if err := store.PersistObservation(ctx, invalid, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE canonical_products SET display_name='=HYPERLINK("https://bad.example")' WHERE id='product-export'`); err != nil {
		t.Fatal(err)
	}
	items, err := store.ExportValidatedHistoryPage(ctx, 0, "", MaxHistoryExportPage)
	if err != nil || len(items) != 1 {
		t.Fatalf("export rows = %+v, %v", items, err)
	}
	if items[0].ProductName != `=HYPERLINK("https://bad.example")` || items[0].AmountMinor != 1299 || items[0].Currency != "ZAR" ||
		items[0].Stock != "unknown" || items[0].Validation != "valid" || items[0].OfferID != "offer-export-public" {
		t.Fatalf("export row did not preserve validated price facts: %+v", items[0])
	}
	if _, err := store.ExportValidatedHistoryPage(ctx, 0, "", MaxHistoryExportPage+1); err == nil {
		t.Fatal("export accepted an oversized page request")
	}
	if _, err := store.ExportValidatedHistoryPage(ctx, -1, "", 1); err == nil {
		t.Fatal("export accepted an invalid cursor")
	}
}

func TestCurrentProductOffersRequireLatestFreshValidatedExactObservation(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-current-offer", "product-current-offer", "listing-current-offer")
	now := time.Now().UTC().Truncate(time.Millisecond)
	observation := Observation{
		ID: "obs-current-offer", ListingID: "listing-current-offer", ObservedAt: now.UnixMilli(), FetchStatus: "success",
		ObservedStock: "unknown", SourceURL: "https://shop.example/item/1", ValidationState: "valid",
	}
	offer := ObservedOffer{ID: "offer-current-offer", Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 2499,
		MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListProductOffers(ctx, "product-current-offer", now, 10)
	if err != nil || len(items) != 1 || items[0].AmountMinor != 2499 || items[0].Stock != "unknown" || items[0].Eligibility != "public" {
		t.Fatalf("current offers = %+v, %v", items, err)
	}
	failed := observation
	failed.ID = "obs-current-offer-failed"
	failed.ObservedAt++
	failed.FetchStatus = "parse_error"
	failed.ValidationState = "invalid"
	if err := store.PersistObservation(ctx, failed, nil); err != nil {
		t.Fatal(err)
	}
	items, err = store.ListProductOffers(ctx, "product-current-offer", now.Add(time.Minute), 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("previous price survived a latest failed check as current: %+v, %v", items, err)
	}
}

func TestProductHistoryPreservesCurrencyAndScale(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-history-currency", "product-history-currency", "listing-history-currency")
	now := time.Now().UTC().Truncate(time.Millisecond)
	observation := Observation{ID: "obs-history-currency", ListingID: "listing-history-currency", ObservedAt: now.UnixMilli(),
		FetchStatus: "success", ObservedStock: "in_stock", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
	offer := ObservedOffer{ID: "offer-history-currency", Kind: "standard", Currency: "USD", CurrencyScale: 2,
		AmountMinor: 1999, MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
		t.Fatal(err)
	}
	history, err := store.ListProductHistory(ctx, "product-history-currency", 10)
	if err != nil || len(history) != 1 || !history[0].AmountMinor.Valid || history[0].Currency != "USD" || history[0].CurrencyScale != 2 {
		t.Fatalf("product history currency = %+v, %v", history, err)
	}
}

func TestVerifiedDealsNeverDisplayForeignCurrencyAsRand(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, testCase := range []struct {
		source, product, listing, observation, offer, currency string
		amount, scale                                          int
	}{
		{"source-usd-deal", "product-usd-deal", "listing-usd-deal", "obs-usd-deal", "offer-usd-deal", "USD", 1999, 2},
		{"source-zar-deal", "product-zar-deal", "listing-zar-deal", "obs-zar-deal", "offer-zar-deal", "ZAR", 2999, 2},
	} {
		seedExactListing(t, store, testCase.source, testCase.product, testCase.listing)
		observation := Observation{ID: testCase.observation, ListingID: testCase.listing, ObservedAt: now.UnixMilli(),
			FetchStatus: "success", ObservedStock: "in_stock", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
		offer := ObservedOffer{ID: testCase.offer, Kind: "standard", Currency: testCase.currency, CurrencyScale: testCase.scale,
			AmountMinor: int64(testCase.amount), MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
		if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
			t.Fatal(err)
		}
	}
	deals, err := store.ListVerifiedDeals(ctx, 10, now)
	if err != nil || len(deals) != 1 || deals[0].Currency != "ZAR" || deals[0].AmountMinor != 2999 {
		t.Fatalf("verified deals = %+v, %v", deals, err)
	}
}

func TestObservedPriceRangeUsesOnlyComparableValidatedInStockPrices(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-range-a", "product-range", "listing-range-a")
	seedApprovedSource(t, store, "source-range-b")
	if _, err := store.db.ExecContext(ctx, `INSERT INTO listings
		(id,source_id,canonical_product_id,url,url_key,match_state,first_seen_at,created_at)
		VALUES ('listing-range-b','source-range-b','product-range','https://shop.example/item/2','https://shop.example/item/2','exact',1,1)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, item := range []struct {
		listing, observation, offer string
		at                          time.Time
		amount                      int64
		stock, eligibility          string
	}{
		{"listing-range-a", "obs-range-a", "offer-range-a", now.Add(-24 * time.Hour), 1999, "in_stock", "public"},
		{"listing-range-b", "obs-range-b", "offer-range-b", now, 2499, "in_stock", "public"},
		{"listing-range-b", "obs-range-unknown", "offer-range-unknown", now.Add(-time.Hour), 1000, "unknown", "public"},
	} {
		observation := Observation{ID: item.observation, ListingID: item.listing, ObservedAt: item.at.UnixMilli(),
			FetchStatus: "success", ObservedStock: item.stock, SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
		offer := ObservedOffer{ID: item.offer, Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: item.amount,
			MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: item.eligibility}
		if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO observed_offers
		(id,observation_id,offer_kind,currency,currency_scale,amount_minor,minimum_qty,offer_evidence,eligibility_state)
		VALUES ('offer-range-a-public-special','obs-range-a','public_special','ZAR',2,1999,1,'jsonld:offers[1]','public')`); err != nil {
		t.Fatal(err)
	}
	summary, err := store.ProductObservedPriceRange(ctx, "product-range", now.Add(-30*24*time.Hour), now)
	if err != nil || summary == nil || summary.MinimumMinor != 1999 || summary.MaximumMinor != 2499 ||
		summary.ObservationCount != 2 || summary.RetailerCount != 2 || summary.Currency != "ZAR" || summary.CurrencyScale != 2 {
		t.Fatalf("observed price range = %+v, %v", summary, err)
	}
	samples, err := store.ListProductPriceSamples(ctx, "product-range", now.Add(-30*24*time.Hour), now, 100)
	if err != nil || len(samples) != 2 || samples[0].AmountMinor != 1999 || samples[1].AmountMinor != 2499 || samples[0].Currency != "ZAR" {
		t.Fatalf("observed chart samples = %+v, %v", samples, err)
	}
	latestOnly, err := store.ListProductPriceSamples(ctx, "product-range", now.Add(-30*24*time.Hour), now, 1)
	if err != nil || len(latestOnly) != 1 || latestOnly[0].AmountMinor != 2499 {
		t.Fatalf("bounded latest chart sample = %+v, %v", latestOnly, err)
	}
	empty, err := store.ProductObservedPriceRange(ctx, "product-range", now.Add(-time.Hour), now.Add(-30*time.Minute))
	if err != nil || empty != nil {
		t.Fatalf("empty price range = %+v, %v", empty, err)
	}
}

func TestObservationAndOffersCommitTogetherAndRemainImmutable(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source1", "product1", "listing1")
	now := time.Now().UTC().UnixMilli()
	observation := Observation{
		ID: "obs1", ListingID: "listing1", ObservedAt: now, FetchStatus: "success",
		ObservedStock: "unknown", SourceURL: "https://shop.example/item/1", ValidationState: "valid",
	}
	first := ObservedOffer{ID: "offer1", Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 12345, MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, observation, []ObservedOffer{first}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE observations SET observed_title = 'rewritten' WHERE id = 'obs1'"); err == nil {
		t.Fatal("observation update succeeded despite immutability trigger")
	}
	bad := observation
	bad.ID = "obs2"
	second := first
	second.ID = "offer1" // force a late FK/unique failure after the observation insert
	if err := store.PersistObservation(ctx, bad, []ObservedOffer{second}); err == nil {
		t.Fatal("duplicate offer insert unexpectedly succeeded")
	}
	var observations, offers int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM observations WHERE id IN ('obs1','obs2')").Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM observed_offers").Scan(&offers); err != nil {
		t.Fatal(err)
	}
	if observations != 1 || offers != 1 {
		t.Fatalf("persisted after rollback: observations=%d offers=%d", observations, offers)
	}
}

func TestAlertEpisodeAndOutboxAreIdempotentAndRequireFreshExactLineage(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source1", "product1", "listing1")
	now := time.Now().UTC().Truncate(time.Millisecond)
	obs := Observation{ID: "obs1", ListingID: "listing1", ObservedAt: now.UnixMilli(), FetchStatus: "success", ObservedStock: "unknown", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
	offer := ObservedOffer{ID: "offer1", Kind: "public_special", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 999, MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, obs, []ObservedOffer{offer}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO watches(id, canonical_product_id, active, created_at, current_rule_version)
		VALUES ('watch1', 'product1', 1, ?, 1)`, now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO watch_rule_versions(watch_id, version, target_minor, notification_channel, created_at)
		VALUES ('watch1', 1, 1500, 'webhook', ?)`, now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	alert := AlertDelivery{EpisodeID: "episode1", WatchID: "watch1", RuleVersion: 1, OfferID: "offer1", EpisodeKey: "watch1:v1:offer1:threshold", ReasonCode: "threshold_met", DestinationKey: "webhook", IdempotencyKey: "alert:watch1:offer1:v1", CreatedAt: now.UnixMilli(), NextAttemptAt: now.UnixMilli(), MaxAge: time.Hour}
	id, created, err := store.CreateAlertEpisode(ctx, alert)
	if err != nil || !created || id != alert.EpisodeID {
		t.Fatalf("CreateAlertEpisode() = %q, %v, %v", id, created, err)
	}
	mismatched := alert
	mismatched.EpisodeID, mismatched.IdempotencyKey, mismatched.DestinationKey = "episode-wrong-destination", "alert:wrong-destination", "in_app"
	if _, _, err := store.CreateAlertEpisode(ctx, mismatched); !errors.Is(err, ErrAlertDestinationMismatch) {
		t.Fatalf("CreateAlertEpisode() wrong destination error = %v, want preference mismatch", err)
	}
	alert.EpisodeID = "episode2"
	id, created, err = store.CreateAlertEpisode(ctx, alert)
	if err != nil || created || id != "episode1" {
		t.Fatalf("duplicate CreateAlertEpisode() = %q, %v, %v", id, created, err)
	}
	var episodes, outbox int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM alert_episodes").Scan(&episodes); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM delivery_outbox").Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if episodes != 1 || outbox != 1 {
		t.Fatalf("episodes=%d outbox=%d, want 1 each", episodes, outbox)
	}
	delivery, err := store.ClaimDelivery(ctx, "notifier-a", now, time.Minute)
	if err != nil || delivery.Status != "leased" || delivery.AttemptCount != 1 {
		t.Fatalf("ClaimDelivery() = %+v, %v", delivery, err)
	}
	if err := store.FinishDelivery(ctx, delivery.ID, "notifier-b", DeliveryResult{Status: "delivered", HTTPStatus: 204}, now); !errors.Is(err, ErrDeliveryLeaseLost) {
		t.Fatalf("FinishDelivery wrong owner = %v, want ErrDeliveryLeaseLost", err)
	}
	if err := store.FinishDelivery(ctx, delivery.ID, "notifier-a", DeliveryResult{Status: "retry_at", NextAttemptAt: now.Add(time.Minute).UnixMilli(), HTTPStatus: 503, LastErrorCode: "provider_error"}, now); err != nil {
		t.Fatal(err)
	}
	delivery, err = store.ClaimDelivery(ctx, "notifier-b", now.Add(2*time.Minute), time.Minute)
	if err != nil || delivery.AttemptCount != 2 {
		t.Fatalf("retry ClaimDelivery() = %+v, %v", delivery, err)
	}
	if err := store.FinishDelivery(ctx, delivery.ID, "notifier-b", DeliveryResult{Status: "delivered", HTTPStatus: 204}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimDelivery(ctx, "notifier-c", now.Add(3*time.Minute), time.Minute); !errors.Is(err, ErrNoDelivery) {
		t.Fatalf("ClaimDelivery after success = %v, want ErrNoDelivery", err)
	}
	recoveryTime := now.Add(3 * time.Minute)
	recoveryObservation := Observation{ID: "obs-recovery", ListingID: "listing1", ObservedAt: recoveryTime.UnixMilli(), FetchStatus: "success", ObservedStock: "in_stock", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
	recoveryOffer := ObservedOffer{ID: "offer-recovery", Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 2000, MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, recoveryObservation, []ObservedOffer{recoveryOffer}); err != nil {
		t.Fatal(err)
	}
	closed, err := store.CloseRecoveredAlertEpisode(ctx, AlertRecovery{WatchID: "watch1", RuleVersion: 1, OfferID: "offer-recovery", RecoveredAt: recoveryTime.UnixMilli(), MaxAge: time.Hour})
	if err != nil || !closed {
		t.Fatalf("CloseRecoveredAlertEpisode() = %v, %v", closed, err)
	}
	secondTime := recoveryTime.Add(time.Minute)
	secondObservation := Observation{ID: "obs-second", ListingID: "listing1", ObservedAt: secondTime.UnixMilli(), FetchStatus: "success", ObservedStock: "unknown", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
	secondOffer := ObservedOffer{ID: "offer-second", Kind: "public_special", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 999, MinimumQty: 1, Evidence: "jsonld:offers[0]", Eligibility: "public"}
	if err := store.PersistObservation(ctx, secondObservation, []ObservedOffer{secondOffer}); err != nil {
		t.Fatal(err)
	}
	alert.EpisodeID, alert.OfferID, alert.CreatedAt = "episode-second", "offer-second", secondTime.UnixMilli()
	alert.EpisodeKey, alert.IdempotencyKey = "watch1:v1:listing1:threshold", "alert:watch1:offer-second:v1"
	id, created, err = store.CreateAlertEpisode(ctx, alert)
	if err != nil || !created || id != "episode-second" {
		t.Fatalf("CreateAlertEpisode() after recovery = %q, %v, %v", id, created, err)
	}
	var episodeNumber int
	if err := store.db.QueryRowContext(ctx, "SELECT episode_number FROM alert_episodes WHERE id = 'episode-second'").Scan(&episodeNumber); err != nil || episodeNumber != 2 {
		t.Fatalf("re-armed episode number = %d, %v", episodeNumber, err)
	}
	alert.EpisodeKey = "stale"
	alert.EpisodeID = "episode3"
	alert.CreatedAt = now.Add(2 * time.Hour).UnixMilli()
	if _, _, err := store.CreateAlertEpisode(ctx, alert); err == nil {
		t.Fatal("stale offer was accepted for an alert")
	}
}

func TestOnlineBackupHasIntegrityManifestAndRejectsTampering(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source1")
	path := filepath.Join(t.TempDir(), "snapshots", "leanprice.sqlite")
	manifest, err := store.Backup(ctx, path)
	if err != nil {
		t.Fatalf("Backup(): %v", err)
	}
	if manifest.SchemaVersion != 4 || manifest.SizeBytes <= 0 || len(manifest.SHA256) != 64 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	verified, err := ValidateBackup(ctx, path)
	if err != nil || verified.SHA256 != manifest.SHA256 {
		t.Fatalf("ValidateBackup() = %+v, %v", verified, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("tamper")); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateBackup(ctx, path); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("ValidateBackup tampered error = %v, want digest mismatch", err)
	}
}

func TestOnlineBackupRunsWhileWritesContinue(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	largeValue := strings.Repeat("x", 4000)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3000; i++ {
		if _, err := tx.ExecContext(ctx, "INSERT INTO app_settings(setting_key, setting_value, updated_at) VALUES (?, ?, 1)", fmt.Sprintf("bulk-%04d", i), largeValue); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	writeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var writes atomic.Int64
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		close(started)
		for i := 0; i < 500; i++ {
			_, err := store.db.ExecContext(writeCtx, "INSERT INTO app_settings(setting_key, setting_value, updated_at) VALUES (?, 'during backup', 2)", fmt.Sprintf("backup-write-%04d", i))
			if err != nil {
				if writeCtx.Err() != nil {
					finished <- nil
					return
				}
				finished <- err
				return
			}
			writes.Add(1)
		}
		finished <- nil
	}()
	<-started
	time.Sleep(time.Millisecond)
	path := filepath.Join(t.TempDir(), "concurrent.sqlite")
	if _, err := store.Backup(ctx, path); err != nil {
		t.Fatalf("Backup() while writing: %v", err)
	}
	cancel()
	if err := <-finished; err != nil {
		t.Fatalf("writer during backup: %v", err)
	}
	if writes.Load() == 0 {
		t.Fatal("no writes completed during the online snapshot")
	}
	if _, err := ValidateBackup(ctx, path); err != nil {
		t.Fatalf("ValidateBackup(concurrent): %v", err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var captured int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM app_settings WHERE setting_key LIKE 'backup-write-%'").Scan(&captured); err != nil {
		t.Fatal(err)
	}
	if captured < 0 || int64(captured) > writes.Load() {
		t.Fatalf("snapshot includes %d concurrent writes, writer completed %d", captured, writes.Load())
	}
}

func TestRestoreStagesAndUpgradesBeforeAtomicReplace(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "live", "leanprice.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	seedApprovedSource(t, store, "source1")
	backupPath := filepath.Join(t.TempDir(), "backup.sqlite")
	if _, err := store.Backup(ctx, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "INSERT INTO retailers(id, name) VALUES ('later', 'Later')"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := RestoreDatabase(ctx, path, backupPath)
	if err != nil {
		t.Fatalf("RestoreDatabase(): %v", err)
	}
	if result.SafetyBackupPath == "" || result.SafetyBackup.SizeBytes <= 0 {
		t.Fatalf("missing safety backup result: %+v", result)
	}
	if _, err := ValidateBackup(ctx, result.SafetyBackupPath); err != nil {
		t.Fatalf("ValidateBackup(safety): %v", err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(restored): %v", err)
	}
	defer store.Close()
	var sourceCount, laterCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM sources WHERE id='source1'").Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM retailers WHERE id='later'").Scan(&laterCount); err != nil {
		t.Fatal(err)
	}
	if sourceCount != 1 || laterCount != 0 {
		t.Fatalf("restored contents: source=%d later retailer=%d", sourceCount, laterCount)
	}
}

func TestRepresentativeCatalogQueriesUseBoundedIndexes(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedApprovedSource(t, store, "source1")
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	productInsert, err := tx.PrepareContext(ctx, `INSERT INTO canonical_products
		(id, display_name, quantity_numerator, quantity_denominator, quantity_dimension, quantity_unit, created_at, updated_at)
		VALUES (?, ?, 1, 1, 'count', 'item', 1, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	listingInsert, err := tx.PrepareContext(ctx, `INSERT INTO listings
		(id, source_id, canonical_product_id, url, url_key, match_state, first_seen_at, created_at)
		VALUES (?, 'source1', ?, ?, ?, 'exact', 1, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	for product := 0; product < 100; product++ {
		productID := fmt.Sprintf("product-%03d", product)
		if _, err := productInsert.ExecContext(ctx, productID, fmt.Sprintf("Example product %03d", product)); err != nil {
			t.Fatal(err)
		}
		for listing := 0; listing < 5; listing++ {
			listingID := fmt.Sprintf("listing-%03d-%d", product, listing)
			url := fmt.Sprintf("https://shop.example/item/%03d/%d", product, listing)
			if _, err := listingInsert.ExecContext(ctx, listingID, productID, url, url); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = productInsert.Close()
	_ = listingInsert.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertPlanUses := func(query, index string, args ...any) {
		t.Helper()
		rows, err := store.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var details []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			details = append(details, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(details, " "), index) {
			t.Fatalf("query plan %q does not use %s: %v", query, index, details)
		}
	}
	assertPlanUses("SELECT id, display_name FROM canonical_products WHERE archived = 0 AND display_name >= ? ORDER BY display_name, id LIMIT ?", "idx_products_name", "Example", 20)
	assertPlanUses("SELECT id FROM listings WHERE canonical_product_id = ? AND match_state = 'exact' ORDER BY id LIMIT ?", "idx_listings_product_match", "product-050", 5)
}

func TestWatchRuleUpdatesAppendVersionAndRejectStaleEdits(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source1", "product1", "listing1")
	now := time.Now().UTC()
	target := int64(1200)
	if err := store.CreateWatch(ctx, "watch1", "product1", "listing1", true, WatchRule{TargetMinor: &target}, now); err != nil {
		t.Fatal(err)
	}
	nextTarget := int64(1000)
	version, err := store.UpdateWatchRule(ctx, "watch1", 1, WatchRule{TargetMinor: &nextTarget}, now.Add(time.Second))
	if err != nil || version != 2 {
		t.Fatalf("UpdateWatchRule() = %d, %v", version, err)
	}
	if _, err := store.UpdateWatchRule(ctx, "watch1", 1, WatchRule{TargetMinor: &target}, now.Add(2*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale UpdateWatchRule() error = %v, want ErrConflict", err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE watch_rule_versions SET target_minor = 900 WHERE watch_id='watch1' AND version=1"); err == nil {
		t.Fatal("immutable watch rule version update succeeded")
	}
	var current, versions int
	if err := store.db.QueryRowContext(ctx, "SELECT current_rule_version FROM watches WHERE id='watch1'").Scan(&current); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM watch_rule_versions WHERE watch_id='watch1'").Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if current != 2 || versions != 2 {
		t.Fatalf("current rule=%d, versions=%d; want 2 and 2", current, versions)
	}
}

func TestAlertCreationRejectsPausedAndStaleWatchEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, *Store, AlertDelivery, time.Time)
	}{
		{
			name: "paused source",
			change: func(t *testing.T, store *Store, _ AlertDelivery, _ time.Time) {
				if _, err := store.db.ExecContext(context.Background(), "UPDATE sources SET enabled=0 WHERE id='source-alert'"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "paused watch",
			change: func(t *testing.T, store *Store, alert AlertDelivery, _ time.Time) {
				if err := store.SetWatchActive(context.Background(), alert.WatchID, false); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "superseded rule version",
			change: func(t *testing.T, store *Store, alert AlertDelivery, now time.Time) {
				nextTarget := int64(900)
				if version, err := store.UpdateWatchRule(context.Background(), alert.WatchID, 1, WatchRule{TargetMinor: &nextTarget}, now.Add(time.Second)); err != nil || version != 2 {
					t.Fatalf("UpdateWatchRule() = %d, %v", version, err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, now, alert := seedAlertEpisodeScenario(t)
			test.change(t, store, alert, now)
			if _, _, err := store.CreateAlertEpisode(context.Background(), alert); err == nil {
				t.Fatal("CreateAlertEpisode() accepted evidence after its source/watch/rule was paused or superseded")
			}
		})
	}
}

func TestConcurrentAlertEvaluationsCreateOneEpisodeAndOutboxRow(t *testing.T) {
	store, _, alert := seedAlertEpisodeScenario(t)
	const workers = 12
	type outcome struct {
		id      string
		created bool
		err     error
	}
	results := make(chan outcome, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			candidate := alert
			candidate.EpisodeID = fmt.Sprintf("episode-alert-%02d", index)
			candidate.IdempotencyKey = "episode:" + candidate.EpisodeID
			id, created, err := store.CreateAlertEpisode(context.Background(), candidate)
			results <- outcome{id: id, created: created, err: err}
		}(index)
	}
	group.Wait()
	close(results)
	createdCount := 0
	winner := ""
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent CreateAlertEpisode(): %v", result.err)
		}
		if winner == "" {
			winner = result.id
		}
		if result.id != winner {
			t.Fatalf("concurrent evaluations returned different episodes %q and %q", winner, result.id)
		}
		if result.created {
			createdCount++
		}
	}
	var episodes, outbox int
	if err := store.db.QueryRowContext(context.Background(), "SELECT count(*) FROM alert_episodes").Scan(&episodes); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(context.Background(), "SELECT count(*) FROM delivery_outbox").Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if createdCount != 1 || episodes != 1 || outbox != 1 {
		t.Fatalf("concurrent result created=%d episodes=%d outbox=%d; want one each", createdCount, episodes, outbox)
	}
}

func TestDeliveryOutboxDeadLettersAfterFiveFailedAttempts(t *testing.T) {
	store, now, alert := seedAlertEpisodeScenario(t)
	if _, created, err := store.CreateAlertEpisode(context.Background(), alert); err != nil || !created {
		t.Fatalf("CreateAlertEpisode() = created %v, error %v", created, err)
	}
	for attempt := 1; attempt <= 5; attempt++ {
		delivery, err := store.ClaimDelivery(context.Background(), "notifier", now, time.Minute)
		if err != nil || delivery.AttemptCount != attempt {
			t.Fatalf("ClaimDelivery() attempt %d = %+v, %v", attempt, delivery, err)
		}
		err = store.FinishDelivery(context.Background(), delivery.ID, "notifier", DeliveryResult{
			Status: "retry_at", NextAttemptAt: now.Add(5 * time.Second).UnixMilli(), LastErrorCode: "delivery_failed",
		}, now)
		if err != nil {
			t.Fatalf("FinishDelivery() attempt %d: %v", attempt, err)
		}
		now = now.Add(6 * time.Second)
	}
	var status string
	var attempts int
	if err := store.db.QueryRowContext(context.Background(), "SELECT status,attempt_count FROM delivery_outbox WHERE alert_episode_id=?", alert.EpisodeID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "dead" || attempts != 5 {
		t.Fatalf("outbox after repeated failures = status %q attempts %d; want dead after five", status, attempts)
	}
	if _, err := store.ClaimDelivery(context.Background(), "notifier", now, time.Minute); !errors.Is(err, ErrNoDelivery) {
		t.Fatalf("ClaimDelivery() after terminal failure = %v, want ErrNoDelivery", err)
	}
}

func seedAlertEpisodeScenario(t *testing.T) (*Store, time.Time, AlertDelivery) {
	t.Helper()
	ctx := context.Background()
	store := openTestStore(t)
	seedExactListing(t, store, "source-alert", "product-alert", "listing-alert")
	now := time.Now().UTC().Truncate(time.Millisecond)
	target := int64(1000)
	if err := store.CreateWatch(ctx, "watch-alert", "product-alert", "", true,
		WatchRule{TargetMinor: &target, ConditionOperator: "any", NotificationChannel: "webhook"}, now); err != nil {
		t.Fatal(err)
	}
	observation := Observation{ID: "obs-alert", ListingID: "listing-alert", ObservedAt: now.UnixMilli(), FetchStatus: "success",
		ObservedStock: "in_stock", SourceURL: "https://shop.example/item/1", ValidationState: "valid"}
	offer := ObservedOffer{ID: "offer-alert", Kind: "standard", Currency: "ZAR", CurrencyScale: 2, AmountMinor: 900,
		MinimumQty: 1, Evidence: "fixture:price", Eligibility: "public"}
	if err := store.PersistObservation(ctx, observation, []ObservedOffer{offer}); err != nil {
		t.Fatal(err)
	}
	alert := AlertDelivery{EpisodeID: "episode-alert", WatchID: "watch-alert", RuleVersion: 1, OfferID: "offer-alert",
		EpisodeKey: "watch:watch-alert:listing-alert:v1", ReasonCode: "threshold_met", DestinationKey: "webhook",
		IdempotencyKey: "episode:episode-alert", CreatedAt: now.UnixMilli(), NextAttemptAt: now.UnixMilli(), MaxAge: time.Hour}
	return store, now, alert
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "leanprice.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedApprovedSource(t *testing.T, store *Store, sourceID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, "INSERT INTO retailers(id, name) VALUES (?, ?)", "retailer-"+sourceID, "Example "+sourceID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO sources
		(id, retailer_id, source_key, base_host, assessment_state, assessment_ref, assessment_at, enabled, created_at, updated_at)
		VALUES (?, ?, ?, 'shop.example', 'approved', 'reviewed-test', 1, 1, 1, 1)`, sourceID, "retailer-"+sourceID, sourceID); err != nil {
		t.Fatal(err)
	}
}

func seedExactListing(t *testing.T, store *Store, sourceID, productID, listingID string) {
	t.Helper()
	ctx := context.Background()
	seedApprovedSource(t, store, sourceID)
	if _, err := store.db.ExecContext(ctx, `INSERT INTO canonical_products
		(id, display_name, quantity_numerator, quantity_denominator, quantity_dimension, quantity_unit, created_at, updated_at)
		VALUES (?, 'Example item', 1, 1, 'count', 'item', 1, 1)`, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO listings
		(id, source_id, canonical_product_id, url, url_key, match_state, first_seen_at, created_at)
		VALUES (?, ?, ?, 'https://shop.example/item/1', 'https://shop.example/item/1', 'exact', 1, 1)`, listingID, sourceID, productID); err != nil {
		t.Fatal(err)
	}
}
