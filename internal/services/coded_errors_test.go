package services

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestCodedErrorsSurviveWrapping(t *testing.T) {
	err := fmt.Errorf("post invoice: %w", ErrAlreadyPosted)

	if !errors.Is(err, ErrAlreadyPosted) {
		t.Fatal("wrapped coded error should preserve errors.Is")
	}
	if got := ErrorCode(err); got != "POSTING_ALREADY_POSTED" {
		t.Fatalf("ErrorCode = %q", got)
	}
	if got := ErrorHTTPStatus(err, http.StatusInternalServerError); got != http.StatusConflict {
		t.Fatalf("ErrorHTTPStatus = %d", got)
	}
}
