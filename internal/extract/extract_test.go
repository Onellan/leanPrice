package extract

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/pricing"
)

const productJSONLD = `{"@context":"https://schema.org","@type":"Product","name":"Domestos Lavender 750 ml","brand":{"@type":"Brand","name":"Domestos"},"color":"Lavender","size":"750 ml","gtin13":"4006381333931","offers":{"@type":"Offer","price":"29.99","priceCurrency":"ZAR","availability":"https://schema.org/InStock"}}`

func testInput() Input {
	return Input{SourceURL: "https://retailer.example/products/abc?session=secret#offer", ObservedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
}

func TestParseJSONLDProductKeepsBoundedProvenanceAndUnknownEligibility(t *testing.T) {
	markup := []byte("<html><head><script type=\"application/ld+json\">" + productJSONLD + "</script></head><body><h1>Domestos Lavender 750 ml</h1></body></html>")
	result, err := Parse(markup, testInput())
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}
	if result.Status != StatusParsed || result.Title != "Domestos Lavender 750 ml" || result.Brand != "Domestos" || result.Variant != "Lavender" {
		t.Fatalf("unexpected product extraction: %#v", result)
	}
	if result.Quantity == nil || result.Quantity.Numerator != 3 || result.Quantity.Denominator != 4 || result.Quantity.Dimension != pricing.DimensionVolume {
		t.Fatalf("quantity = %#v", result.Quantity)
	}
	if result.GTIN != "4006381333931" || result.EvidenceDigest == "" || result.FieldOrigins["title"].Kind != "json_ld" || result.FieldOrigins["gtin"].Path != "jsonld[0].gtin13" || result.FieldOrigins["quantity"].Path != "jsonld[0].size" {
		t.Fatalf("provenance not retained: %#v", result)
	}
	if result.SourceURL != "https://retailer.example/products/abc" {
		t.Fatalf("URL was not sanitized: %q", result.SourceURL)
	}
	if len(result.Offers) != 1 || !result.Offers[0].Valid || result.Offers[0].Offer.Money.Minor != 2999 {
		t.Fatalf("offers = %#v", result.Offers)
	}
	if result.Offers[0].Offer.Type != pricing.OfferUnknown || result.Offers[0].Offer.Eligibility != pricing.EligibilityUnknown || result.Offers[0].Offer.TermsKnown {
		t.Fatalf("generic JSON-LD was incorrectly promoted to public offer: %#v", result.Offers[0].Offer)
	}
	if result.Stock != pricing.StockInStock {
		t.Fatalf("stock = %q", result.Stock)
	}
}

func TestSyntheticSanitizedFixture(t *testing.T) {
	markup, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "synthetic", "domestos-lavender-750.html"))
	if err != nil {
		t.Fatalf("read synthetic fixture: %v", err)
	}
	result, err := Parse(markup, testInput())
	if err != nil || result.Status != StatusParsed || result.Title != "Domestos Lavender 750 ml" || result.Quantity == nil || len(result.Offers) != 1 {
		t.Fatalf("synthetic fixture extraction = %#v, %v", result, err)
	}
	if result.SourceURL == "" || result.EvidenceDigest == "" {
		t.Fatal("synthetic fixture missing sanitized provenance")
	}
}

func TestParseRejectsConflictingStructuredAndHTMLFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		warn string
	}{
		{
			name: "price conflict",
			body: `<meta property="product:price:amount" content="30.00"><meta property="product:price:currency" content="ZAR"><script type="application/ld+json">` + productJSONLD + `</script><h1>Domestos Lavender 750 ml</h1>`,
			warn: "structured_price_conflict",
		},
		{
			name: "title conflict",
			body: `<script type="application/ld+json">` + productJSONLD + `</script><h1>Domestos Original 750 ml</h1>`,
			warn: "structured_title_conflict",
		},
		{
			name: "multiple product conflict",
			body: `<script type="application/ld+json">[` + productJSONLD + `,{"@type":"Product","name":"Other product","brand":"Other"}]</script>`,
			warn: "multiple_conflicting_products",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Parse([]byte(test.body), testInput())
			if err != nil {
				t.Fatalf("Parse(): %v", err)
			}
			if result.Status != StatusConflict || !contains(result.Warnings, test.warn) || len(result.Offers) != 0 {
				t.Fatalf("conflicting extraction was accepted: %#v", result)
			}
		})
	}
}

func TestParseReturnsUnknownForMalformedOrMissingOffer(t *testing.T) {
	markup := []byte(`<script type="application/ld+json">{"@type":"Product","name":"Product","offers":{"price":"0","priceCurrency":"ZAR"}}</script>`)
	result, err := Parse(markup, testInput())
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}
	if result.Status != StatusParsed || len(result.Offers) != 1 || result.Offers[0].Valid || result.Offers[0].InvalidReason != "invalid_price" {
		t.Fatalf("invalid price was not classified: %#v", result)
	}
	if result.Stock != pricing.StockUnknown {
		t.Fatalf("missing stock was inferred: %q", result.Stock)
	}

	missing, err := Parse([]byte(`<html><h1>Product</h1></html>`), testInput())
	if err != nil || missing.Status != StatusNoProduct || len(missing.Offers) != 0 {
		t.Fatalf("missing structured product = %#v, %v", missing, err)
	}
}

func TestParseEnforcesBodyAndDigestLimits(t *testing.T) {
	if _, err := Parse(make([]byte, MaxMarkupBytes+1), testInput()); !errors.Is(err, ErrMarkupTooLarge) {
		t.Fatalf("oversized input error = %v", err)
	}
	input := testInput()
	input.BodyDigest = strings.Repeat("0", 64)
	if _, err := Parse([]byte("<h1>Product</h1>"), input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("mismatched digest error = %v", err)
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
