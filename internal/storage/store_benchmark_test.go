package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkSyntheticPiCatalogPages measures bounded product and history reads
// against the planned 100-product/500-listing catalog shape. It is workstation
// query evidence only; it does not certify Raspberry Pi memory or latency.
func BenchmarkSyntheticPiCatalogPages(b *testing.B) {
	b.StopTimer()
	store, err := Open(context.Background(), filepath.Join(b.TempDir(), "synthetic.sqlite"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	seedSyntheticPiCatalog(b, store)

	ctx := context.Background()
	b.ReportMetric(100, "products")
	b.ReportMetric(500, "listings")
	b.Run("product_first_page", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			page, more, err := store.ListProductPage(ctx, "", "", "", 25)
			if err != nil || len(page) != 25 || !more {
				b.Fatalf("ListProductPage() = %d rows, more=%v, err=%v", len(page), more, err)
			}
		}
	})
	b.Run("product_page_after_sqlite_cache_shrink", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := store.db.ExecContext(ctx, "PRAGMA shrink_memory"); err != nil {
				b.Fatal(err)
			}
			page, more, err := store.ListProductPage(ctx, "", "", "", 25)
			if err != nil || len(page) != 25 || !more {
				b.Fatalf("ListProductPage() after cache shrink = %d rows, more=%v, err=%v", len(page), more, err)
			}
		}
	})
	b.Run("history_first_page", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			page, more, err := store.ListProductHistoryPage(ctx, "product-000", nil, 25)
			if err != nil || len(page) != 25 || !more {
				b.Fatalf("ListProductHistoryPage() = %d rows, more=%v, err=%v", len(page), more, err)
			}
		}
	})
	b.Run("history_page_after_sqlite_cache_shrink", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := store.db.ExecContext(ctx, "PRAGMA shrink_memory"); err != nil {
				b.Fatal(err)
			}
			page, more, err := store.ListProductHistoryPage(ctx, "product-000", nil, 25)
			if err != nil || len(page) != 25 || !more {
				b.Fatalf("ListProductHistoryPage() after cache shrink = %d rows, more=%v, err=%v", len(page), more, err)
			}
		}
	})
	b.Run("product_and_history_pages_parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ReportMetric(2, "queries/op")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				productPage, productMore, productErr := store.ListProductPage(ctx, "", "", "", 25)
				if productErr != nil || len(productPage) != 25 || !productMore {
					b.Errorf("parallel ListProductPage() = %d rows, more=%v, err=%v", len(productPage), productMore, productErr)
					return
				}
				historyPage, historyMore, historyErr := store.ListProductHistoryPage(ctx, "product-000", nil, 25)
				if historyErr != nil || len(historyPage) != 25 || !historyMore {
					b.Errorf("parallel ListProductHistoryPage() = %d rows, more=%v, err=%v", len(historyPage), historyMore, historyErr)
					return
				}
			}
		})
	})
}

func seedSyntheticPiCatalog(b *testing.B, store *Store) {
	b.Helper()
	ctx := context.Background()
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	retailerStmt, err := tx.PrepareContext(ctx, "INSERT INTO retailers(id,name) VALUES(?,?)")
	if err != nil {
		b.Fatal(err)
	}
	sourceStmt, err := tx.PrepareContext(ctx, "INSERT INTO sources(id,retailer_id,source_key,base_host,assessment_state,assessment_ref,assessment_at,enabled,created_at,updated_at) VALUES(?,?,?,?,'approved','synthetic-benchmark',?,1,?,?)")
	if err != nil {
		_ = retailerStmt.Close()
		b.Fatal(err)
	}
	productStmt, err := tx.PrepareContext(ctx, "INSERT INTO canonical_products(id,display_name,quantity_numerator,quantity_denominator,quantity_dimension,quantity_unit,created_at,updated_at) VALUES(?,?,1,1,'count','item',?,?)")
	if err != nil {
		_ = retailerStmt.Close()
		_ = sourceStmt.Close()
		b.Fatal(err)
	}
	listingStmt, err := tx.PrepareContext(ctx, "INSERT INTO listings(id,source_id,canonical_product_id,url,url_key,match_state,first_seen_at,created_at) VALUES(?,?,?,?,?,'exact',?,?)")
	if err != nil {
		_ = retailerStmt.Close()
		_ = sourceStmt.Close()
		_ = productStmt.Close()
		b.Fatal(err)
	}
	observationStmt, err := tx.PrepareContext(ctx, "INSERT INTO observations(id,listing_id,observed_at,fetch_status,observed_stock,source_url,validation_state) VALUES(?,?,?,'success','in_stock',?,'valid')")
	if err != nil {
		_ = retailerStmt.Close()
		_ = sourceStmt.Close()
		_ = productStmt.Close()
		_ = listingStmt.Close()
		b.Fatal(err)
	}
	offerStmt, err := tx.PrepareContext(ctx, "INSERT INTO observed_offers(id,observation_id,offer_kind,currency,currency_scale,amount_minor,minimum_qty,offer_evidence,eligibility_state) VALUES(?,?,'standard','ZAR',2,?,1,'synthetic:price','public')")
	if err != nil {
		_ = retailerStmt.Close()
		_ = sourceStmt.Close()
		_ = productStmt.Close()
		_ = listingStmt.Close()
		_ = observationStmt.Close()
		b.Fatal(err)
	}
	defer retailerStmt.Close()
	defer sourceStmt.Close()
	defer productStmt.Close()
	defer listingStmt.Close()
	defer observationStmt.Close()
	defer offerStmt.Close()

	for source := 0; source < 5; source++ {
		retailerID := fmt.Sprintf("retailer-%02d", source)
		sourceID := fmt.Sprintf("source-%02d", source)
		host := fmt.Sprintf("shop-%02d.example", source)
		if _, err := retailerStmt.ExecContext(ctx, retailerID, retailerID); err != nil {
			b.Fatal(err)
		}
		if _, err := sourceStmt.ExecContext(ctx, sourceID, retailerID, sourceID, host, base, base, base); err != nil {
			b.Fatal(err)
		}
	}
	for product := 0; product < 100; product++ {
		productID := fmt.Sprintf("product-%03d", product)
		if _, err := productStmt.ExecContext(ctx, productID, fmt.Sprintf("Synthetic Product %03d", product), base, base); err != nil {
			b.Fatal(err)
		}
		for source := 0; source < 5; source++ {
			listingID := fmt.Sprintf("listing-%03d-%02d", product, source)
			host := fmt.Sprintf("shop-%02d.example", source)
			url := fmt.Sprintf("https://%s/product/%03d", host, product)
			if _, err := listingStmt.ExecContext(ctx, listingID, fmt.Sprintf("source-%02d", source), productID, url, url, base, base); err != nil {
				b.Fatal(err)
			}
			for sample := 0; sample < 8; sample++ {
				observedAt := base + int64((product*5*8+source*8+sample+1))*time.Minute.Milliseconds()
				observationID := fmt.Sprintf("observation-%03d-%02d-%02d", product, source, sample)
				offerID := "offer-" + observationID
				if _, err := observationStmt.ExecContext(ctx, observationID, listingID, observedAt, url); err != nil {
					b.Fatal(err)
				}
				amount := int64(1000 + product + source*10 + sample)
				if _, err := offerStmt.ExecContext(ctx, offerID, observationID, amount); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
}
