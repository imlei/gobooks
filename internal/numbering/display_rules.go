// Package numbering holds business display numbering configuration (prefix, sequence, padding).
// It does not manage internal immutable entity numbers (e.g. EN…); those are backend-only.
package numbering

import (
	"fmt"
	"strings"
)

// Module keys are stable identifiers for routing, storage, and future APIs.
const (
	ModuleJournalEntry = "journal_entry"
	ModuleInvoice      = "invoice"
	ModulePayment      = "payment"
	ModuleCustomer     = "customer"
	ModuleVendor       = "vendor"
)

// DisplayRule describes how user-visible document/reference numbers are formatted for one module.
// This is separate from internal database identifiers and entity_number values.
type DisplayRule struct {
	ModuleKey       string `json:"module_key"`
	ModuleName      string `json:"module_name"`
	Prefix          string `json:"prefix"`
	NextNumber      int    `json:"next_number"`
	PaddingLength   int    `json:"padding_length"`
	Enabled         bool   `json:"enabled"`
}

// DefaultDisplayRules returns the built-in defaults when no file exists yet.
func DefaultDisplayRules() []DisplayRule {
	return []DisplayRule{
		{ModuleKey: ModuleJournalEntry, ModuleName: "Journal Entry", Prefix: "JE-", NextNumber: 1, PaddingLength: 4, Enabled: true},
		{ModuleKey: ModuleInvoice, ModuleName: "Invoice", Prefix: "INV-", NextNumber: 1, PaddingLength: 4, Enabled: true},
		{ModuleKey: ModulePayment, ModuleName: "Payment", Prefix: "PMT-", NextNumber: 1, PaddingLength: 4, Enabled: true},
		{ModuleKey: ModuleCustomer, ModuleName: "Customer", Prefix: "CUST-", NextNumber: 1, PaddingLength: 4, Enabled: true},
		{ModuleKey: ModuleVendor, ModuleName: "Vendor", Prefix: "VEN-", NextNumber: 1, PaddingLength: 4, Enabled: true},
	}
}

// FormatPreview builds a sample display number from prefix + padded next value.
func FormatPreview(prefix string, next int, padding int) string {
	prefix = strings.TrimSpace(prefix)
	if padding <= 0 {
		return prefix + fmt.Sprintf("%d", next)
	}
	if padding > 32 {
		padding = 32
	}
	return prefix + fmt.Sprintf("%0*d", padding, next)
}

// NormalizeRule clamps values to safe ranges for storage and UI.
func NormalizeRule(r DisplayRule) DisplayRule {
	if r.NextNumber < 0 {
		r.NextNumber = 0
	}
	if r.PaddingLength < 0 {
		r.PaddingLength = 0
	}
	if r.PaddingLength > 20 {
		r.PaddingLength = 20
	}
	if len(r.Prefix) > 64 {
		r.Prefix = r.Prefix[:64]
	}
	return r
}
