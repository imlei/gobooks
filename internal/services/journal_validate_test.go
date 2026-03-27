// 遵循产品需求 v1.0
package services

import (
	"strings"
	"testing"
)

func TestValidateJournalLines_balanced_twoLines(t *testing.T) {
	lines, err := ValidateJournalLines([]JournalLineDraft{
		{AccountID: "1", Debit: "100", Credit: ""},
		{AccountID: "2", Debit: "", Credit: "100"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
}

func TestValidateJournalLines_unbalanced(t *testing.T) {
	_, err := ValidateJournalLines([]JournalLineDraft{
		{AccountID: "1", Debit: "100", Credit: ""},
		{AccountID: "2", Debit: "", Credit: "99"},
	})
	if err == nil || !strings.Contains(err.Error(), "equal") {
		t.Fatalf("expected balance error, got %v", err)
	}
}

func TestValidateJournalLines_bothDebitAndCredit(t *testing.T) {
	_, err := ValidateJournalLines([]JournalLineDraft{
		{AccountID: "1", Debit: "10", Credit: "10"},
		{AccountID: "2", Debit: "", Credit: "20"},
	})
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected both debit/credit error, got %v", err)
	}
}

func TestValidateJournalLines_lessThanTwoLines(t *testing.T) {
	_, err := ValidateJournalLines([]JournalLineDraft{
		{AccountID: "1", Debit: "100", Credit: ""},
	})
	if err == nil || !strings.Contains(err.Error(), "2 valid") {
		t.Fatalf("expected line count error, got %v", err)
	}
}

func TestValidateJournalLines_skipsEmptyRows(t *testing.T) {
	lines, err := ValidateJournalLines([]JournalLineDraft{
		{},
		{AccountID: "1", Debit: "50", Credit: ""},
		{AccountID: "2", Debit: "50", Credit: ""},
		{AccountID: "3", Debit: "", Credit: "100"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines", len(lines))
	}
}

func TestValidateJournalLines_repeated(t *testing.T) {
	// Pure validation: safe to hammer serially (parallel requests hit DB separately).
	for i := 0; i < 500; i++ {
		_, err := ValidateJournalLines([]JournalLineDraft{
			{AccountID: "10", Debit: "1", Credit: ""},
			{AccountID: "20", Debit: "", Credit: "1"},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
