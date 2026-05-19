package producers

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestEmployeeDocumentOmitsSensitiveFields(t *testing.T) {
	dob := time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC)
	employee := models.Employee{
		ID:                   7,
		CompanyID:            1,
		EmployeeNo:           "E-007",
		LegalName:            "Avery Payroll",
		DisplayName:          "Avery",
		Email:                "avery@example.test",
		Mobile:               "555-0100",
		Position:             "Technician",
		AddrStreet1:          "123 Private St",
		ProvinceOfEmployment: "BC",
		SINCiphertext:        "encrypted-sin",
		SINLast4:             "1234",
		DateOfBirth:          &dob,
		Status:               models.EmployeeStatusActive,
		CreatedAt:            time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}

	doc := EmployeeDocument(employee)
	combined := strings.Join([]string{
		doc.DocNumber,
		doc.Title,
		doc.Subtitle,
		doc.Memo,
		doc.Amount,
		doc.Currency,
		doc.URLPath,
	}, " ")

	for _, forbidden := range []string{
		"avery@example.test",
		"555-0100",
		"123 Private St",
		"encrypted-sin",
		"1234",
		"1990",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("employee projection leaked sensitive field %q in %q", forbidden, combined)
		}
	}
	if !strings.Contains(combined, "Avery") || !strings.Contains(combined, "E-007") || !strings.Contains(combined, "Technician") {
		t.Fatalf("employee projection missing expected safe search fields: %+v", doc)
	}
}

func TestPayrollDocumentsUseExpectedEntityTypes(t *testing.T) {
	run := models.PayrollRun{
		ID:          8,
		CompanyID:   1,
		RunNumber:   "PAY-2026-01",
		PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
		TotalNetPay: decimal.NewFromInt(1200),
		Status:      models.PayrollRunCalculated,
	}
	if doc := PayrollRunDocument(run); doc.EntityType != EntityTypePayrollRun || doc.DocNumber != run.RunNumber {
		t.Fatalf("unexpected payroll run document: %+v", doc)
	}

	cheque := models.Cheque{
		ID:           9,
		CompanyID:    1,
		ChequeNumber: "000123",
		PayeeName:    "Avery Payroll",
		ChequeDate:   run.PayDate,
		Amount:       decimal.NewFromInt(1200),
		CurrencyCode: "CAD",
		Status:       models.ChequeStatusDraft,
	}
	if doc := ChequeDocument(cheque); doc.EntityType != EntityTypeCheque || doc.DocNumber != cheque.ChequeNumber {
		t.Fatalf("unexpected cheque document: %+v", doc)
	}

	remittance := models.PayrollRemittance{
		ID:               10,
		CompanyID:        1,
		PayrollRunID:     run.ID,
		PayrollRun:       run,
		RemittanceNumber: "PAY-2026-01",
		DueDate:          time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		TotalAmount:      decimal.NewFromInt(460),
		Status:           models.PayrollRemittanceDraft,
	}
	doc := PayrollRemittanceDocument(remittance)
	if doc.EntityType != EntityTypePayrollRemittance || doc.DocNumber != remittance.RemittanceNumber || doc.Amount != "460.00" {
		t.Fatalf("unexpected remittance document: %+v", doc)
	}
	if strings.Contains(strings.Join([]string{doc.Title, doc.Subtitle, doc.URLPath}, " "), "Bank") {
		t.Fatalf("remittance projection should not include bank details: %+v", doc)
	}
}
