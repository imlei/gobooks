package services

import (
	"os"
	"strings"
	"testing"
)

func TestRecordPayBillsLocksBillsBeforeSettlement(t *testing.T) {
	bodyBytes, err := os.ReadFile("pay_bills.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(bodyBytes)
	for _, want := range []string{
		"sort.Slice(payments",
		"seenBills := make(map[uint]struct{}",
		"bill %d was selected more than once",
		"applyLockForUpdate(",
		`tx.Where("id = ? AND company_id = ?", bp.BillID, in.CompanyID)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("RecordPayBills missing concurrency guard %q", want)
		}
	}
}
