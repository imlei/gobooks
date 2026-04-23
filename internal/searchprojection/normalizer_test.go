// 遵循project_guide.md
package searchprojection

import "testing"

func TestAsciiNormalizer_Native(t *testing.T) {
	n := AsciiNormalizer{}
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"Acme Corp", "acme corp"},
		{"  ACME  ", "acme"},
		{"Acme,Corp", "acme corp"}, // punctuation → word boundary
		{"Acme-Corp", "acme corp"},
		{"a   b\t c", "a b c"}, // whitespace runs collapse
		{"123 INV-001", "123 inv 001"},
		// Non-ASCII passes through (Native preserves any codepoint).
		{"李华", "李华"},
		{"Café", "café"},
	}
	for _, tc := range cases {
		if got := n.Native(tc.in); got != tc.want {
			t.Errorf("Native(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAsciiNormalizer_Latin(t *testing.T) {
	n := AsciiNormalizer{}
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"Acme Corp", "acme corp"},
		// CJK is dropped in the ASCII impl — multi-language impl will
		// override to return pinyin.
		{"李华", ""},
		// Mixed: only the ASCII portion survives.
		{"李华 Trading", "trading"},
		// Accented Latin survives Native but is non-ASCII — also dropped.
		{"Café", "caf"},
	}
	for _, tc := range cases {
		if got := n.Latin(tc.in); got != tc.want {
			t.Errorf("Latin(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAsciiNormalizer_Initials(t *testing.T) {
	n := AsciiNormalizer{}
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"Acme Corp", "ac"},
		{"acme", "a"},
		{"ABC Trading Co", "atc"},
		// Punctuation as separator.
		{"Acme,Corp", "ac"},
		// CJK words contribute nothing in the ASCII impl.
		{"李华", ""},
		{"李华 Trading", "t"},
	}
	for _, tc := range cases {
		if got := n.Initials(tc.in); got != tc.want {
			t.Errorf("Initials(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestApplyNormalizer_NilFallsBackToAscii(t *testing.T) {
	got := applyNormalizer(nil, Document{Title: "Acme,Corp", Memo: "Note 1"})
	if got.TitleNative != "acme corp" {
		t.Errorf("TitleNative = %q, want %q", got.TitleNative, "acme corp")
	}
	if got.TitleInitials != "ac" {
		t.Errorf("TitleInitials = %q, want %q", got.TitleInitials, "ac")
	}
	if got.MemoNative != "note 1" {
		t.Errorf("MemoNative = %q, want %q", got.MemoNative, "note 1")
	}
}
