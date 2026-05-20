package pages

import (
	"balanciz/internal/models"
	"balanciz/internal/services"
)

type EmployeesVM struct {
	HasCompany bool

	Employees []models.Employee
	Query     string
	Status    string

	Created   bool
	FormError string
	CanManage bool
}

type PayrollRunsVM struct {
	HasCompany bool

	Runs []models.PayrollRun

	Created        bool
	Calculated     bool
	FormError      string
	CanRun         bool
	CanViewDetails bool
}

type PayrollRemittancesVM struct {
	HasCompany bool

	Remittances  []models.PayrollRemittance
	BankAccounts []models.Account

	Created   bool
	Paid      bool
	Voided    bool
	FormError string
	CanPost   bool
}

type PayrollRunDetailVM struct {
	HasCompany bool

	Run                models.PayrollRun
	Entries            []models.PayrollEntry
	ChequeBankAccounts []models.ChequeBankAccount

	Calculated           bool
	Finalized            bool
	Posted               bool
	RemittanceCreated    bool
	ChequesCreated       bool
	FormError            string
	CanRun               bool
	CanFinalize          bool
	CanPost              bool
	CanCreateRemittance  bool
	CanExport            bool
	CanCreateCheques     bool
	CanViewDetails       bool
	PostedJournalEntryID uint
	RemittanceID         uint
}

type PayrollSummaryReportVM struct {
	HasCompany bool

	From      string
	To        string
	DateLabel string
	Report    services.PayrollSummaryReport

	FormError string
	Toolbar   ReportToolbarVM
}

type PayrollEmployeeHistoryReportVM struct {
	HasCompany bool

	From       string
	To         string
	EmployeeID uint
	Employees  []models.Employee
	Report     services.PayrollEmployeeHistoryReport

	FormError string
	Toolbar   ReportToolbarVM
}
