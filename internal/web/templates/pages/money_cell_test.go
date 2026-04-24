// 遵循project_guide.md
package pages

import (
	"context"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// TestRenderMoneyCell_LinkedRendersAnchor locks the contract that a
// non-empty DrillURL renders as a clickable <a> with the primary-color
// classes. The summary report templates rely on this — change here
// breaks BS / IS / TB / Aging visual.
func TestRenderMoneyCell_LinkedRendersAnchor(t *testing.T) {
	cell := NewMoneyCell(decimal.NewFromInt(1234), "/reports/account-transactions?account_id=42")

	var sb strings.Builder
	if err := RenderMoneyCell(cell).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()

	for _, want := range []string{
		"<a ",
		"href=\"/reports/account-transactions?account_id=42\"",
		"text-primary",
		"data-numfmt", // global num-format.js must still apply
		"1234.00",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q\nfull output: %s", want, html)
		}
	}
}

// TestRenderMoneyCell_NoURLRendersPlainSpan verifies that cells with
// no drill target (Net Income, Total Assets, etc.) render as a plain
// <span> — no anchor tag, no hover styling — so the operator doesn't
// see a "link" that goes nowhere.
func TestRenderMoneyCell_NoURLRendersPlainSpan(t *testing.T) {
	cell := NewMoneyCell(decimal.NewFromInt(99), "")

	var sb strings.Builder
	if err := RenderMoneyCell(cell).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()

	if strings.Contains(html, "<a ") {
		t.Errorf("expected no anchor for empty URL, got: %s", html)
	}
	if !strings.Contains(html, "<span") {
		t.Errorf("expected <span>, got: %s", html)
	}
	if !strings.Contains(html, "99.00") {
		t.Errorf("expected amount 99.00 in output: %s", html)
	}
}

// TestRenderMoneyCellBlank_ZeroAmountRendersNothing locks the Trial
// Balance convention: the Debit / Credit columns each render only when
// populated (one or the other per row, never both).
func TestRenderMoneyCellBlank_ZeroAmountRendersNothing(t *testing.T) {
	cell := NewMoneyCell(decimal.Zero, "/reports/account-transactions?account_id=1")

	var sb strings.Builder
	if err := RenderMoneyCellBlank(cell).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	html := sb.String()

	if strings.Contains(html, "0.00") {
		t.Errorf("expected blank for zero amount, got: %s", html)
	}
	if strings.Contains(html, "<a ") {
		t.Errorf("expected no anchor for zero amount, got: %s", html)
	}
}
