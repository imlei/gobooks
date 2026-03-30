// 遵循project_guide.md
package pages

import (
	"time"

	"gobooks/internal/models"
	"gobooks/internal/services"
)

type BankReconcileVM struct {
	HasCompany bool

	Accounts []models.Account

	AccountID string
	StatementDate string
	EndingBalance string

	Active string

	FormError string
	Saved bool

	PreviouslyCleared string

	Candidates []services.ReconcileCandidate

	StatementDateTime time.Time
}

