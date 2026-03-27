package models

import "testing"

func TestValidateAccountCode(t *testing.T) {
	t.Parallel()
	if err := ValidateAccountCode(""); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := ValidateAccountCode("1000"); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := ValidateAccountCode("100"); err != nil {
		t.Fatalf("3 digits: %v", err)
	}
	if err := ValidateAccountCode("12"); err == nil {
		t.Fatal("expected error for 2 digits")
	}
	if err := ValidateAccountCode("1"); err == nil {
		t.Fatal("expected error for 1 digit")
	}
	if err := ValidateAccountCode("123456789012"); err != nil {
		t.Fatalf("12 digits: %v", err)
	}
	if err := ValidateAccountCode("12a"); err == nil {
		t.Fatal("expected error for letter")
	}
	if err := ValidateAccountCode("12-3"); err == nil {
		t.Fatal("expected error for symbol")
	}
	if err := ValidateAccountCode("1234567890123"); err == nil {
		t.Fatal("expected error for 13 digits")
	}
}
