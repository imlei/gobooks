// 遵循产品需求 v1.0
package services

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseDecimalMoney_commas(t *testing.T) {
	d, err := ParseDecimalMoney("1,234.56")
	if err != nil {
		t.Fatal(err)
	}
	exp, _ := decimal.NewFromString("1234.56")
	if !d.Equal(exp) {
		t.Fatalf("got %s", d.String())
	}
}

func TestParseParty(t *testing.T) {
	pt, id, err := ParseParty("customer:42")
	if err != nil || id != 42 {
		t.Fatalf("got %v %d %v", pt, id, err)
	}
	_, _, err = ParseParty("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}
