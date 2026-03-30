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

// SendTestSMS validates cfg and, when valid, returns a success message.
//
// IMPORTANT: This function only validates that the required fields are present.
// It does NOT make a real API call to the SMS provider. Consequently,
// SMSVerificationReady being true means "config looks complete", not
// "delivery confirmed". Real provider integration (Twilio, Vonage, etc.)
// is deferred to a future release at which point this function should be
// replaced with an actual test-send via the provider SDK.
func SendTestSMS(cfg SMSConfig) (string, error) {
	if err := ValidateSMSConfig(cfg); err != nil {
		return "", err
	}
	return "SMS configuration looks valid (fields complete). Note: no test message was sent — real provider connectivity is not yet verified.", nil
}
