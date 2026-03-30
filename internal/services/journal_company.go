// 遵循project_guide.md
package services

import (
	"fmt"

	"gobooks/internal/models"

	"gorm.io/gorm"
)

// EnsureJournalLineAccountsBelongToCompany checks that every line's account_id
// references an account row with the given company_id.
func EnsureJournalLineAccountsBelongToCompany(tx *gorm.DB, companyID uint, lines []models.JournalLine) error {
	for _, line := range lines {
		if line.AccountID == 0 {
			return fmt.Errorf("invalid account on a journal line")
		}
		var acc models.Account
		if err := tx.Select("id", "company_id").First(&acc, line.AccountID).Error; err != nil {
			return err
		}
		if acc.CompanyID != companyID {
			return fmt.Errorf("one or more accounts do not belong to this company")
		}
	}
	return nil
}
