// 遵循产品需求 v1.0
package ui

// TableCellVM represents a cell value and an optional alignment hint.
type TableCellVM struct {
	Value      string
	AlignRight bool
}

// TableRowVM represents one table row.
type TableRowVM struct {
	Cells []TableCellVM
}

// TableVM is a generic view model for reusable tables.
type TableVM struct {
	Headers []string
	Rows    []TableRowVM
}

