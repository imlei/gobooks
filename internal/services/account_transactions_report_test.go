// 遵循project_guide.md
package services

import "testing"

// TestTransactionTypeLabel locks the source-type → display-label
// mapping. Unmapped values must read as "Journal Entry" (not blank)
// so the report's Type column never renders empty.
func TestTransactionTypeLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"invoice", "Invoice"},
		{"bill", "Bill"},
		{"expense", "Expense"},
		{"receipt", "Receipt"},
		{"customer_receipt", "Receipt"},
		{"credit_note", "Credit Memo"},
		{"vendor_credit_note", "Vendor Credit"},
		{"ar_refund", "Customer Refund"},
		{"vendor_refund", "Vendor Refund"},
		{"customer_deposit", "Customer Deposit"},
		{"vendor_prepayment", "Vendor Prepayment"},
		{"reversal", "Reversal"},
		{"opening_balance", "Opening Balance"},
		{"", "Journal Entry"},                 // manual JE
		{"revaluation", "Journal Entry"},      // unmapped → JE fallback
		{"payment_gateway", "Journal Entry"},  // unmapped → JE fallback
		{"made_up_garbage", "Journal Entry"},  // unknown → JE fallback
	}
	for _, tc := range cases {
		if got := transactionTypeLabel(tc.in); got != tc.want {
			t.Errorf("transactionTypeLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestDocumentURL locks the drill-down URL contract. Source types with
// a dedicated detail page route there; everything else falls back to
// the JE detail page so every row in the report is clickable.
func TestDocumentURL(t *testing.T) {
	cases := []struct {
		name           string
		sourceType     string
		sourceID       uint
		journalEntryID uint
		want           string
	}{
		{"invoice", "invoice", 42, 100, "/invoices/42"},
		{"bill", "bill", 17, 200, "/bills/17"},
		{"expense routes to /edit (no detail page)", "expense", 9, 300, "/expenses/9/edit"},
		{"receipt", "receipt", 5, 400, "/receipts/5"},
		{"customer_receipt", "customer_receipt", 6, 500, "/receipts/6"},
		{"credit_note", "credit_note", 22, 600, "/credit-notes/22"},
		{"vendor_credit_note", "vendor_credit_note", 23, 700, "/vendor-credit-notes/23"},
		{"ar_refund", "ar_refund", 31, 800, "/refunds/31"},
		{"vendor_refund", "vendor_refund", 32, 900, "/vendor-refunds/32"},
		{"customer_deposit", "customer_deposit", 11, 1000, "/deposits/11"},
		{"vendor_prepayment", "vendor_prepayment", 12, 1100, "/vendor-prepayments/12"},
		{"manual JE (empty source)", "", 0, 1200, "/journal-entry/1200"},
		{"reversal falls back to JE", "reversal", 50, 1300, "/journal-entry/1300"},
		{"unmapped falls back to JE", "made_up", 99, 1400, "/journal-entry/1400"},
		{"sourceID=0 falls back to JE even with known type", "invoice", 0, 1500, "/journal-entry/1500"},
		{"both zero returns empty", "", 0, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := documentURL(tc.sourceType, tc.sourceID, tc.journalEntryID); got != tc.want {
				t.Errorf("documentURL(%q, %d, %d) = %q, want %q",
					tc.sourceType, tc.sourceID, tc.journalEntryID, got, tc.want)
			}
		})
	}
}

// TestSourceTable verifies the (table, column) mapping used by
// loadSourceDocumentNumbers. Unmapped types must return ("","") so the
// helper skips them rather than running a query against a non-existent
// table.
func TestSourceTable(t *testing.T) {
	cases := []struct {
		sourceType string
		wantTable  string
		wantCol    string
	}{
		{"invoice", "invoices", "invoice_number"},
		{"bill", "bills", "bill_number"},
		{"expense", "expenses", "expense_number"},
		{"receipt", "receipts", "receipt_number"},
		{"customer_receipt", "customer_receipts", "receipt_number"},
		{"credit_note", "credit_notes", "credit_note_number"},
		{"vendor_credit_note", "vendor_credit_notes", "credit_note_number"},
		{"ar_refund", "ar_refunds", "refund_number"},
		{"vendor_refund", "vendor_refunds", "refund_number"},
		{"customer_deposit", "customer_deposits", "deposit_number"},
		{"vendor_prepayment", "vendor_prepayments", "prepayment_number"},
		// Unmapped source types must return ("","") — the helper relies
		// on that to skip the query entirely.
		{"", "", ""},
		{"reversal", "", ""},
		{"opening_balance", "", ""},
		{"revaluation", "", ""},
		{"payment_gateway", "", ""},
		{"ar_return_receipt", "", ""},  // sourceID points at receipt, not return — skip
	}
	for _, tc := range cases {
		gotTable, gotCol := sourceTable(tc.sourceType)
		if gotTable != tc.wantTable || gotCol != tc.wantCol {
			t.Errorf("sourceTable(%q) = (%q, %q), want (%q, %q)",
				tc.sourceType, gotTable, gotCol, tc.wantTable, tc.wantCol)
		}
	}
}

// TestFormatID + TestDocPartyKeys lock the tiny string helpers that
// the report's drill-down map keys depend on.
func TestFormatID(t *testing.T) {
	cases := []struct {
		prefix string
		id     uint
		want   string
	}{
		{"/invoices/", 42, "/invoices/42"},
		{"/bills/", 1, "/bills/1"},
		{"/foo/", 0, ""}, // id=0 → empty (caller falls back)
		{"/x/", 1234567890, "/x/1234567890"},
	}
	for _, tc := range cases {
		if got := formatID(tc.prefix, tc.id); got != tc.want {
			t.Errorf("formatID(%q, %d) = %q, want %q", tc.prefix, tc.id, got, tc.want)
		}
	}
}

func TestDocPartyKeys(t *testing.T) {
	if got := docKey("invoice", 42); got != "invoice:42" {
		t.Errorf("docKey = %q, want invoice:42", got)
	}
	if got := docKey("invoice", 0); got != "" {
		t.Errorf("docKey with zero id should be empty, got %q", got)
	}
	if got := partyKey("customer", 7); got != "customer:7" {
		t.Errorf("partyKey = %q, want customer:7", got)
	}
	if got := partyKey("", 7); got != "" {
		t.Errorf("partyKey with empty type should be empty, got %q", got)
	}
}
