package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestSplitAbsorbsRoundingResidualInLargestShare(t *testing.T) {
	total, err := ParseMoney("0.10")
	if err != nil {
		t.Fatalf("parse total: %v", err)
	}
	parts, residual, err := SplitByBasisPoints(total, []Allocation{
		{Label: "community", BasisPoints: 6000},
		{Label: "project", BasisPoints: 3500},
		{Label: "verifier", BasisPoints: 500},
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if residual.IsZero() {
		t.Fatal("this split should produce rounding drift for the largest share to absorb")
	}
	var sum decimal.Decimal
	for _, part := range parts {
		sum = sum.Add(part)
	}
	if !sum.Equal(total) {
		t.Fatalf("split lost value: parts sum %s, total %s, residual %s", sum.StringFixed(MoneyScale), total.StringFixed(MoneyScale), residual)
	}
	// 0.06 exact, then 0.04 and 0.01 rounded up: the 0.01 overdraw lands on the
	// largest share rather than vanishing from the ledger.
	if parts[0].StringFixed(MoneyScale) != "0.05" {
		t.Fatalf("community share unexpected after residual absorption: %s", parts[0].StringFixed(MoneyScale))
	}
}

func TestSplitRejectsIncompleteBasisPoints(t *testing.T) {
	if _, _, err := SplitByBasisPoints(One, []Allocation{{BasisPoints: 6000}, {BasisPoints: 3000}}); err == nil {
		t.Fatal("expected an error when allocations do not total 10000 basis points")
	}
}

func TestQuantityAndMoneyScales(t *testing.T) {
	quantity, err := ParseQuantity("12.3456789")
	if err != nil {
		t.Fatalf("parse quantity: %v", err)
	}
	if Quantity(quantity) != "12.345679" {
		t.Fatalf("quantity scale: %s", Quantity(quantity))
	}
	cash, err := ParseMoney(" 1200.5 ")
	if err != nil {
		t.Fatalf("parse money: %v", err)
	}
	if Cash(cash) != "1200.50" {
		t.Fatalf("money scale: %s", Cash(cash))
	}
	if Total(cash, quantity).StringFixed(MoneyScale) != "14820.99" {
		t.Fatalf("unexpected total: %s", Total(cash, quantity))
	}
}

func TestParseRejectsNonDecimalInput(t *testing.T) {
	for _, input := range []string{"", "1e5", "12.5 USD", "1,000", "-1.0", "1/2"} {
		if _, err := ParseQuantity(input); err == nil {
			t.Errorf("quantity %q accepted", input)
		}
		if _, err := ParseMoney(input); err == nil {
			t.Errorf("money %q accepted", input)
		}
	}
}
