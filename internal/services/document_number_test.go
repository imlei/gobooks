// 遵循project_guide.md
package services

import "testing"

func TestValidateDocumentNumber(t *testing.T) {
	if err := ValidateDocumentNumber("INV-001"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocumentNumber("A#@"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocumentNumber(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if err := ValidateDocumentNumber("bad space"); err == nil {
		t.Fatal("expected error for space")
	}
}

func TestNextDocumentNumber(t *testing.T) {
	if got := NextDocumentNumber("IN001", "X-001"); got != "IN002" {
		t.Fatalf("got %q", got)
	}
	if got := NextDocumentNumber("", "FALL"); got != "FALL" {
		t.Fatalf("got %q", got)
	}
}
