// 遵循project_guide.md
package pdf

// values.go — runtime data the renderer feeds into a Schema.
//
// The pdf package is intentionally agnostic about WHERE values come from
// (Invoice / Quote / Bill / etc.). Each document-type adapter (lives in
// services/, e.g. services/invoice_pdf_adapter.go) builds a DocumentValues
// + []LineValues bundle from the GORM models and hands it to RenderHTML.
//
// Field resolution is deliberately string-typed at this boundary so:
//   • the renderer can format uniformly (money, date, address) without
//     reflecting on doc-specific structs;
//   • the adapter is the single place where currency precision / locale
//     formatting decisions live.
//
// LinkURL on LineValues is a small affordance for the future "click line
// to open product detail" Phase-B feature; the renderer ignores it for now.

// DocumentValues is the resolved scalar field map for one document.
// Keys MUST match Field.Key strings from the registry. Missing keys
// resolve to "" — the renderer hides FieldRefs marked HideWhenEmpty.
type DocumentValues map[string]string

// LineValues is one row in a lines_table block. Keys are the lines.* field
// keys from the registry (e.g. "lines.qty", "lines.line_total"). Same
// missing-key rule as DocumentValues.
type LineValues map[string]string

// RenderInput bundles everything RenderHTML needs.
type RenderInput struct {
	// DocumentType picks the field registry — controls which keys are
	// resolvable in FieldRefs. Required.
	DocumentType string
	// Schema is the parsed template (output of ParseSchema).
	Schema Schema
	// Values are the doc-level scalar resolutions.
	Values DocumentValues
	// Lines are the per-row resolutions for the lines_table block.
	Lines []LineValues
}

// Get looks up a doc-level value with a default for missing keys.
func (v DocumentValues) Get(key string) string {
	if v == nil {
		return ""
	}
	return v[key]
}

// Get looks up a line-level value with a default for missing keys.
func (l LineValues) Get(key string) string {
	if l == nil {
		return ""
	}
	return l[key]
}
