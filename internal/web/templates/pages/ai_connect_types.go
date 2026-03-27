// 遵循产品需求 v1.0
package pages

// AIConnectVM is the view-model for Settings → AI Connect.
type AIConnectVM struct {
	HasCompany bool

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
