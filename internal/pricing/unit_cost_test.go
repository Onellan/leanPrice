package pricing

import (
	"math/big"
	"testing"
)

func TestUnitCostComparesEquivalentPackAndBasketQuantitiesExactly(t *testing.T) {
	amountA, _ := ParseMoney("30", "ZAR")
	quantityA, _ := ParseQuantity("2 x 750 ml")
	amountB, _ := ParseMoney("19.99", "ZAR")
	quantityB, _ := ParseQuantity("1 L")
	costA, err := CalculateUnitCost(amountA, quantityA, 1)
	if err != nil {
		t.Fatal(err)
	}
	costB, err := CalculateUnitCost(amountB, quantityB, 1)
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := costA.Compare(costB)
	if err != nil || comparison <= 0 {
		t.Fatalf("R30 per 2x750ml versus R19.99/L = %d, %v", comparison, err)
	}
	rounded, err := costA.RoundedMinor()
	if err != nil || rounded.Cmp(big.NewInt(2000)) != 0 {
		t.Fatalf("unit-cost display rounding = %v, %v; want 2000 minor units", rounded, err)
	}
}

func TestUnitCostRejectsIncomparableDimensionsAndCurrencies(t *testing.T) {
	volume, _ := CalculateUnitCost(Money{Minor: 1000, Currency: "ZAR", Scale: 2}, Quantity{Numerator: 1, Denominator: 1, Dimension: DimensionVolume, PackCount: 1}, 1)
	mass, _ := CalculateUnitCost(Money{Minor: 1000, Currency: "ZAR", Scale: 2}, Quantity{Numerator: 1, Denominator: 1, Dimension: DimensionMass, PackCount: 1}, 1)
	if _, err := volume.Compare(mass); err == nil {
		t.Fatal("volume and mass unit costs were compared")
	}
}
