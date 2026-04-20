// 遵循project_guide.md
package models

// company_feature.go — Company-scoped feature enablement registry.
//
// See migrations/072_company_features.sql for persistence semantics.
// This file defines the model shape + the static feature registry
// that the service layer consults when answering "which features
// does this product even ship?" and "what are their policies?"

import (
	"time"

	"github.com/google/uuid"
)

// ── Feature keys ─────────────────────────────────────────────────────────────

// FeatureKey is the stable identifier a feature is persisted under.
// New keys must be added here AND to AllCompanyFeatureDefinitions
// below; unknown keys are rejected by the service layer.
type FeatureKey string

const (
	FeatureKeyInventory FeatureKey = "inventory"
	FeatureKeyTask      FeatureKey = "task"
)

// ── Feature status ───────────────────────────────────────────────────────────

// FeatureStatus is the current effective state of a (company, feature)
// row. "off" is the default (absence of a row is also treated as
// "off"). "enabled" means the company has completed the self-serve
// enablement flow and the feature is gating open for this company.
type FeatureStatus string

const (
	FeatureStatusOff     FeatureStatus = "off"
	FeatureStatusEnabled FeatureStatus = "enabled"
)

// ── Feature maturity ─────────────────────────────────────────────────────────

// FeatureMaturity is a product-lifecycle label: alpha / beta / ga are
// real stages; coming_soon is a marker for features that are
// advertised on the UI but not yet available for self-enablement.
type FeatureMaturity string

const (
	FeatureMaturityAlpha      FeatureMaturity = "alpha"
	FeatureMaturityBeta       FeatureMaturity = "beta"
	FeatureMaturityGA         FeatureMaturity = "ga"
	FeatureMaturityComingSoon FeatureMaturity = "coming_soon"
)

// ── Reason codes ─────────────────────────────────────────────────────────────

// ReasonCode is the owner's declared reason for opting into an alpha
// feature. Recorded for analytics and audit; not enforced beyond the
// enumerated set.
type ReasonCode string

const (
	ReasonCodeTrialPilot              ReasonCode = "trial_pilot"
	ReasonCodeStartInventoryWorkflow  ReasonCode = "start_inventory_workflow"
	ReasonCodeMigration               ReasonCode = "migration"
	ReasonCodeSuggestedBySupport      ReasonCode = "suggested_by_support"
	ReasonCodeOther                   ReasonCode = "other"
)

// AllReasonCodes returns the enumerated set in display order.
func AllReasonCodes() []ReasonCode {
	return []ReasonCode{
		ReasonCodeTrialPilot,
		ReasonCodeStartInventoryWorkflow,
		ReasonCodeMigration,
		ReasonCodeSuggestedBySupport,
		ReasonCodeOther,
	}
}

// ReasonCodeLabel gives the human-facing label for a reason code.
func ReasonCodeLabel(c ReasonCode) string {
	switch c {
	case ReasonCodeTrialPilot:
		return "Trial / Pilot"
	case ReasonCodeStartInventoryWorkflow:
		return "Starting an inventory workflow"
	case ReasonCodeMigration:
		return "Migration from another system"
	case ReasonCodeSuggestedBySupport:
		return "Suggested by support"
	case ReasonCodeOther:
		return "Other"
	}
	return string(c)
}

// ── Ack versions ─────────────────────────────────────────────────────────────

// Acknowledgement version identifiers. When the risk copy or the
// required checkboxes change, a new version is introduced and the
// historical enablement rows retain the version they agreed to.
const (
	AckVersionInventoryAlphaV1 = "inventory-alpha-v1"
)

// ── Feature registry ─────────────────────────────────────────────────────────

// CompanyFeatureDefinition is the static (compile-time) description
// of a feature that the product offers. It is NOT persisted; it lives
// in code and is joined with per-company state at read time.
type CompanyFeatureDefinition struct {
	Key              FeatureKey
	Label            string
	Maturity         FeatureMaturity
	Description      string
	FitDescription   string
	SelfServeEnable  bool
	TypedConfirmText string
	AckVersion       string
}

// AllCompanyFeatureDefinitions is the canonical list of features the
// product offers today. Adding a new feature = adding a row here and
// a FeatureKey constant. Ordering is display order on the Features
// page.
func AllCompanyFeatureDefinitions() []CompanyFeatureDefinition {
	return []CompanyFeatureDefinition{
		{
			Key:      FeatureKeyInventory,
			Label:    "Inventory",
			Maturity: FeatureMaturityAlpha,
			Description: "Inventory workflows for businesses that hold physical " +
				"stock. Enabling this opens the Phase H receipt-first inbound " +
				"family of capabilities; disabling later does not rewrite any " +
				"historical records.",
			FitDescription: "Fits retail, wholesale, and light-manufacturing " +
				"businesses that move physical inventory. Not intended for " +
				"service-only or pure-software businesses.",
			SelfServeEnable:  true,
			TypedConfirmText: "ENABLE INVENTORY",
			AckVersion:       AckVersionInventoryAlphaV1,
		},
		{
			Key:             FeatureKeyTask,
			Label:           "Task",
			Maturity:        FeatureMaturityComingSoon,
			Description:     "Task and project tracking integrated with billable time, expenses, and reinvoice workflows.",
			FitDescription:  "Not yet available for self-enablement.",
			SelfServeEnable: false,
		},
	}
}

// LookupCompanyFeatureDefinition finds a definition by key. Returns
// nil when the key is not registered.
func LookupCompanyFeatureDefinition(key FeatureKey) *CompanyFeatureDefinition {
	for _, d := range AllCompanyFeatureDefinitions() {
		if d.Key == key {
			defCopy := d
			return &defCopy
		}
	}
	return nil
}

// InventoryAlphaRequiredAcknowledgements returns the set of
// acknowledgement labels that a user must agree to before the
// Inventory Alpha enablement is accepted. Order matters: the service
// layer validates that the caller supplied exactly this many booleans
// all set to true.
//
// Bumping the text here requires bumping the AckVersion above, so
// that historical enablements remain attributable to the language
// they agreed to.
func InventoryAlphaRequiredAcknowledgements() []string {
	return []string{
		"I understand this feature is in Alpha and still being validated.",
		"I understand disabling it later will not rewrite historical records.",
		"I agree to review related documents and results after enablement.",
	}
}

// ── CompanyFeature persistence model ─────────────────────────────────────────

// CompanyFeature persists per-company state for one feature. See
// migrations/072_company_features.sql for column semantics.
//
// Row presence semantics:
//   - No row for (company, feature) = feature is OFF for that company
//     AND has never been toggled. The service layer treats the
//     absence as FeatureStatusOff.
//   - Row with status='off' but non-nil enabled_at / enabled_by_user_id
//     = feature was enabled at some point and then disabled. History
//     preserved; current state is still off.
//   - Row with status='enabled' = currently enabled. enabled_at /
//     enabled_by_user_id / acknowledged_at / ack_version all populated.
type CompanyFeature struct {
	ID        uint       `gorm:"primaryKey"`
	CompanyID uint       `gorm:"not null;index"`
	FeatureKey FeatureKey `gorm:"column:feature_key;type:text;not null"`

	Status   FeatureStatus   `gorm:"type:text;not null;default:'off'"`
	Maturity FeatureMaturity `gorm:"type:text;not null;default:'alpha'"`

	EnabledAt       *time.Time
	EnabledByUserID *uuid.UUID `gorm:"type:uuid"`
	AcknowledgedAt  *time.Time
	AckVersion      string `gorm:"not null;default:''"`
	ReasonCode      ReasonCode `gorm:"column:reason_code;type:text;not null;default:''"`
	ReasonNote      string     `gorm:"not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName binds the model to the `company_features` table.
func (CompanyFeature) TableName() string {
	return "company_features"
}
