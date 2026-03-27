package numbering

import "testing"

func TestFormatPreview(t *testing.T) {
	if got := FormatPreview("INV-", 1, 4); got != "INV-0001" {
		t.Fatalf("got %q", got)
	}
	if got := FormatPreview("JE-", 12, 0); got != "JE-12" {
		t.Fatalf("got %q", got)
	}
}
