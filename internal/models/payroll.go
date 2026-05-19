package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type EmployeeStatus string

const (
	EmployeeStatusActive     EmployeeStatus = "active"
	EmployeeStatusTerminated EmployeeStatus = "terminated"
	EmployeeStatusInactive   EmployeeStatus = "inactive"
)

type EmployeeMemberType string

const (
	EmployeeMemberEmployee               EmployeeMemberType = "employee"
	EmployeeMemberContractor             EmployeeMemberType = "contractor"
	EmployeeMemberConstructionContractor EmployeeMemberType = "construction_contractor"
)

type EmployeeSalaryType string

const (
	EmployeeSalarySalaried  EmployeeSalaryType = "salaried"
	EmployeeSalaryTimeBased EmployeeSalaryType = "time_based"
)

type PayFrequency string

const (
	PayFrequencyWeekly      PayFrequency = "weekly"
	PayFrequencyBiweekly    PayFrequency = "biweekly"
	PayFrequencySemiMonthly PayFrequency = "semimonthly"
	PayFrequencyMonthly     PayFrequency = "monthly"
)

// Employee is the company-scoped people master shared by Payroll, Cheque, and
// future HR workflows. Search projections must use only non-sensitive fields:
// employee number, display/legal name, position, province, and status.
type Employee struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	Company   Company

	EmployeeNo  string `gorm:"type:varchar(64);not null;default:'';index"`
	LegalName   string `gorm:"type:text;not null;default:''"`
	DisplayName string `gorm:"type:text;not null;default:''"`
	Email       string `gorm:"type:text;not null;default:''"`
	Mobile      string `gorm:"type:text;not null;default:''"`
	Position    string `gorm:"type:text;not null;default:''"`
	Notes       string `gorm:"type:text;not null;default:''"`

	AddrStreet1    string `gorm:"type:text;not null;default:''"`
	AddrStreet2    string `gorm:"type:text;not null;default:''"`
	AddrCity       string `gorm:"type:text;not null;default:''"`
	AddrProvince   string `gorm:"type:text;not null;default:''"`
	AddrPostalCode string `gorm:"type:text;not null;default:''"`
	AddrCountry    string `gorm:"type:text;not null;default:'CA'"`

	ProvinceOfEmployment string `gorm:"type:varchar(16);not null;default:''"`
	SINCiphertext        string `gorm:"type:text;not null;default:''"`
	SINLast4             string `gorm:"type:varchar(4);not null;default:''"`
	DateOfBirth          *time.Time
	HireDate             *time.Time
	TerminationDate      *time.Time

	MemberType   EmployeeMemberType `gorm:"type:text;not null;default:'employee'"`
	SalaryType   EmployeeSalaryType `gorm:"type:text;not null;default:'time_based'"`
	Status       EmployeeStatus     `gorm:"type:text;not null;default:'active';index"`
	PayRate      decimal.Decimal    `gorm:"type:numeric(18,6);not null;default:0"`
	PayRateUnit  string             `gorm:"type:text;not null;default:'hourly'"`
	PaysPerYear  int                `gorm:"not null;default:26"`
	PayFrequency PayFrequency       `gorm:"type:text;not null;default:'biweekly'"`
	HoursPerWeek decimal.Decimal    `gorm:"type:numeric(18,4);not null;default:0"`

	TD1Federal    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TD1Provincial decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	PaidYTDOtherPayroll bool `gorm:"not null;default:false"`
	AutoVacation        bool `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (e Employee) SearchName() string {
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.LegalName
}

type PayrollRunStatus string

const (
	PayrollRunDraft      PayrollRunStatus = "draft"
	PayrollRunCalculated PayrollRunStatus = "calculated"
	PayrollRunFinalized  PayrollRunStatus = "finalized"
	PayrollRunVoided     PayrollRunStatus = "voided"
)

type PayrollRun struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	Company   Company

	RunNumber    string           `gorm:"type:varchar(64);not null;default:'';index"`
	PeriodStart  time.Time        `gorm:"not null;index"`
	PeriodEnd    time.Time        `gorm:"not null;index"`
	PayDate      time.Time        `gorm:"not null;index"`
	PaysPerYear  int              `gorm:"not null;default:26"`
	PayFrequency PayFrequency     `gorm:"type:text;not null;default:'biweekly'"`
	PayrollType  string           `gorm:"type:text;not null;default:'regular'"`
	Status       PayrollRunStatus `gorm:"type:text;not null;default:'draft';index"`

	TotalGross          decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployeeTax    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployeeCPP    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployeeCPP2   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployeeEI     decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployerCPP    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployerCPP2   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalEmployerEI     decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalDeductions     decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalNetPay         decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	CalculationSnapshot datatypes.JSON  `gorm:"type:jsonb"`

	FinalizedAt       *time.Time
	FinalizedByUserID *uuid.UUID `gorm:"type:uuid"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type PayrollEntryStatus string

const (
	PayrollEntryDraft    PayrollEntryStatus = "draft"
	PayrollEntryApproved PayrollEntryStatus = "approved"
)

type PayrollEntry struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	Company   Company

	PayrollRunID uint       `gorm:"not null;index;uniqueIndex:uq_payroll_entry_run_employee,priority:1"`
	PayrollRun   PayrollRun `gorm:"foreignKey:PayrollRunID"`
	EmployeeID   uint       `gorm:"not null;index;uniqueIndex:uq_payroll_entry_run_employee,priority:2"`
	Employee     Employee   `gorm:"foreignKey:EmployeeID"`

	Hours    decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0"`
	PayRate  decimal.Decimal `gorm:"type:numeric(18,6);not null;default:0"`
	GrossPay decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	CPPEmployee     decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	CPP2Employee    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	EIEmployee      decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	FederalTax      decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	ProvincialTax   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalDeductions decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	NetPay          decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	CPPEmployer  decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	CPP2Employer decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	EIEmployer   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	YTDGross        decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	YTDCPPEmployee  decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	YTDCPP2Employee decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	YTDEIEmployee   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	CalculationSnapshot datatypes.JSON     `gorm:"type:jsonb"`
	PaymentType         string             `gorm:"type:text;not null;default:'cheque'"`
	Status              PayrollEntryStatus `gorm:"type:text;not null;default:'draft'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type PayrollEarningCode struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index;uniqueIndex:uq_payroll_earning_codes_company_code,priority:1"`
	Company   Company

	Code          string          `gorm:"type:varchar(64);not null;uniqueIndex:uq_payroll_earning_codes_company_code,priority:2"`
	Name          string          `gorm:"type:text;not null;default:''"`
	Enabled       bool            `gorm:"not null;default:true"`
	CPP           bool            `gorm:"not null;default:true"`
	EI            bool            `gorm:"not null;default:true"`
	TaxFederal    bool            `gorm:"not null;default:true"`
	TaxProvincial bool            `gorm:"not null;default:true"`
	NonCash       bool            `gorm:"not null;default:false"`
	Vacationable  bool            `gorm:"not null;default:true"`
	Multiplier    decimal.Decimal `gorm:"type:numeric(9,4);not null;default:1"`
	IsSystem      bool            `gorm:"not null;default:false"`
	T4Box         string          `gorm:"type:varchar(16);not null;default:''"`
	SortOrder     int             `gorm:"not null;default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type PayrollEntryEarning struct {
	ID             uint `gorm:"primaryKey"`
	PayrollEntryID uint `gorm:"not null;index"`
	PayrollEntry   PayrollEntry
	EarningCodeID  uint               `gorm:"not null;index"`
	EarningCode    PayrollEarningCode `gorm:"foreignKey:EarningCodeID"`

	Hours  decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0"`
	Rate   decimal.Decimal `gorm:"type:numeric(18,6);not null;default:0"`
	Amount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChequeBankAccount struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	Company   Company

	Label                 string `gorm:"type:text;not null;default:''"`
	BankName              string `gorm:"type:text;not null;default:''"`
	BankAddress           string `gorm:"type:text;not null;default:''"`
	BankCity              string `gorm:"type:text;not null;default:''"`
	BankProvince          string `gorm:"type:text;not null;default:''"`
	BankPostalCode        string `gorm:"type:text;not null;default:''"`
	MICRCountry           string `gorm:"type:varchar(8);not null;default:'CA'"`
	BankInstitution       string `gorm:"type:text;not null;default:''"`
	BankTransit           string `gorm:"type:text;not null;default:''"`
	BankRoutingABA        string `gorm:"type:text;not null;default:''"`
	BankAccountCiphertext string `gorm:"type:text;not null;default:''"`
	BankAccountLast4      string `gorm:"type:varchar(4);not null;default:''"`
	BankIBANCiphertext    string `gorm:"type:text;not null;default:''"`
	BankSWIFT             string `gorm:"type:text;not null;default:''"`
	LedgerAccountID       *uint  `gorm:"index"`
	LedgerAccount         *Account
	NextChequeNumber      string `gorm:"type:varchar(64);not null;default:''"`
	DefaultCurrencyCode   string `gorm:"type:varchar(3);not null;default:'CAD'"`
	IsActive              bool   `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChequeStatus string

const (
	ChequeStatusDraft   ChequeStatus = "draft"
	ChequeStatusPrinted ChequeStatus = "printed"
	ChequeStatusVoided  ChequeStatus = "voided"
)

type Cheque struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	Company   Company

	BankAccountID uint              `gorm:"not null;index"`
	BankAccount   ChequeBankAccount `gorm:"foreignKey:BankAccountID"`
	ChequeNumber  string            `gorm:"type:varchar(64);not null;default:'';index"`

	PayeeType  string `gorm:"type:text;not null;default:'other'"`
	PayeeName  string `gorm:"type:text;not null;default:''"`
	VendorID   *uint  `gorm:"index"`
	Vendor     *Vendor
	EmployeeID *uint `gorm:"index"`
	Employee   *Employee

	PayrollRunID   *uint `gorm:"index"`
	PayrollRun     *PayrollRun
	PayrollEntryID *uint `gorm:"index"`
	PayrollEntry   *PayrollEntry

	ChequeDate   time.Time       `gorm:"not null;index"`
	CurrencyCode string          `gorm:"type:varchar(3);not null;default:'CAD'"`
	Amount       decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	Memo         string          `gorm:"type:text;not null;default:''"`
	Status       ChequeStatus    `gorm:"type:text;not null;default:'draft';index"`
	PrintedAt    *time.Time
	VoidedAt     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PayrollRemittanceStatus string

const (
	PayrollRemittanceDraft  PayrollRemittanceStatus = "draft"
	PayrollRemittancePaid   PayrollRemittanceStatus = "paid"
	PayrollRemittanceVoided PayrollRemittanceStatus = "voided"
)

type PayrollRemittance struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	Company   Company

	PayrollRunID uint       `gorm:"not null;index;uniqueIndex:uq_payroll_remittance_run"`
	PayrollRun   PayrollRun `gorm:"foreignKey:PayrollRunID"`

	RemittanceNumber string                  `gorm:"type:varchar(64);not null;default:'';index"`
	Status           PayrollRemittanceStatus `gorm:"type:text;not null;default:'draft';index"`
	PeriodStart      time.Time               `gorm:"not null;index"`
	PeriodEnd        time.Time               `gorm:"not null;index"`
	DueDate          time.Time               `gorm:"not null;index"`
	PaymentDate      *time.Time

	CPPAmount   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	EIAmount    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TaxAmount   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	TotalAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	BankLedgerAccountID    *uint    `gorm:"index"`
	BankLedgerAccount      *Account `gorm:"foreignKey:BankLedgerAccountID"`
	JournalEntryID         *uint    `gorm:"index"`
	JournalEntry           *JournalEntry
	VoidedAt               *time.Time
	ReversalJournalEntryID *uint         `gorm:"index"`
	ReversalJournalEntry   *JournalEntry `gorm:"foreignKey:ReversalJournalEntryID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
