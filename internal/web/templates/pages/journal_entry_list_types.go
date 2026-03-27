// 遵循产品需求 v1.0
package pages

type JournalEntryListItem struct {
	ID        uint
	EntryDate string
	JournalNo string
	LineCount   int
	TotalDebit  string
	TotalCredit string
	CanReverse  bool
	ReverseHint string
}

type JournalEntryListVM struct {
	HasCompany bool
	Active     string
	Items      []JournalEntryListItem
	FormError  string
	Reversed   bool
}

