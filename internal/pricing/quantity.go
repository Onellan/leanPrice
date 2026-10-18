package pricing

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

type Dimension string

const (
	DimensionMass   Dimension = "mass"
	DimensionVolume Dimension = "volume"
	DimensionCount  Dimension = "count"
)

var ErrInvalidQuantity = errors.New("invalid product quantity")

var quantityPattern = regexp.MustCompile(`(?i)^\s*(?:(\d+)\s*[x×]\s*)?(\d+(?:[.,]\d+)?)\s*(ml|millilit(?:er|re)s?|l|lit(?:er|re)s?|g|grams?|kg|kilograms?|count|ea|pcs?)\s*$`)

type Quantity struct {
	Numerator   int64
	Denominator int64
	Dimension   Dimension
	PackCount   int64
}

func (q Quantity) Validate() error {
	if q.Numerator <= 0 || q.Denominator <= 0 || q.PackCount <= 0 {
		return ErrInvalidQuantity
	}
	switch q.Dimension {
	case DimensionMass, DimensionVolume, DimensionCount:
		return nil
	default:
		return ErrInvalidQuantity
	}
}

func (q Quantity) SamePhysical(other Quantity) (bool, error) {
	if err := q.Validate(); err != nil {
		return false, err
	}
	if err := other.Validate(); err != nil {
		return false, err
	}
	if q.Dimension != other.Dimension || q.PackCount != other.PackCount {
		return false, nil
	}
	left := new(big.Int).Mul(big.NewInt(q.Numerator), big.NewInt(other.Denominator))
	right := new(big.Int).Mul(big.NewInt(other.Numerator), big.NewInt(q.Denominator))
	return left.Cmp(right) == 0, nil
}

// UnitCost retains the exact rational minor-unit cost for one canonical unit.
// Display rounding is intentionally left to the presentation layer.
type UnitCost struct {
	Numerator   *big.Int
	Denominator *big.Int
	Currency    string
	Scale       int
	Dimension   Dimension
}

func CalculateUnitCost(amount Money, quantity Quantity, minimumQty int) (UnitCost, error) {
	if err := amount.Validate(); err != nil {
		return UnitCost{}, err
	}
	if err := quantity.Validate(); err != nil || minimumQty < 1 || minimumQty > 10000 {
		return UnitCost{}, ErrInvalidQuantity
	}
	packQty := new(big.Int).Mul(big.NewInt(quantity.PackCount), big.NewInt(int64(minimumQty)))
	numerator := new(big.Int).Mul(big.NewInt(amount.Minor), big.NewInt(quantity.Denominator))
	denominator := new(big.Int).Mul(big.NewInt(quantity.Numerator), packQty)
	if numerator.Sign() <= 0 || denominator.Sign() <= 0 {
		return UnitCost{}, ErrInvalidQuantity
	}
	gcd := new(big.Int).GCD(nil, nil, numerator, denominator)
	numerator.Quo(numerator, gcd)
	denominator.Quo(denominator, gcd)
	return UnitCost{Numerator: numerator, Denominator: denominator, Currency: amount.Currency, Scale: amount.Scale, Dimension: quantity.Dimension}, nil
}

func (u UnitCost) Compare(other UnitCost) (int, error) {
	if u.Numerator == nil || u.Denominator == nil || other.Numerator == nil || other.Denominator == nil ||
		u.Numerator.Sign() <= 0 || u.Denominator.Sign() <= 0 || other.Numerator.Sign() <= 0 || other.Denominator.Sign() <= 0 {
		return 0, ErrInvalidQuantity
	}
	if u.Currency != other.Currency || u.Scale != other.Scale {
		return 0, ErrInvalidMoney
	}
	if u.Dimension != other.Dimension {
		return 0, ErrInvalidQuantity
	}
	left := new(big.Int).Mul(u.Numerator, other.Denominator)
	right := new(big.Int).Mul(other.Numerator, u.Denominator)
	return left.Cmp(right), nil
}

func (u UnitCost) RoundedMinor() (*big.Int, error) {
	if u.Numerator == nil || u.Denominator == nil || u.Numerator.Sign() <= 0 || u.Denominator.Sign() <= 0 {
		return nil, ErrInvalidQuantity
	}
	quotient, remainder := new(big.Int).QuoRem(u.Numerator, u.Denominator, new(big.Int))
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(u.Denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient, nil
}

func ParseQuantity(raw string) (Quantity, error) {
	if len(raw) == 0 || len(raw) > 128 {
		return Quantity{}, ErrInvalidQuantity
	}
	parts := quantityPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if parts == nil {
		return Quantity{}, ErrInvalidQuantity
	}
	packCount := int64(1)
	if parts[1] != "" {
		parsed, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || parsed <= 0 {
			return Quantity{}, ErrInvalidQuantity
		}
		packCount = parsed
	}
	value := strings.Replace(parts[2], ",", ".", 1)
	decimalParts := strings.Split(value, ".")
	if len(decimalParts) > 2 || len(decimalParts) == 2 && len(decimalParts[1]) > 3 {
		return Quantity{}, ErrInvalidQuantity
	}
	whole, err := strconv.ParseInt(decimalParts[0], 10, 64)
	if err != nil || whole < 0 {
		return Quantity{}, ErrInvalidQuantity
	}
	denominator := int64(1)
	fraction := int64(0)
	if len(decimalParts) == 2 {
		denominator, err = strconv.ParseInt("1"+strings.Repeat("0", len(decimalParts[1])), 10, 64)
		if err != nil || decimalParts[1] == "" {
			return Quantity{}, ErrInvalidQuantity
		}
		fraction, err = strconv.ParseInt(decimalParts[1], 10, 64)
		if err != nil || whole > (math.MaxInt64-fraction)/denominator {
			return Quantity{}, ErrInvalidQuantity
		}
	}
	whole = whole*denominator + fraction
	if whole <= 0 {
		return Quantity{}, ErrInvalidQuantity
	}
	unit := strings.ToLower(parts[3])
	dimension := DimensionCount
	baseUnit := int64(1)
	switch unit {
	case "ml", "milliliter", "milliliters", "millilitre", "millilitres":
		dimension, baseUnit = DimensionVolume, 1000
	case "l", "liter", "liters", "litre", "litres":
		dimension = DimensionVolume
	case "g", "gram", "grams":
		dimension, baseUnit = DimensionMass, 1000
	case "kg", "kilogram", "kilograms":
		dimension = DimensionMass
	case "count", "ea", "pc", "pcs":
		dimension = DimensionCount
	default:
		return Quantity{}, ErrInvalidQuantity
	}
	if denominator > math.MaxInt64/baseUnit {
		return Quantity{}, ErrInvalidQuantity
	}
	denominator *= baseUnit
	numerator := whole
	divisor := gcd(numerator, denominator)
	denominator /= divisor
	numerator /= divisor
	if dimension == DimensionCount && denominator != 1 {
		return Quantity{}, ErrInvalidQuantity
	}
	return Quantity{Numerator: numerator, Denominator: denominator, Dimension: dimension, PackCount: packCount}, nil
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
