// 遵循产品需求 v1.0
package ui

import (
	"fmt"
	"strconv"
)

// AccountCodeHTMLPattern returns a pattern attribute for exact digit length, first digit 1–9.
func AccountCodeHTMLPattern(digitLen int) string {
	if digitLen < 1 {
		return "^[0-9]*$"
	}
	if digitLen == 1 {
		return "^[1-9]$"
	}
	return fmt.Sprintf("^[1-9][0-9]{%d}$", digitLen-1)
}

// AccountCodeInputTitle is the browser validation tooltip for the code field.
func AccountCodeInputTitle(digitLen int) string {
	return fmt.Sprintf("Exactly %d digits; cannot start with 0.", digitLen)
}

// IntString formats an int for HTML maxlength/minlength attributes.
func IntString(n int) string {
	return strconv.Itoa(n)
}
