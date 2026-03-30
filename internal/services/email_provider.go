// 遵循project_guide.md
package services

import (
	"errors"
	"strings"

	"gobooks/internal/models"
)

// EmailConfig holds the SMTP parameters required to send email.
type EmailConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string // plaintext, already decrypted by caller
	FromEmail  string
	FromName   string
	Encryption models.SMTPEncryption
}

// ValidateEmailConfig checks that the minimum required fields are present.
// Returns a non-nil error describing the first missing requirement.
func ValidateEmailConfig(cfg EmailConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("SMTP host is required")
	}
	if cfg.Port <= 0 {
		return errors.New("SMTP port must be a positive integer")
	}
	if strings.TrimSpace(cfg.FromEmail) == "" {
		return errors.New("From email address is required")
	}
	return nil
}

// SendTestEmail validates cfg and, when valid, performs a stub send.
//
// Real SMTP delivery (net/smtp or a library) is intentionally deferred until
// actual notification features are built. Until then, this function validates
// the configuration and returns a clear message so the UI can distinguish
// "config looks complete" from "config is missing required fields".
//
// Returns (message, error).  error is non-nil only for configuration problems,
// not for SMTP delivery failures (which are handled by the caller as soft errors).
//
// STUB: No email is sent. Replace with a real dialler when email delivery is
// implemented (see Phase 4 implementation note).
func SendTestEmail(cfg EmailConfig) (string, error) {
	if err := ValidateEmailConfig(cfg); err != nil {
		return "", err
	}
	// TODO(phase4): replace stub with actual net/smtp or gomail dial + send.
	return "Configuration looks valid. (Stub — no email sent. Real delivery will be wired in a future release.)", nil
}
