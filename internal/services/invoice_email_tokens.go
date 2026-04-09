// 遵循project_guide.md
package services

// invoice_email_tokens.go — Controlled token substitution for invoice email subject/body.
//
// Design principles:
//   - Whitelist-only: only the tokens defined in EmailTokenData are substituted.
//   - Backend-rendered: tokens are resolved server-side from invoice snapshot fields
//     and company data, never from user-controlled template strings evaluated as code.
//   - No HTML: tokens are substituted into plain-text email body only.
//   - Deterministic: given the same invoice state, the same output is always produced.
//
// Supported tokens:
//   {{CompanyName}}    — company display name
//   {{CustomerName}}   — customer name from invoice snapshot
//   {{InvoiceNumber}}  — invoice number
//   {{InvoiceDate}}    — invoice date formatted as "January 2, 2006"
//   {{DueDate}}        — due date formatted as "January 2, 2006" (empty string if not set)
//   {{BalanceDue}}     — balance due formatted to 2 decimal places
//   {{Amount}}         — invoice total amount formatted to 2 decimal places
//   {{Currency}}       — invoice currency code (e.g. "CAD"), or company base currency

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// EmailTokenData holds the resolved values for all supported email tokens.
// All fields are sourced from invoice snapshots or company data — never from
// user-editable free-form fields that could contain injection content.
type EmailTokenData struct {
	CompanyName    string
	CustomerName   string
	InvoiceNumber  string
	InvoiceDate    time.Time
	DueDate        *time.Time
	BalanceDue     decimal.Decimal
	Amount         decimal.Decimal
	Currency       string // e.g. "CAD"; falls back to company base currency
}

// tokenMap builds the substitution map from a TokenData struct.
// Keys are the exact token strings as they appear in templates.
func (d EmailTokenData) tokenMap() map[string]string {
	dueDate := ""
	if d.DueDate != nil {
		dueDate = d.DueDate.Format("January 2, 2006")
	}
	curr := d.Currency
	if curr == "" {
		curr = "CAD"
	}
	return map[string]string{
		"{{CompanyName}}":   d.CompanyName,
		"{{CustomerName}}":  d.CustomerName,
		"{{InvoiceNumber}}": d.InvoiceNumber,
		"{{InvoiceDate}}":   d.InvoiceDate.Format("January 2, 2006"),
		"{{DueDate}}":       dueDate,
		"{{BalanceDue}}":    d.BalanceDue.StringFixed(2),
		"{{Amount}}":        d.Amount.StringFixed(2),
		"{{Currency}}":      curr,
	}
}

// RenderEmailTokens substitutes whitelisted tokens in subject and body strings.
// Unknown tokens (not in the whitelist) are left verbatim — they are not evaluated
// or removed, which makes template errors visible rather than silently swallowed.
//
// Both subject and body are processed independently and returned in the same order.
func RenderEmailTokens(subject, body string, data EmailTokenData) (string, string) {
	tokens := data.tokenMap()
	for token, value := range tokens {
		subject = strings.ReplaceAll(subject, token, value)
		body = strings.ReplaceAll(body, token, value)
	}
	return subject, body
}

// DefaultEmailSubject returns the standard fallback subject when no template subject is set.
func DefaultEmailSubject(invoiceNumber string) string {
	return "Invoice #" + invoiceNumber
}

// DefaultEmailBody returns the standard fallback body when no template body is set.
// Uses the same token placeholders so callers can pass it through RenderEmailTokens.
// Pass attachPDF=true to include "Please find your invoice attached.";
// pass false for neutral wording when no PDF is sent.
func DefaultEmailBody(attachPDF bool) string {
	intro := "Please review your invoice details below."
	if attachPDF {
		intro = "Please find your invoice attached."
	}
	return "Dear {{CustomerName}},\n\nThank you for your business. " + intro + "\n\nInvoice #: {{InvoiceNumber}}\nDate: {{InvoiceDate}}\nAmount Due: {{Currency}} {{BalanceDue}}{{DueDateLine}}\n\nPlease remit payment to the address listed on the invoice.\n\nThank you!"
}

// DefaultEmailBodyRendered returns the fully-rendered default body (no token placeholders).
// DueDateLine is handled specially here since it's a conditional line.
// Pass attachPDF=true to include "Please find your invoice attached.";
// pass false for neutral wording when no PDF is sent.
func DefaultEmailBodyRendered(data EmailTokenData, attachPDF bool) string {
	intro := "Please review your invoice details below."
	if attachPDF {
		intro = "Please find your invoice attached."
	}

	body := "Dear {{CustomerName}},\n\nThank you for your business. " + intro + "\n\nInvoice #: {{InvoiceNumber}}\nDate: {{InvoiceDate}}\nAmount Due: {{Currency}} {{BalanceDue}}"

	if data.DueDate != nil {
		body += "\nDue Date: {{DueDate}}"
	}
	body += "\n\nPlease remit payment to the address listed on the invoice.\n\nThank you!"

	_, rendered := RenderEmailTokens("", body, data)
	return rendered
}
