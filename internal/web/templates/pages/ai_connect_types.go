// 遵循project_guide.md
package pages

// AIConnectVM is the view-model for Settings → AI Connect.
type AIConnectVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
	// ReadOnly is true for members who may view but not edit (non-owner/admin).
	ReadOnly bool

	FormError string
	Saved     bool
	Tested    bool

	Provider    string
	APIBaseURL  string
	ModelName   string
	Enabled     bool
	VisionEnabled bool

	HasAPIKey  bool
	APIKeyHint string

	HasLastTest         bool
	LastTestAtFormatted string
	LastTestOK          bool
	LastTestMessage     string
}
