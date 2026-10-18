package pricing

import (
	"errors"
	"strings"
	"testing"
)

func TestParseMoneyUsesExactMinorUnits(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		currency string
		want     int64
	}{
		{name: "decimal dot", input: "29.99", currency: "ZAR", want: 2999},
		{name: "South African grouping", input: "R 1 299,90", currency: "ZAR", want: 129990},
		{name: "unicode spaces", input: "ZAR 1 299,9", currency: "zar", want: 129990},
		{name: "mixed grouping", input: "1,299.90", currency: "ZAR", want: 129990},
		{name: "integer", input: "42", currency: "ZAR", want: 4200},
		{name: "one decimal", input: "42,5", currency: "ZAR", want: 4250},
		{name: "foreign currency explicit", input: "12.50", currency: "USD", want: 1250},
		{name: "zero decimal currency", input: "1,234", currency: "JPY", want: 1234},
		{name: "three decimal currency", input: "12.345", currency: "KWD", want: 12345},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseMoney(test.input, test.currency)
			if err != nil {
				t.Fatalf("ParseMoney(%q): %v", test.input, err)
			}
			scale, _ := MinorUnitScale(test.currency)
			if got.Minor != test.want || got.Scale != scale || got.Currency != strings.ToUpper(test.currency) {
				t.Fatalf("ParseMoney(%q) = %#v, want %d minor units", test.input, got, test.want)
			}
		})
	}
}

func TestParseMoneyRejectsAmbiguousOrInvalidValues(t *testing.T) {
	for _, input := range []string{"", "0", "0.00", "-1.00", "+1.00", "1.001", "1,29,90", "1.2.3", "NaN", "R 1,234,56", "92233720368547759.00"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseMoney(input, "ZAR"); !errors.Is(err, ErrInvalidMoney) {
				t.Fatalf("ParseMoney(%q) error = %v, want ErrInvalidMoney", input, err)
			}
		})
	}
	if _, err := ParseMoney("1.00", "ZAR1"); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("invalid currency error = %v", err)
	}
}

func TestMoneyComparisonRequiresSameCurrency(t *testing.T) {
	zar := Money{Minor: 2999, Currency: "ZAR", Scale: 2}
	usd := Money{Minor: 2999, Currency: "USD", Scale: 2}
	if _, err := zar.Compare(usd); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("cross-currency comparison error = %v", err)
	}
	if result, err := zar.Compare(Money{Minor: 3000, Currency: "ZAR", Scale: 2}); err != nil || result >= 0 {
		t.Fatalf("same-currency comparison = %d, %v", result, err)
	}
}
