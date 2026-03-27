// 遵循产品需求 v1.0
package pages

import "gobooks/internal/numbering"

// NumberingSettingsVM is the Settings > Numbering page (display numbering only).
type NumberingSettingsVM struct {
	HasCompany bool
	Rules      []numbering.DisplayRule
	FormError  string
	Saved      bool
}
