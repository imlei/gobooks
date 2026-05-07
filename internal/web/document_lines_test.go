package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"balanciz/internal/services"
)

func TestParseDocumentLinesSkipsDefaultBlankRows(t *testing.T) {
	app := fiber.New()
	var got []documentLine
	app.Post("/", func(c *fiber.Ctx) error {
		got = parseDocumentLines(c)
		return nil
	})

	form := url.Values{}
	form.Set("line_qty_0", "1")
	form.Set("line_price_0", "0.00")
	form.Set("line_product_service_id_1", "7")
	form.Set("line_description_1", "Test Parts")
	form.Set("line_qty_1", "1")
	form.Set("line_price_1", "12.00")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := app.Test(req); err != nil {
		t.Fatalf("submit form: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("lines: got %d want 1", len(got))
	}
	if got[0].ProductServiceID == nil || *got[0].ProductServiceID != 7 {
		t.Fatalf("product id: got %v want 7", got[0].ProductServiceID)
	}
	if !got[0].UnitPrice.Equal(decimal.RequireFromString("12.00")) {
		t.Fatalf("unit price: got %s want 12.00", got[0].UnitPrice)
	}
}

func TestParsePOInputSkipsDefaultBlankRows(t *testing.T) {
	app := fiber.New()
	var got []services.POLineInput
	app.Post("/", func(c *fiber.Ctx) error {
		in, err := parsePOInput(c)
		if err != nil {
			t.Fatalf("parse PO input: %v", err)
		}
		got = in.Lines
		return nil
	})

	form := url.Values{}
	form.Set("vendor_id", "3")
	form.Set("po_date", "2026-05-07")
	form.Set("lines[0][product_service_id]", "7")
	form.Set("lines[0][description]", "Computer 1")
	form.Set("lines[0][qty]", "1")
	form.Set("lines[0][unit_price]", "150.00")
	form.Set("lines[1][product_service_id]", "")
	form.Set("lines[1][expense_account_id]", "")
	form.Set("lines[1][description]", "")
	form.Set("lines[1][qty]", "1.00")
	form.Set("lines[1][unit_price]", "0.00")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := app.Test(req); err != nil {
		t.Fatalf("submit form: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("lines: got %d want 1", len(got))
	}
	if got[0].ProductServiceID == nil || *got[0].ProductServiceID != 7 {
		t.Fatalf("product id: got %v want 7", got[0].ProductServiceID)
	}
	if got[0].SortOrder != 1 {
		t.Fatalf("sort order: got %d want 1", got[0].SortOrder)
	}
	if !got[0].UnitPrice.Equal(decimal.RequireFromString("150.00")) {
		t.Fatalf("unit price: got %s want 150.00", got[0].UnitPrice)
	}
}
