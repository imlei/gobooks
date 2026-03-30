package services

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAggregateJournalLines_mergesSameAccountSameSide(t *testing.T) {
	frags := []PostingFragment{
		{AccountID: 10, Debit: d("100.00"), Credit: d("0"), Memo: "a"},
		{AccountID: 10, Debit: d("50.00"), Credit: d("0"), Memo: "b"},
		{AccountID: 20, Debit: d("0"), Credit: d("150.00"), Memo: "c"},
	}
	got, err := AggregateJournalLines(frags)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d", len(got))
	}
	var tenDebit, twentyCredit *PostingFragment
	for i := range got {
		switch got[i].AccountID {
		case 10:
			tenDebit = &got[i]
		case 20:
			twentyCredit = &got[i]
		}
	}
	if tenDebit == nil || !tenDebit.Debit.Equal(d("150.00")) || tenDebit.Credit.Sign() != 0 {
		t.Fatalf("account 10 debit: %+v", tenDebit)
	}
	if twentyCredit == nil || !twentyCredit.Credit.Equal(d("150.00")) {
		t.Fatalf("account 20 credit: %+v", twentyCredit)
	}
}

func TestAggregateJournalLines_separateDebitAndCreditSameAccount(t *testing.T) {
	// Same GL account should NOT merge debit with credit (different legs).
	frags := []PostingFragment{
		{AccountID: 10, Debit: d("100.00"), Credit: d("0"), Memo: ""},
		{AccountID: 10, Debit: d("0"), Credit: d("40.00"), Memo: ""},
	}
	got, err := AggregateJournalLines(frags)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d %+v", len(got), got)
	}
}

func TestAggregateJournalLines_rejectsBothSides(t *testing.T) {
	_, err := AggregateJournalLines([]PostingFragment{
		{AccountID: 1, Debit: d("10"), Credit: d("5")},
	})
	if err != ErrInvalidPostingFragment {
		t.Fatalf("want ErrInvalidPostingFragment, got %v", err)
	}
}

func TestSalesAggregation_mergeRevenueAndTaxLines(t *testing.T) {
	// Two revenue lines to same account; two tax credits to same payable account → two merged credits.
	frags := []PostingFragment{
		{AccountID: 1, Debit: d("1155.00"), Credit: d("0"), Memo: "AR"},
		{AccountID: 10, Debit: d("0"), Credit: d("1000.00"), Memo: "A"},
		{AccountID: 10, Debit: d("0"), Credit: d("100.00"), Memo: "B"},
		{AccountID: 99, Debit: d("0"), Credit: d("50.00"), Memo: "tax A"},
		{AccountID: 99, Debit: d("0"), Credit: d("5.00"), Memo: "tax B"},
	}
	got, err := AggregateJournalLines(frags)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 aggregated lines, got %d %+v", len(got), got)
	}
	var rev, tax *PostingFragment
	for i := range got {
		switch got[i].AccountID {
		case 10:
			rev = &got[i]
		case 99:
			tax = &got[i]
		}
	}
	if rev == nil || !rev.Credit.Equal(d("1100.00")) {
		t.Fatalf("revenue merge: %+v", rev)
	}
	if tax == nil || !tax.Credit.Equal(d("55.00")) {
		t.Fatalf("tax merge: %+v", tax)
	}
}

func TestPurchaseAggregation_mergeExpenseSameAccount(t *testing.T) {
	frags := []PostingFragment{
		{AccountID: 50, Debit: d("1.07"), Credit: d("0"), Memo: "pen"},
		{AccountID: 50, Debit: d("10.70"), Credit: d("0"), Memo: "paper"},
		{AccountID: 60, Debit: d("1070.00"), Credit: d("0"), Memo: "printer"},
		{AccountID: 2, Debit: d("0"), Credit: d("1081.77"), Memo: "AP"},
	}
	got, err := AggregateJournalLines(frags)
	if err != nil {
		t.Fatal(err)
	}
	var office *PostingFragment
	for i := range got {
		if got[i].AccountID == 50 {
			office = &got[i]
			break
		}
	}
	if office == nil || !office.Debit.Equal(d("11.77")) {
		t.Fatalf("office supplies merged debit: %+v", office)
	}
}

func TestAggregateJournalLines_differentAccountsStaySeparate(t *testing.T) {
	// Two debit fragments on different accounts must never be merged, even though
	// they are on the same side (debit). This verifies the "different accounts do
	// not merge" invariant: merging is keyed on (account_id, side), not side alone.
	frags := []PostingFragment{
		{AccountID: 10, Debit: d("300.00"), Credit: d("0"), Memo: "office supplies"},
		{AccountID: 20, Debit: d("700.00"), Credit: d("0"), Memo: "equipment"},
		{AccountID: 99, Debit: d("0"), Credit: d("1000.00"), Memo: "AP"},
	}
	got, err := AggregateJournalLines(frags)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 lines (accounts kept separate), got %d: %+v", len(got), got)
	}
	var acct10, acct20 *PostingFragment
	for i := range got {
		switch got[i].AccountID {
		case 10:
			acct10 = &got[i]
		case 20:
			acct20 = &got[i]
		}
	}
	if acct10 == nil || !acct10.Debit.Equal(d("300.00")) {
		t.Fatalf("account 10 debit: %+v", acct10)
	}
	if acct20 == nil || !acct20.Debit.Equal(d("700.00")) {
		t.Fatalf("account 20 debit: %+v", acct20)
	}
}

func TestAggregateJournalLines_dropsZeros(t *testing.T) {
	got, err := AggregateJournalLines([]PostingFragment{
		{AccountID: 1, Debit: decimal.Zero, Credit: decimal.Zero},
		{AccountID: 2, Debit: d("1.00"), Credit: decimal.Zero},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AccountID != 2 {
		t.Fatalf("got %+v", got)
	}
}
