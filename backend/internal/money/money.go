// Package money centralises fixed-precision arithmetic. Carbon quantities and
// currency never touch float64, and every persisted value is a canonical string.
package money

import (
	"fmt"
	"regexp"

	"github.com/shopspring/decimal"
)

// decimalLiteral is the only accepted wire form. shopspring also parses exponents,
// thousands separators and expressions, and none of those may reach a ledger.
var decimalLiteral = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)

const (
	// QuantityScale is the registry precision for tonnes of CO2e.
	QuantityScale = 6
	// MoneyScale is the settlement precision for currency amounts.
	MoneyScale = 2
	// BasisPointsWhole is the denominator for revenue split rules.
	BasisPointsWhole = 10000
)

var (
	Zero    = decimal.Zero
	One     = decimal.NewFromInt(1)
	Hundred = decimal.NewFromInt(100)
)

// ParseQuantity accepts a decimal string such as "12.5" and normalises it to the
// registry scale. Anything else (expressions, currency symbols, exponents) fails.
func ParseQuantity(text string) (decimal.Decimal, error) {
	amount, err := parse(text)
	if err != nil {
		return Zero, fmt.Errorf("quantity %q is not a decimal number", text)
	}
	if amount.IsNegative() {
		return Zero, fmt.Errorf("quantity must be positive")
	}
	return amount.Round(QuantityScale), nil
}

// ParseMoney accepts a currency amount and normalises it to settlement precision.
func ParseMoney(text string) (decimal.Decimal, error) {
	amount, err := parse(text)
	if err != nil {
		return Zero, fmt.Errorf("amount %q is not a decimal number", text)
	}
	if amount.IsNegative() {
		return Zero, fmt.Errorf("amount must be positive")
	}
	return amount.Round(MoneyScale), nil
}

// Quantity renders a tonnage for persistence or display.
func Quantity(amount decimal.Decimal) string {
	return amount.StringFixed(QuantityScale)
}

// Cash renders a currency amount for persistence or display.
func Cash(amount decimal.Decimal) string {
	return amount.StringFixed(MoneyScale)
}

// Total is price multiplied by quantity, rounded once at settlement precision.
func Total(unitPrice, quantity decimal.Decimal) decimal.Decimal {
	return unitPrice.Mul(quantity).Round(MoneyScale)
}

// Share applies a basis-point rate to a total, rounded at settlement precision.
func Share(total decimal.Decimal, basisPoints int) decimal.Decimal {
	return total.Mul(decimal.NewFromInt(int64(basisPoints))).Div(decimal.NewFromInt(BasisPointsWhole)).Round(MoneyScale)
}

// Allocation is one labelled slice of a revenue split.
type Allocation struct {
	Label       string
	BasisPoints int
}

// SplitByBasisPoints divides total across the allocations so the parts sum back to
// total exactly. Rounding drift is absorbed by the largest share, never dropped.
func SplitByBasisPoints(total decimal.Decimal, allocations []Allocation) ([]decimal.Decimal, decimal.Decimal, error) {
	var points int
	for _, allocation := range allocations {
		if allocation.BasisPoints < 0 {
			return nil, Zero, fmt.Errorf("basis points cannot be negative")
		}
		points += allocation.BasisPoints
	}
	if points != BasisPointsWhole {
		return nil, Zero, fmt.Errorf("revenue split must total 10000 basis points, got %d", points)
	}
	parts := make([]decimal.Decimal, len(allocations))
	var assigned decimal.Decimal
	largest := 0
	for index, allocation := range allocations {
		parts[index] = Share(total, allocation.BasisPoints)
		assigned = assigned.Add(parts[index])
		if allocation.BasisPoints > allocations[largest].BasisPoints {
			largest = index
		}
	}
	residual := total.Sub(assigned)
	parts[largest] = parts[largest].Add(residual)
	return parts, residual, nil
}

// Amount is the wire shape for a fixed-precision currency value. Clients render
// it as text; it never becomes a float in transit.
type Amount struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func NewAmount(value decimal.Decimal, currency string) Amount {
	return Amount{Amount: Cash(value), Currency: currency}
}

// Percent converts a basis-point rate into a whole-number percentage string.
func Percent(basisPoints int) string {
	return decimal.NewFromInt(int64(basisPoints)).Div(decimal.NewFromInt(100)).StringFixed(2)
}

func trim(text string) string {
	out := make([]rune, 0, len(text))
	for _, symbol := range text {
		if symbol == ' ' || symbol == '\t' {
			continue
		}
		out = append(out, symbol)
	}
	return string(out)
}

func parse(text string) (decimal.Decimal, error) {
	cleaned := trim(text)
	if !decimalLiteral.MatchString(cleaned) {
		return Zero, fmt.Errorf("%q is not a plain decimal literal", text)
	}
	return decimal.NewFromString(cleaned)
}
