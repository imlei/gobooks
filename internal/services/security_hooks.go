// 遵循project_guide.md
package services

// Package-level integration hooks for future security rule enforcement.
//
// ── Design intent ───────────────────────────────────────────────────────────
//
// This file defines the integration points that the login flow (and future
// operations) will call to evaluate security rules and dispatch alerts.
// No heavy detection logic is built yet; every function below is either a
// no-op stub or a thin wrapper around LogSecurityEvent.
//
// ── Future event flow ────────────────────────────────────────────────────────
//
// 1. Login handler detects a successful login (or failed attempt).
// 2. It calls EvaluateLoginSecurity(db, ctx).
// 3. EvaluateLoginSecurity loads company + system security settings.
// 4. For each enabled rule it dispatches an alert via the notification provider
//    (email / SMS / both) using LoadCompanyNotificationSettings / LoadSystemNotificationSettings.
// 5. The event is recorded via LogSecurityEvent for the audit trail.
//
// ── What is intentionally deferred ─────────────────────────────────────────
//
// - IP reputation / geolocation comparison (needs persistent IP history)
// - Device fingerprint storage and comparison
// - Failed-login threshold counting (needs a short-lived counter store)
// - Real alert dispatch (SendTestEmail / SendTestSMS are stubs today)
//
// ─────────────────────────────────────────────────────────────────────────────

import "gorm.io/gorm"

// LoginSecurityContext carries the information the login handler knows at the
// moment of authentication. Pass this to EvaluateLoginSecurity.
type LoginSecurityContext struct {
	CompanyID *uint  // nil for sysadmin logins
	UserID    string // UUID string or sysadmin ID
	IPAddress string
	UserAgent string
	Success   bool // false = failed login attempt
}

// EvaluateLoginSecurity is the single integration point for login-time security
// rules. It is safe to call from any login handler — it is a no-op until real
// detection logic is added.
//
// TODO(security): implement:
//   - load CompanySecuritySettings + SystemSecuritySettings
//   - compare IP against known IPs for the user (requires ip_history table)
//   - compare device fingerprint (requires device_fingerprint table)
//   - threshold-check failed login counts (requires short-lived counter)
//   - dispatch alerts via email/SMS providers when a rule fires
func EvaluateLoginSecurity(db *gorm.DB, ctx LoginSecurityContext) {
	// Record the raw event regardless of detection results.
	// This gives us a base audit trail that detection logic can query later.
	_ = LogSecurityEvent(
		db,
		ctx.CompanyID,
		&ctx.UserID,
		loginEventType(ctx.Success),
		ctx.IPAddress,
		ctx.UserAgent,
		nil,
	)

	// TODO(security): rule evaluation — load settings and fire alerts.
}

// loginEventType returns the canonical event_type string for login outcomes.
func loginEventType(success bool) string {
	if success {
		return "login.success"
	}
	return "login.failed"
}
