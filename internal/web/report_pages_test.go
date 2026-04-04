package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"gobooks/internal/services"
)

func TestResolveAsOfDateAt_UsesPresetEndDateForBalanceSheet(t *testing.T) {
	ref := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		preset     string
		fyEnd      string
		asOfRaw    string
		wantPreset string
		wantAsOf   string
	}{
		{
			name:       "last month uses prior month end",
			preset:     string(services.PresetLastMonth),
			fyEnd:      "12-31",
			wantPreset: string(services.PresetLastMonth),
			wantAsOf:   "2026-03-31",
		},
		{
			name:       "year to date uses today",
			preset:     string(services.PresetYearToDate),
			fyEnd:      "12-31",
			wantPreset: string(services.PresetYearToDate),
			wantAsOf:   "2026-04-03",
		},
		{
			name:       "last fiscal year uses prior fiscal year end",
			preset:     string(services.PresetLastFiscalYear),
			fyEnd:      "03-31",
			wantPreset: string(services.PresetLastFiscalYear),
			wantAsOf:   "2026-03-31",
		},
		{
			name:       "custom explicit date stays explicit",
			preset:     "",
			fyEnd:      "12-31",
			asOfRaw:    "2026-02-14",
			wantPreset: string(services.PresetCustom),
			wantAsOf:   "2026-02-14",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPreset, gotAsOf := resolveAsOfDateAt(tc.preset, tc.asOfRaw, tc.fyEnd, ref)
			if gotPreset != tc.wantPreset {
				t.Fatalf("preset: want %q, got %q", tc.wantPreset, gotPreset)
			}
			if gotAsOf != tc.wantAsOf {
				t.Fatalf("asOf: want %q, got %q", tc.wantAsOf, gotAsOf)
			}
		})
	}
}

func TestReportPages_UseToolbarAndPrintHideReportNavArea(t *testing.T) {
	db := testErrorFeedbackDB(t)
	server := &Server{DB: db}
	user := seedErrorFeedbackUser(t, db)
	companyID := seedValidationCompany(t, db, "Reports Toolbar Co")
	if err := db.Exec(`UPDATE companies SET fiscal_year_end = ? WHERE id = ?`, "03-31", companyID).Error; err != nil {
		t.Fatal(err)
	}

	app := errorFeedbackApp(server, user, companyID)
	app.Get("/reports/balance-sheet", server.handleBalanceSheet)
	app.Get("/reports/journal-entries", server.handleJournalEntryReport)

	balanceResp := performRequest(t, app, "/reports/balance-sheet?period=last_fiscal_year", "")
	if balanceResp.StatusCode != http.StatusOK {
		t.Fatalf("balance sheet: expected %d, got %d", http.StatusOK, balanceResp.StatusCode)
	}
	balanceBody := readResponseBody(t, balanceResp)
	wantAsOf := services.ComputeReportPeriod(services.PresetLastFiscalYear, "03-31", time.Now()).To.Format("2006-01-02")
	if !strings.Contains(balanceBody, `data-mode="asof"`) {
		t.Fatalf("expected balance sheet toolbar to run in asof mode, got %q", balanceBody)
	}
	if !strings.Contains(balanceBody, `data-as-of="`+wantAsOf+`"`) {
		t.Fatalf("expected balance sheet toolbar as-of %q, got %q", wantAsOf, balanceBody)
	}
	if !strings.Contains(balanceBody, "For Balance Sheet, presets choose the As Of date.") {
		t.Fatalf("expected balance sheet as-of helper text, got %q", balanceBody)
	}

	journalResp := performRequest(t, app, "/reports/journal-entries", "")
	if journalResp.StatusCode != http.StatusOK {
		t.Fatalf("journal report: expected %d, got %d", http.StatusOK, journalResp.StatusCode)
	}
	journalBody := readResponseBody(t, journalResp)
	if !strings.Contains(journalBody, `class="report-toolbar-form`) {
		t.Fatalf("expected journal report to use unified toolbar, got %q", journalBody)
	}
	if !strings.Contains(journalBody, "Report Period") {
		t.Fatalf("expected journal report toolbar label, got %q", journalBody)
	}
	if !strings.Contains(journalBody, `name="period"`) {
		t.Fatalf("expected journal report toolbar period field, got %q", journalBody)
	}
	if strings.Contains(journalBody, `>Export CSV</a>`) {
		t.Fatalf("expected journal report toolbar to hide CSV export link when no export URL, got %q", journalBody)
	}
	if !strings.Contains(journalBody, ".report-nav-area {") {
		t.Fatalf("expected print styles to hide report-nav-area, got %q", journalBody)
	}
}
