package models

import "testing"

func TestExpandAccountCodeToLength(t *testing.T) {
	t.Parallel()
	cases := []struct {
		base   string
		target int
		want   string
	}{
		{"1000", 4, "1000"},
		{"1000", 5, "10000"},
		{"1000", 6, "100000"},
		{"2100", 5, "21000"},
	}
	for _, tc := range cases {
		got, err := ExpandAccountCodeToLength(tc.base, tc.target)
		if err != nil {
			t.Fatalf("%q len %d: %v", tc.base, tc.target, err)
		}
		if got != tc.want {
			t.Fatalf("%q → %d: got %q want %q", tc.base, tc.target, got, tc.want)
		}
	}
}

func TestValidateAccountCodeStrict(t *testing.T) {
	t.Parallel()
	const L = 5
	if err := ValidateAccountCodeStrict("", L); err != nil {
		t.Fatal("empty: expected nil")
	}
	if err := ValidateAccountCodeStrict("10000", L); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := ValidateAccountCodeStrict("01000", L); err == nil {
		t.Fatal("leading zero: expected error")
	}
	if err := ValidateAccountCodeStrict("1000", L); err == nil {
		t.Fatal("short: expected error")
	}
	if err := ValidateAccountCodeStrict("100000", L); err == nil {
		t.Fatal("long: expected error")
	}
	if err := ValidateAccountCodeStrict("10a00", L); err == nil {
		t.Fatal("non-digit: expected error")
	}
}

func TestRootRequiredPrefixDigit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		root RootAccountType
		d    byte
	}{
		{RootAsset, '1'},
		{RootLiability, '2'},
		{RootEquity, '3'},
		{RootRevenue, '4'},
		{RootCostOfSales, '5'},
		{RootExpense, '6'},
	}
	for _, tc := range cases {
		got, err := RootRequiredPrefixDigit(tc.root)
		if err != nil {
			t.Fatalf("%s: %v", tc.root, err)
		}
		if got != tc.d {
			t.Fatalf("%s: got %c want %c", tc.root, got, tc.d)
		}
	}
}

func TestValidateAccountCodePrefixForRoot(t *testing.T) {
	t.Parallel()
	if err := ValidateAccountCodePrefixForRoot("11000", RootAsset); err != nil {
		t.Fatalf("11000 + asset: %v", err)
	}
	if err := ValidateAccountCodePrefixForRoot("21000", RootAsset); err == nil {
		t.Fatal("21000 + asset: expected error")
	}
}

func TestNormalizeAccountNameForSave(t *testing.T) {
	t.Parallel()
	if got := NormalizeAccountNameForSave("  Cash  on  Hand  "); got != "Cash on Hand" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeAccountNameForSave(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestTrimGifiForStorage(t *testing.T) {
	t.Parallel()
	if got := TrimGifiForStorage("  1599  "); got != "1599" {
		t.Fatalf("got %q", got)
	}
	if got := TrimGifiForStorage(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestValidateGifiCode(t *testing.T) {
	t.Parallel()
	if err := ValidateGifiCode(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGifiCode("  "); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGifiCode("1599"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGifiCode("159"); err == nil {
		t.Fatal("3 digits: expected error")
	}
	if err := ValidateGifiCode("15999"); err == nil {
		t.Fatal("5 digits: expected error")
	}
	if err := ValidateGifiCode("15a9"); err == nil {
		t.Fatal("non-digit: expected error")
	}
}

func TestLegacyTypeColumnToRootDetail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		legacy string
		root   RootAccountType
		detail DetailAccountType
	}{
		{"Bank", RootAsset, DetailBank},
		{"Income", RootRevenue, DetailOperatingRevenue},
		{"asset", RootAsset, DetailOtherAsset},
		{"expense", RootExpense, DetailOperatingExpense},
	}
	for _, tc := range cases {
		r, d, err := LegacyTypeColumnToRootDetail(tc.legacy)
		if err != nil {
			t.Fatalf("%q: %v", tc.legacy, err)
		}
		if r != tc.root || d != tc.detail {
			t.Fatalf("%q: got %s/%s want %s/%s", tc.legacy, r, d, tc.root, tc.detail)
		}
	}
}
