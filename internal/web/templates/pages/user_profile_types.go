// 遵循project_guide.md
package pages

// UserProfileVM is the view-model for GET /profile.
type UserProfileVM struct {
	HasCompany bool

	// Current user state (read from DB, never from form input).
	CurrentEmail string
	DisplayName  string

	// Flash messages shown at top.
	FormError   string
	FormSuccess string

	// ── Email change state ────────────────────────────────────────────────────
	// EmailStep: "" = show request form, "verify" = show code-entry form
	EmailStep     string
	EmailNewInput string // echoed back on validation error
	EmailChallengeID string

	// ── Password change state ─────────────────────────────────────────────────
	// PasswordStep: "" = show request form, "verify" = show code+new-pw form
	PasswordStep        string
	PasswordChallengeID string
}
