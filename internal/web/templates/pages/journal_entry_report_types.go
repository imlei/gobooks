// 遵循产品需求 v1.0
package pages

import "gobooks/internal/services"

// JournalEntryReportVM is the view-model for Reports → Journal Entries.
type JournalEntryReportVM struct {
	HasCompany bool

	From string
	To   string

	ActiveTab string

	Entries []services.JournalEntryReportEntry

	FormError string
}
