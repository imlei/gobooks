// 遵循project_guide.md
package models

import "strings"

// NormalizeAccountNameForSave trims leading/trailing whitespace and collapses
// runs of internal whitespace to a single space. Used on create/edit save only;
// does not alter meaning beyond spacing.
func NormalizeAccountNameForSave(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

// TrimGifiForStorage returns whitespace-trimmed GIFI for persistence.
// Call only after ValidateGifiCode succeeds (or for empty optional field).
func TrimGifiForStorage(raw string) string {
	return strings.TrimSpace(raw)
}
