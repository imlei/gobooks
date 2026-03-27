// 遵循产品需求 v1.0
package pages

import "gobooks/internal/numbering"

// NumberingSettingsVM is Settings > Company > Numbering (display numbering only).
type NumberingSettingsVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
	Rules      []numbering.DisplayRule
	FormError  string
	Saved      bool
}
