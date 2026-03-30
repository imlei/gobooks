// 遵循project_guide.md
package pages

import (
	"fmt"
	"strconv"

	"gobooks/internal/models"
)

// Uitoa formats a uint as a string (handy in templates).
func Uitoa(id uint) string {
	return fmt.Sprintf("%d", id)
}

// Itoa formats an int as a string (handy in templates).
func Itoa(i int) string {
	return strconv.Itoa(i)
}

// AccountRowClass styles inactive chart rows without changing overall table layout.
func AccountRowClass(a models.Account) string {
	if !a.IsActive {
		return "border-b border-border-subtle bg-background text-text-muted2"
	}
	return "border-b border-border-subtle"
}

// AccountClassificationLabel formats root · detail for tables.
func AccountClassificationLabel(a models.Account) string {
	return models.ClassificationDisplay(a.RootAccountType, a.DetailAccountType)
}

