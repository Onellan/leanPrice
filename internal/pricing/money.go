package pricing

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrInvalidMoney     = errors.New("invalid monetary amount")
	ErrCurrencyMismatch = errors.New("currency mismatch")
)

type Money struct {
	Minor    int64
	Currency string
	Scale    int
}

var currencyScales = map[string]int{
	"ZAR": 2, "USD": 2, "EUR": 2, "GBP": 2, "AUD": 2, "CAD": 2, "NZD": 2,
	"CHF": 2, "CNY": 2, "INR": 2, "SGD": 2, "HKD": 2,
	"JPY": 0, "KRW": 0,
	"BHD": 3, "KWD": 3, "OMR": 3, "TND": 3,
}

func MinorUnitScale(currency string) (int, bool) {
	scale, ok := currencyScales[strings.ToUpper(strings.TrimSpace(currency))]
	return scale, ok
}

func ParseMoney(raw, currency string) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	scale, supported := MinorUnitScale(currency)
	if !supported || len(currency) != 3 || !asciiLetters(currency) || len(raw) == 0 || len(raw) > 128 {
		return Money{}, ErrInvalidMoney
	}
	amount := strings.TrimSpace(raw)
	if currency == "ZAR" {
		upper := strings.ToUpper(amount)
		if strings.HasPrefix(upper, "ZAR") {
			amount = strings.TrimSpace(amount[3:])
		} else if strings.HasPrefix(upper, "R") {
			amount = strings.TrimSpace(amount[1:])
		}
	}
	var compact strings.Builder
	for _, r := range amount {
		if unicode.IsSpace(r) || r == '\u00a0' || r == '\u202f' || r == '\u2009' {
			continue
		}
		compact.WriteRune(r)
	}
	amount = compact.String()
	if amount == "" || strings.ContainsAny(amount, "+-eE") {
		return Money{}, ErrInvalidMoney
	}
	whole, fraction, ok := splitAmount(amount, currency, scale)
	if !ok {
		return Money{}, ErrInvalidMoney
	}
	if whole == "" {
		whole = "0"
	}
	if len(fraction) > scale {
		return Money{}, ErrInvalidMoney
	}
	if len(fraction) == 0 {
		fraction = strings.Repeat("0", scale)
	} else if len(fraction) < scale {
		fraction += strings.Repeat("0", scale-len(fraction))
	}
	wholeValue, err := strconv.ParseInt(whole, 10, 64)
	multiplier := int64(1)
	for i := 0; i < scale; i++ {
		multiplier *= 10
	}
	fractionValue := int64(0)
	fractionErr := error(nil)
	if fraction != "" {
		fractionValue, fractionErr = strconv.ParseInt(fraction, 10, 64)
	}
	if err != nil || fractionErr != nil || wholeValue > (math.MaxInt64-fractionValue)/multiplier {
		return Money{}, ErrInvalidMoney
	}
	minor := wholeValue*multiplier + fractionValue
	if minor <= 0 {
		return Money{}, ErrInvalidMoney
	}
	return Money{Minor: minor, Currency: currency, Scale: scale}, nil
}

func (m Money) Validate() error {
	scale, supported := MinorUnitScale(m.Currency)
	if m.Minor <= 0 || len(m.Currency) != 3 || !asciiLetters(m.Currency) || m.Currency != strings.ToUpper(m.Currency) || !supported || scale != m.Scale {
		return ErrInvalidMoney
	}
	return nil
}

func (m Money) Compare(other Money) (int, error) {
	if err := m.Validate(); err != nil {
		return 0, err
	}
	if err := other.Validate(); err != nil {
		return 0, err
	}
	if m.Currency != other.Currency {
		return 0, fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	if m.Minor < other.Minor {
		return -1, nil
	}
	if m.Minor > other.Minor {
		return 1, nil
	}
	return 0, nil
}

func splitAmount(amount, currency string, scale int) (string, string, bool) {
	if strings.ContainsAny(amount, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return "", "", false
	}
	comma := strings.LastIndexByte(amount, ',')
	dot := strings.LastIndexByte(amount, '.')
	if comma >= 0 && dot >= 0 {
		decimal := comma
		grouping := byte('.')
		if dot > comma {
			decimal = dot
			grouping = ','
		}
		whole := amount[:decimal]
		fraction := amount[decimal+1:]
		if !validDigits(fraction) || len(fraction) < 1 || len(fraction) > scale || !validGrouping(whole, grouping) {
			return "", "", false
		}
		return strings.ReplaceAll(whole, string(grouping), ""), fraction, true
	}
	separator := byte(0)
	if comma >= 0 {
		separator = ','
	} else if dot >= 0 {
		separator = '.'
	}
	if separator == 0 {
		if !validDigits(amount) {
			return "", "", false
		}
		return amount, "", true
	}
	parts := strings.Split(amount, string(separator))
	if len(parts) == 1 || len(parts[0]) == 0 {
		return "", "", false
	}
	if len(parts) > 2 {
		if !validGrouping(amount, separator) {
			return "", "", false
		}
		return strings.ReplaceAll(amount, string(separator), ""), "", true
	}
	whole, tail := parts[0], parts[1]
	if !validDigits(whole) || !validDigits(tail) {
		return "", "", false
	}
	if separator == ',' && currency != "ZAR" {
		if len(tail) == 3 && validGrouping(amount, separator) {
			return strings.ReplaceAll(amount, string(separator), ""), "", true
		}
		return "", "", false
	}
	if len(tail) == 3 {
		if scale == 3 && separator == '.' {
			return whole, tail, true
		}
		// A single three-digit separator is ambiguous between grouping and
		// fractional precision. Fail closed; use explicit space grouping.
		return "", "", false
	}
	if len(tail) > scale || scale == 0 {
		return "", "", false
	}
	return whole, tail, true
}

func validGrouping(value string, separator byte) bool {
	parts := strings.Split(value, string(separator))
	if len(parts) < 2 || len(parts[0]) < 1 || len(parts[0]) > 3 || !validDigits(parts[0]) {
		return false
	}
	for _, part := range parts[1:] {
		if len(part) != 3 || !validDigits(part) {
			return false
		}
	}
	return true
}

func validDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func asciiLetters(value string) bool {
	for _, c := range value {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
