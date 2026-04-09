// 遵循project_guide.md
package pages

import (
	"gobooks/internal/models"
	"gobooks/internal/services"
)

// ── Batch 23: Payment Reverse Exception VMs ───────────────────────────────────

type PaymentReverseExceptionListVM struct {
	HasCompany   bool
	Exceptions   []models.PaymentReverseException
	JustActioned bool
	FormError    string
}

type PaymentReverseExceptionDetailVM struct {
	HasCompany   bool
	Exception    *models.PaymentReverseException
	JustActioned bool
	ActionError  string

	// Linked transaction summaries (nil if not found).
	ReverseTxn  *models.PaymentTransaction
	OriginalTxn *models.PaymentTransaction
}

type PaymentGatewaysVM struct {
	HasCompany bool
	Accounts   []services.GatewayAccountSummary
	Created    bool
	FormError  string
}

type PaymentMappingsVM struct {
	HasCompany      bool
	GatewayAccounts []models.PaymentGatewayAccount
	GLAccounts      []models.Account
	Mappings        map[uint]*models.PaymentAccountingMapping
	Saved           bool
	FormError       string
}

type PaymentRequestsVM struct {
	HasCompany bool
	Requests   []models.PaymentRequest
	Accounts   []models.PaymentGatewayAccount
	Created    bool
	FormError  string
}

type PaymentTransactionsVM struct {
	HasCompany   bool
	Transactions []models.PaymentTransaction
	Accounts     []models.PaymentGatewayAccount
	Created      bool
	JustPosted   bool

	// TxnStates maps txn_id → unified action state (accounting + application + actions).
	TxnStates map[uint]services.PaymentActionState

	JustApplied              bool
	JustRefundApplied        bool
	JustChargebackApplied    bool
	JustUnapplied            bool
	// Batch 22: multi-alloc reverse apply just completed.
	JustReverseAllocApplied  bool
	FormError                string

	// Batch 22: ReverseAllocations maps reverse_txn_id → its PaymentReverseAllocation rows.
	// Only populated for transactions where IsReverseAllocated == true.
	ReverseAllocations map[uint][]models.PaymentReverseAllocation
}
