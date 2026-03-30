// 遵循project_guide.md
package services

import (
	"errors"
	"strings"
)

// SMSConfig holds the provider parameters required to send an SMS message.
type SMSConfig struct {
	Provider  string // e.g. "twilio", "vonage"
	APIKey    string // plaintext, already decrypted by caller
	APISecret string // plaintext, already decrypted by caller
	SenderID  string // phone number or alphanumeric sender ID
}

// ValidateSMSConfig checks that the minimum required fields are present.
func ValidateSMSConfig(cfg SMSConfig) error {
	if strings.TrimSpace(cfg.Provider) == "" {
		return errors.New("SMS provider is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New("SMS API key is required")
	}
	if strings.TrimSpace(cfg.SenderID) == "" {
		return errors.New("SMS sender ID / from number is required")
	}
	return nil
}

// SendTestSMS validates cfg and, when valid, performs a stub send.
//
// Real provider integration (Twilio, Vonage, etc.) is intentionally deferred
// until actual notification features are built. This function validates the
// configuration and returns a clear message for UI feedback.
//
// STUB: No SMS is sent. Replace with a real provider client when SMS delivery
// is implemented (see Phase 4 implementation note).
func SendTestSMS(cfg SMSConfig) (string, error) {
	if err := ValidateSMSConfig(cfg); err != nil {
		return "", err
	}
	// TODO(phase4): replace stub with provider-specific REST call (Twilio/Vonage SDK).
	return "Configuration looks valid. (Stub — no SMS sent. Real delivery will be wired in a future release.)", nil
}
