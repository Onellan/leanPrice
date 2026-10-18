// Package match performs deterministic, fail-closed product identity matching.
package match

import (
	"errors"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/Onellan/leanPrice/internal/pricing"
)

const AlgorithmVersion = "identity-v1"

type Status string

const (
	StatusExact    Status = "exact"
	StatusPossible Status = "possible"
	StatusRejected Status = "rejected"
)

type Identity struct {
	Brand        string
	Line         string
	Variant      string
	VariantKnown bool
	Quantity     *pricing.Quantity
	GTIN         string
}

type Decision struct {
	Status          Status
	Confidence      int
	Reason          string
	Algorithm       string
	NormalizedGTIN  string
	ReviewedBy      string
	ReviewedAt      *time.Time
	ReviewRationale string
}

type ManualReview struct {
	Before     Decision
	After      Decision
	Actor      string
	At         time.Time
	Rationale  string
	PreviousID string
}

var ErrInvalidReview = errors.New("invalid manual match review")

func NormalizeText(value string) string {
	value = norm.NFKC.String(value)
	var builder strings.Builder
	space := true
	for _, r := range value {
		r = unicode.ToLower(r)
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func NormalizeGTIN(value string) (string, bool) {
	var digits strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) || r == '-' {
			continue
		}
		if r < '0' || r > '9' {
			return "", false
		}
		digits.WriteRune(r)
	}
	gtin := digits.String()
	if len(gtin) != 8 && len(gtin) != 12 && len(gtin) != 13 && len(gtin) != 14 {
		return "", false
	}
	sum := 0
	weight := 3
	for i := len(gtin) - 2; i >= 0; i-- {
		sum += int(gtin[i]-'0') * weight
		if weight == 3 {
			weight = 1
		} else {
			weight = 3
		}
	}
	checkDigit := (10 - sum%10) % 10
	if int(gtin[len(gtin)-1]-'0') != checkDigit {
		return "", false
	}
	return gtin, true
}

func Evaluate(canonical, listing Identity) Decision {
	decision := Decision{Status: StatusPossible, Algorithm: AlgorithmVersion, Reason: "insufficient_identity_fields"}
	canonicalBrand, listingBrand := NormalizeText(canonical.Brand), NormalizeText(listing.Brand)
	canonicalLine, listingLine := NormalizeText(canonical.Line), NormalizeText(listing.Line)
	canonicalVariant, listingVariant := NormalizeText(canonical.Variant), NormalizeText(listing.Variant)
	canonicalGTIN, canonicalGTINValid := NormalizeGTIN(canonical.GTIN)
	listingGTIN, listingGTINValid := NormalizeGTIN(listing.GTIN)
	if listingGTINValid {
		decision.NormalizedGTIN = listingGTIN
	} else if canonicalGTINValid {
		decision.NormalizedGTIN = canonicalGTIN
	}

	if canonicalBrand != "" && listingBrand != "" && canonicalBrand != listingBrand {
		return rejected(decision, "brand_mismatch")
	}
	if canonicalLine != "" && listingLine != "" && canonicalLine != listingLine {
		return rejected(decision, "product_line_mismatch")
	}
	if canonical.VariantKnown && listing.VariantKnown && canonicalVariant != listingVariant {
		return rejected(decision, "variant_mismatch")
	}
	if canonical.Quantity != nil && listing.Quantity != nil {
		same, err := canonical.Quantity.SamePhysical(*listing.Quantity)
		if err != nil {
			decision.Reason = "invalid_quantity"
			return decision
		}
		if !same {
			return rejected(decision, "quantity_or_pack_mismatch")
		}
	}
	if canonicalGTINValid && listingGTINValid && canonicalGTIN != listingGTIN {
		return rejected(decision, "gtin_mismatch")
	}

	complete := canonicalBrand != "" && listingBrand != "" && canonicalLine != "" && listingLine != "" &&
		canonical.VariantKnown && listing.VariantKnown && canonical.Quantity != nil && listing.Quantity != nil
	if !complete {
		return decision
	}
	decision.Status = StatusExact
	decision.Confidence = 100
	decision.Reason = "physical_identity_matches"
	return decision
}

func ApplyManualConfirmation(previous Decision, actor string, at time.Time, rationale string, previousID string) (ManualReview, error) {
	actor = strings.TrimSpace(actor)
	rationale = strings.TrimSpace(rationale)
	if previous.Status != StatusPossible || actor == "" || len(actor) > 128 || rationale == "" || len(rationale) > 1000 || at.IsZero() || previousID == "" {
		return ManualReview{}, ErrInvalidReview
	}
	confirmed := previous
	confirmed.Status = StatusExact
	confirmed.Confidence = 100
	confirmed.Reason = "human_confirmed_possible_match"
	confirmed.ReviewedBy = actor
	reviewedAt := at.UTC()
	confirmed.ReviewedAt = &reviewedAt
	confirmed.ReviewRationale = rationale
	return ManualReview{Before: previous, After: confirmed, Actor: actor, At: reviewedAt, Rationale: rationale, PreviousID: previousID}, nil
}

func rejected(previous Decision, reason string) Decision {
	previous.Status = StatusRejected
	previous.Confidence = 0
	previous.Reason = reason
	return previous
}
