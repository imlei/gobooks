// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// TaxFamily identifies the tax type for compound calculation ordering.
// Retained for TaxComponent; not used by the flat TaxCode design.
type TaxFamily string

const (
	TaxFamilyGST TaxFamily = "gst"
	TaxFamilyHST TaxFamily = "hst"
	TaxFamilyQST TaxFamily = "qst" // Québec: compound base = netAmount + GST
	TaxFamilyPST TaxFamily = "pst"
	TaxFamilyRST TaxFamily = "rst"
)

// TaxScope controls which transaction direction a TaxCode applies to.
type TaxScope string

const (
	TaxScopeSales    TaxScope = "sales"
	TaxScopePurchase TaxScope = "purchase"
	TaxScopeBoth     TaxScope = "both"
)

// TaxRecoveryMode controls how much purchase tax is recoverable as an Input Tax Credit (ITC).
type TaxRecoveryMode string

const (
	TaxRecoveryFull    TaxRecoveryMode = "full"
	TaxRecoveryPartial TaxRecoveryMode = "partial"
	TaxRecoveryNone    TaxRecoveryMode = "none"
)

// TaxAgency is the government authority that collects the tax (e.g. CRA, Revenu Québec).
// Company-scoped so each company can maintain its own agency records.
type TaxAgency struct {
	ID        uint   `gorm:"primaryKey"`
	CompanyID uint   `gorm:"not null;index"`
	Name      string `gorm:"not null"`      // "Canada Revenue Agency"
	ShortCode string `gorm:"not null"`      // "CRA", "RQ"
	IsActive  bool   `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TaxComponent is a single rate element: GST 5%, ON-HST 13%, QST 9.975%, BC-PST 7%, etc.
// Each component posts to exactly one liability account when an invoice is posted.
//
// Province: empty string = federal (GST, HST); province code = provincial (QST, PST, RST).
// IsRecoverable: true = eligible for input tax credits (ITC) on the purchasing side.
type TaxComponent struct {
	ID                 uint            `gorm:"primaryKey"`
	CompanyID          uint            `gorm:"not null;index"`
	Name               string          `gorm:"not null"` // "GST", "ON-HST", "QST", "BC-PST"
	TaxFamily          TaxFamily       `gorm:"type:text;not null"`
	TaxAgencyID        uint            `gorm:"not null;index"`
	TaxAgency          TaxAgency       `gorm:"foreignKey:TaxAgencyID"`
	Rate               decimal.Decimal `gorm:"type:numeric(8,6);not null"` // e.g. 0.050000
	LiabilityAccountID uint            `gorm:"not null;index"`
	LiabilityAccount   Account         `gorm:"foreignKey:LiabilityAccountID"`
	Province           string          `gorm:"not null;default:''"` // "BC","QC","ON"; "" = federal
	IsRecoverable      bool            `gorm:"not null;default:true"`
	IsActive           bool            `gorm:"not null;default:true"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// TaxCode is the user-facing tax code applied to invoice/bill lines.
//
// Flat-rate design: rate is carried directly on TaxCode, no component bridge needed.
//
//   - Scope: which transaction direction the code applies to (sales / purchase / both).
//   - RecoveryMode / RecoveryRate: how much of the purchase tax the company can reclaim
//     as an Input Tax Credit (ITC). RecoveryRate is a percentage (0–100).
//   - SalesTaxAccountID: the GL liability account credited when tax is collected on a sale
//     (e.g. "GST/HST Payable").
//   - PurchaseRecoverableAccountID: the GL asset/receivable account debited for the
//     recoverable portion of tax paid on purchases (e.g. "Input Tax Credits Receivable");
//     NULL when RecoveryMode is none or Scope is sales-only.
type TaxCode struct {
	ID                           uint            `gorm:"primaryKey"`
	CompanyID                    uint            `gorm:"not null;index:idx_tax_codes_company_active,priority:1"`
	Name                         string          `gorm:"not null"`
	// Code and TaxType are legacy columns from early migrations (005). Flat-rate tax uses Name + Rate only.
	// We set Code = Name and TaxType = "taxable" on write so older databases with NOT NULL code/tax_type still accept inserts.
	Code    string `gorm:"column:code;size:255;default:''"`
	TaxType string `gorm:"column:tax_type;size:32;default:'taxable'"`
	Rate                         decimal.Decimal `gorm:"type:numeric(8,6);not null"`
	Scope                        TaxScope        `gorm:"type:text;not null;default:'both'"`
	RecoveryMode                 TaxRecoveryMode `gorm:"type:text;not null;default:'none'"`
	RecoveryRate                 decimal.Decimal `gorm:"type:numeric(5,2);not null;default:0"` // 0–100 percentage
	SalesTaxAccountID            uint            `gorm:"not null;index"`
	SalesTaxAccount              Account         `gorm:"foreignKey:SalesTaxAccountID"`
	PurchaseRecoverableAccountID *uint           `gorm:"index"`
	PurchaseRecoverableAccount   *Account        `gorm:"foreignKey:PurchaseRecoverableAccountID"`
	IsActive                     bool            `gorm:"not null;default:true;index:idx_tax_codes_company_active,priority:2"`
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}
