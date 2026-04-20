// 遵循project_guide.md
package services

// bill_post_h_hardening_test.go — Phase H hardening slice H-hardening-1:
// row-level write lock on referenced receipt_lines during Bill matching.
//
// What the hardening fixes
// ------------------------
// Before H-hardening-1, two concurrent PostBill calls that both
// referenced the same receipt_line could both read `prior_matched`
// before either committed, producing a cumulative matched_qty that
// exceeds the receipt line's qty. Each computed its `available` on a
// stale snapshot and silently over-matched.
//
// Fix: resolveBillLineMatchingContext now loads the referenced
// receipt_line via `applyLockForUpdate(tx.Where(...))`, emitting
// SELECT ... FOR UPDATE on PostgreSQL. The second tx blocks on that
// lock until the first commits, then recomputes `prior_matched`
// against the first's now-committed bill line and correctly
// diminishes `available`.
//
// Testing on SQLite vs PostgreSQL
// -------------------------------
// SQLite's in-memory engine serialises all writes at the database
// level (BEGIN IMMEDIATE takes a global write lock), so the race
// this hardening defends against is structurally unreachable on
// SQLite — any test written here would pass with OR without the
// lock. The lock is verified on PostgreSQL by:
//
//   (1) Code-level review: resolveBillLineMatchingContext calls
//       applyLockForUpdate on the receipt_line fetch (bill_receipt_matching.go).
//       A regression that removes this call makes the file diff
//       visible in review.
//
//   (2) Pattern consistency: the idiom is identical to the locks
//       established in customer_credit_service.go,
//       gateway_dispute_service.go, bill_post.go (bill row lock) and
//       bill_void.go. An import-guard style check comparable to the
//       inventory-package rule could be added if regressions prove
//       common; for now the idiom + skip sentinel suffices.
//
//   (3) The sequential-cumulative test already locked in H.5
//       (TestPostBill_H5_CumulativePartialMatches_ClearGRIR) exercises
//       the matched-qty computation on the happy path. The lock
//       protects that computation's input under concurrent load; the
//       inputs themselves are the same.
//
// The skip-sentinel test below records the contract in test output.

import (
	"strings"
	"testing"
)

// TestBillLineMatching_ReceiptLineLocked_DocumentedOnSQLiteSkip is the
// H-hardening-1 sentinel test. It does not exercise a race on SQLite
// (impossible) but records the contract and fails loud if a future
// edit removes the FOR UPDATE clause from the matching path.
//
// The verification uses a file-contents probe, same mechanism as the
// inventory-package import-guard in receipt_service_test.go:
// TestInventoryPackage_DoesNotImportAccountingPackages. A code change
// that drops `applyLockForUpdate` from the receipt-line load inside
// resolveBillLineMatchingContext breaks this test at CI.
func TestBillLineMatching_ReceiptLineLocked_H_Hardening_1(t *testing.T) {
	const path = "bill_receipt_matching.go"
	data, err := osReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(data)

	// The FOR UPDATE wrap must appear on the receipt_line fetch. The
	// check is intentionally loose (looks for the call pattern inside
	// the resolveBillLineMatchingContext function body, not an exact
	// line match) so non-semantic reformatting doesn't trip it.
	idx := strings.Index(src, "func resolveBillLineMatchingContext")
	if idx < 0 {
		t.Fatalf("resolveBillLineMatchingContext not found in %s — function renamed?", path)
	}
	// Scan from the function opening to its closing brace. Approximate
	// by scanning forward to the next top-level `func ` declaration.
	rest := src[idx:]
	endIdx := strings.Index(rest[1:], "\nfunc ")
	body := rest
	if endIdx >= 0 {
		body = rest[:endIdx+1]
	}
	if !strings.Contains(body, "applyLockForUpdate(") {
		t.Fatalf("resolveBillLineMatchingContext body missing applyLockForUpdate — concurrent over-match race returns. See H-hardening-1 contract.")
	}
	// Belt + suspenders: ensure the lock applies to the receipt_line
	// fetch specifically (not some unrelated call). The lock wrapper
	// sits on a clause that filters by id==receipt_line_id.
	if !strings.Contains(body, "Where(\"id = ?\", *line.ReceiptLineID)") {
		t.Fatalf("receipt_line filter pattern changed — re-check that applyLockForUpdate still wraps the correct query")
	}
	// Cannot reproduce the race on SQLite. The record stays in test
	// output so future contributors see why this file exists.
	t.Skip("FOR UPDATE lock verified by code-level probe above; the race it defends against is structurally unreachable on SQLite (global writer lock). Real concurrency behavior on PostgreSQL is inherited from applyLockForUpdate's established usage pattern (customer_credit_service, gateway_dispute_service, bill_post).")
}
