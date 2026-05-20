package pages

import (
	"context"
	"strings"
	"testing"
)

func TestSettingsHubIncludesTemplatesEntry(t *testing.T) {
	var sb strings.Builder
	vm := SettingsHubVM{HasCompany: true, CanViewSensitive: true}

	if err := SettingsHub(vm).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render settings hub: %v", err)
	}
	html := sb.String()

	for _, want := range []string{
		"Templates",
		`href="/settings/templates"`,
		"Manage PDF and print templates",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected settings hub HTML to contain %q", want)
		}
	}
}

func TestSettingsHubHidesSensitiveEntriesWithoutPermission(t *testing.T) {
	var sb strings.Builder
	vm := SettingsHubVM{HasCompany: true}

	if err := SettingsHub(vm).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render settings hub: %v", err)
	}
	html := sb.String()

	for _, blocked := range []string{
		`href="/settings/templates"`,
		`href="/settings/channels"`,
		`href="/settings/payment-gateways"`,
		`href="/settings/ar-ap-control"`,
		`href="/settings/ai-connect"`,
		`href="/settings/audit-log"`,
	} {
		if strings.Contains(html, blocked) {
			t.Fatalf("expected settings hub HTML not to contain %q", blocked)
		}
	}
}
