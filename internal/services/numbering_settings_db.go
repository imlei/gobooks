// 遵循project_guide.md
package services

import (
	"encoding/json"
	"errors"

	"gobooks/internal/models"
	"gobooks/internal/numbering"

	"gorm.io/gorm"
)

// LoadMergedDisplayRules loads company-scoped numbering rules from numbering_settings.rules_json,
// merged onto defaults (same semantics as file-based numbering).
func LoadMergedDisplayRules(db *gorm.DB, companyID uint) ([]numbering.DisplayRule, error) {
	defaults := numbering.DefaultDisplayRules()
	var row models.NumberingSetting
	err := db.Where("company_id = ?", companyID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaults, nil
		}
		return nil, err
	}
	if len(row.RulesJSON) == 0 || string(row.RulesJSON) == "null" {
		return defaults, nil
	}
	var saved []numbering.DisplayRule
	if err := json.Unmarshal(row.RulesJSON, &saved); err != nil {
		return defaults, nil
	}
	return numbering.MergeSavedOntoDefaults(defaults, saved), nil
}

// SaveMergedDisplayRules persists the full merged rule list for a company.
func SaveMergedDisplayRules(db *gorm.DB, companyID uint, rules []numbering.DisplayRule) error {
	b, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	var row models.NumberingSetting
	err = db.Where("company_id = ?", companyID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.NumberingSetting{
			CompanyID: companyID,
			Version:   1,
			RulesJSON: b,
		}
		return db.Create(&row).Error
	}
	if err != nil {
		return err
	}
	row.RulesJSON = b
	return db.Save(&row).Error
}

// SuggestNextInvoiceNumber returns the next display number from DB-backed invoice module settings.
func SuggestNextInvoiceNumber(db *gorm.DB, companyID uint) (string, error) {
	rules, err := LoadMergedDisplayRules(db, companyID)
	if err != nil {
		return "", err
	}
	for _, r := range rules {
		if r.ModuleKey == numbering.ModuleInvoice && r.Enabled {
			return numbering.FormatPreview(r.Prefix, r.NextNumber, r.PaddingLength), nil
		}
	}
	return "IN001", nil
}

// BumpInvoiceNextNumberAfterCreate increments the invoice module's next_number in numbering_settings.
func BumpInvoiceNextNumberAfterCreate(db *gorm.DB, companyID uint) error {
	rules, err := LoadMergedDisplayRules(db, companyID)
	if err != nil {
		return err
	}
	for i := range rules {
		if rules[i].ModuleKey == numbering.ModuleInvoice {
			rules[i].NextNumber++
			rules[i] = numbering.NormalizeRule(rules[i])
			break
		}
	}
	return SaveMergedDisplayRules(db, companyID, rules)
}
