// 遵循产品需求 v1.0
package models

import "time"

// AI provider identifiers (extensible for future OCR / document workflows).
const (
	AIProviderOpenAICompatible = "openai_compatible"
	AIProviderCustomEndpoint   = "custom_endpoint"
)

// AIConnectionSettings stores server-side AI API configuration for the company.
// Secrets stay in the database; never send full API keys to the browser or audit JSON.
type AIConnectionSettings struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"uniqueIndex;not null"`

	Provider  string `gorm:"type:text;not null;default:openai_compatible"`
	APIBaseURL string `gorm:"type:text"`
	APIKey     string `gorm:"type:text"`
	ModelName  string `gorm:"type:text"`

	Enabled       bool `gorm:"not null;default:false"`
	VisionEnabled bool `gorm:"not null;default:false"`

	LastTestAt      *time.Time
	LastTestOK      bool
	LastTestMessage string `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
