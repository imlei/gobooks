// 遵循产品需求 v1.0
package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Allowed chars per product requirement: letters, digits, -, #, @
	docNoAllowed = regexp.MustCompile(`^[A-Za-z0-9\-#@]+$`)
	docNoSuffix  = regexp.MustCompile(`^(.*?)(\d+)$`)
)

// ValidateDocumentNumber checks allowed characters and non-empty value.
func ValidateDocumentNumber(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("document number is required")
	}
	if !docNoAllowed.MatchString(s) {
		return fmt.Errorf("only letters, numbers, -, #, @ are allowed")
	}
	return nil
}

// NextDocumentNumber derives next number from the last number.
//
// Examples:
// - IN001 -> IN002
// - INV-0099 -> INV-0100
// - A#@ -> A#@-001
func NextDocumentNumber(last string, fallback string) string {
	last = strings.TrimSpace(last)
	if last == "" {
		return fallback
	}
	if !docNoAllowed.MatchString(last) {
		return fallback
	}

	m := docNoSuffix.FindStringSubmatch(last)
	if len(m) != 3 {
		return last + "-001"
	}

	prefix := m[1]
	digits := m[2]
	n, err := strconv.Atoi(digits)
	if err != nil {
		return fallback
	}
	n++

	return fmt.Sprintf("%s%0*d", prefix, len(digits), n)
}

