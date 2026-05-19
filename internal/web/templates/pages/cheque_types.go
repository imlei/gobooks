package pages

import "balanciz/internal/models"

type ChequesVM struct {
	HasCompany bool

	Cheques        []models.Cheque
	BankAccounts   []models.ChequeBankAccount
	GLBankAccounts []models.Account
	Query          string
	Status         string

	Created        bool
	AccountCreated bool
	Printed        bool
	Voided         bool
	FormError      string
	CanPrint       bool
	CanManageBank  bool
}
