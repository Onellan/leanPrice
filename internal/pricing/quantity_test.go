package pricing

import "testing"

func TestQuantityComparisonPreservesDimensionAndPackCount(t *testing.T) {
	base := Quantity{Numerator: 750, Denominator: 1000, Dimension: DimensionVolume, PackCount: 1}
	equivalent := Quantity{Numerator: 3, Denominator: 4, Dimension: DimensionVolume, PackCount: 1}
	if same, err := base.SamePhysical(equivalent); err != nil || !same {
		t.Fatalf("equivalent quantities = %v, %v", same, err)
	}
	variants := []Quantity{
		{Numerator: 1, Denominator: 1, Dimension: DimensionVolume, PackCount: 1},
		{Numerator: 750, Denominator: 1000, Dimension: DimensionVolume, PackCount: 2},
		{Numerator: 750, Denominator: 1000, Dimension: DimensionMass, PackCount: 1},
	}
	for _, other := range variants {
		if same, err := base.SamePhysical(other); err != nil || same {
			t.Fatalf("mismatched quantity %#v = %v, %v", other, same, err)
		}
	}
}

func TestParseQuantityNormalizesUnitsAndPacks(t *testing.T) {
	tests := []struct {
		input       string
		numerator   int64
		denominator int64
		dimension   Dimension
		packs       int64
	}{
		{input: "750 ml", numerator: 3, denominator: 4, dimension: DimensionVolume, packs: 1},
		{input: "0.75 L", numerator: 3, denominator: 4, dimension: DimensionVolume, packs: 1},
		{input: "2 x 750ml", numerator: 3, denominator: 4, dimension: DimensionVolume, packs: 2},
		{input: "1,5 kg", numerator: 3, denominator: 2, dimension: DimensionMass, packs: 1},
		{input: "12 count", numerator: 12, denominator: 1, dimension: DimensionCount, packs: 1},
	}
	for _, test := range tests {
		got, err := ParseQuantity(test.input)
		if err != nil {
			t.Fatalf("ParseQuantity(%q): %v", test.input, err)
		}
		if got.Numerator != test.numerator || got.Denominator != test.denominator || got.Dimension != test.dimension || got.PackCount != test.packs {
			t.Errorf("ParseQuantity(%q) = %#v", test.input, got)
		}
	}
	for _, input := range []string{"0 ml", "-2 kg", "750 ml x 2", "1 box", ""} {
		if _, err := ParseQuantity(input); err == nil {
			t.Errorf("ParseQuantity(%q) accepted unsupported/ambiguous quantity", input)
		}
	}
}
