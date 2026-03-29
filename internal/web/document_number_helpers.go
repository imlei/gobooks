package web

import "strings"

func invoiceSaveErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint"):
		return "Invoice number already exists for this company."
	default:
		return "Could not save invoice: " + msg
	}
}

func billSaveErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint"):
		return "Bill number already exists for this vendor in this company."
	default:
		return "Could not create bill. Please try again."
	}
}
