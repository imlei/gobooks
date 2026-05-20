package web

import (
	"fmt"
	"strings"

	"balanciz/internal/models"
)

type smartPickerContextDefinition struct {
	Context         string
	ProviderContext string
	EntityType      string
	Description     string
	RequiredFeature models.FeatureKey
	RequiredAction  string
}

var smartPickerContextDefinitions = map[string]smartPickerContextDefinition{}

func init() {
	for _, def := range []smartPickerContextDefinition{
		{Context: "expense_form_category", ProviderContext: "expense_form_category", EntityType: "account", Description: "expense category account", RequiredAction: ActionBillView},
		{Context: "expense.category_picker", ProviderContext: "expense_form_category", EntityType: "account", Description: "expense category account", RequiredAction: ActionBillView},
		{Context: "journal_entry_account", ProviderContext: "journal_entry_account", EntityType: "account", Description: "journal entry posting account", RequiredAction: ActionJournalView},
		{Context: "journal_entry.account_picker", ProviderContext: "journal_entry_account", EntityType: "account", Description: "journal entry posting account", RequiredAction: ActionJournalView},
		{Context: "expense_form_payment", ProviderContext: "expense_form_payment", EntityType: "payment_account", Description: "expense payment account", RequiredAction: ActionBillView},
		{Context: "expense.payment_account_picker", ProviderContext: "expense_form_payment", EntityType: "payment_account", Description: "expense payment account", RequiredAction: ActionBillView},
		{Context: "company.switcher", ProviderContext: "company.switcher", EntityType: "company", Description: "company switcher"},

		{Context: "invoice_editor_customer", ProviderContext: "invoice_editor_customer", EntityType: "customer", Description: "invoice customer", RequiredAction: ActionInvoiceView},
		{Context: "invoice_customer", ProviderContext: "invoice_editor_customer", EntityType: "customer", Description: "invoice customer", RequiredAction: ActionInvoiceView},
		{Context: "invoice.customer_picker", ProviderContext: "invoice_editor_customer", EntityType: "customer", Description: "invoice customer", RequiredAction: ActionInvoiceView},
		{Context: "quote.customer_picker", ProviderContext: "quote.customer_picker", EntityType: "customer", Description: "quote customer", RequiredAction: ActionInvoiceView},
		{Context: "task_form_customer", ProviderContext: "task_form_customer", EntityType: "customer", Description: "task customer", RequiredFeature: models.FeatureKeyTask, RequiredAction: ActionTaskView},
		{Context: "task.customer_picker", ProviderContext: "task_form_customer", EntityType: "customer", Description: "task customer", RequiredFeature: models.FeatureKeyTask, RequiredAction: ActionTaskView},
		{Context: "invoices_filter", ProviderContext: "invoices_filter", EntityType: "customer", Description: "invoice list customer filter", RequiredAction: ActionInvoiceView},
		{Context: "quotes_filter", ProviderContext: "quotes_filter", EntityType: "customer", Description: "quote list customer filter", RequiredAction: ActionInvoiceView},
		{Context: "sales_orders_filter", ProviderContext: "sales_orders_filter", EntityType: "customer", Description: "sales order customer filter", RequiredAction: ActionInvoiceView},
		{Context: "receipts_filter", ProviderContext: "receipts_filter", EntityType: "customer", Description: "receipt customer filter", RequiredAction: ActionInvoiceView},
		{Context: "refunds_filter", ProviderContext: "refunds_filter", EntityType: "customer", Description: "refund customer filter", RequiredAction: ActionInvoiceView},
		{Context: "returns_filter", ProviderContext: "returns_filter", EntityType: "customer", Description: "return customer filter", RequiredAction: ActionInvoiceView},
		{Context: "deposits_filter", ProviderContext: "deposits_filter", EntityType: "customer", Description: "deposit customer filter", RequiredAction: ActionInvoiceView},

		{Context: "expense_form_vendor", ProviderContext: "expense_form_vendor", EntityType: "vendor", Description: "expense vendor", RequiredAction: ActionBillView},
		{Context: "expense_vendor", ProviderContext: "expense_form_vendor", EntityType: "vendor", Description: "expense vendor", RequiredAction: ActionBillView},
		{Context: "expense.vendor_picker", ProviderContext: "expense_form_vendor", EntityType: "vendor", Description: "expense vendor", RequiredAction: ActionBillView},
		{Context: "bill.vendor_picker", ProviderContext: "bill.vendor_picker", EntityType: "vendor", Description: "bill vendor", RequiredAction: ActionBillView},
		{Context: "bills_filter", ProviderContext: "bills_filter", EntityType: "vendor", Description: "bill list vendor filter", RequiredAction: ActionBillView},
		{Context: "purchase_orders_filter", ProviderContext: "purchase_orders_filter", EntityType: "vendor", Description: "purchase order vendor filter", RequiredAction: ActionBillView},
		{Context: "vendor_credit_notes_filter", ProviderContext: "vendor_credit_notes_filter", EntityType: "vendor", Description: "vendor credit note filter", RequiredAction: ActionBillView},
		{Context: "vendor_prepayments_filter", ProviderContext: "vendor_prepayments_filter", EntityType: "vendor", Description: "vendor prepayment filter", RequiredAction: ActionBillView},
		{Context: "vendor_refunds_filter", ProviderContext: "vendor_refunds_filter", EntityType: "vendor", Description: "vendor refund filter", RequiredAction: ActionBillView},
		{Context: "vendor_returns_filter", ProviderContext: "vendor_returns_filter", EntityType: "vendor", Description: "vendor return filter", RequiredAction: ActionBillView},

		{Context: "invoice_line_item", ProviderContext: "invoice_line_item", EntityType: "product_service", Description: "invoice product or service", RequiredAction: ActionInvoiceView},
		{Context: "invoice.item_picker", ProviderContext: "invoice_line_item", EntityType: "product_service", Description: "invoice product or service", RequiredAction: ActionInvoiceView},
		{Context: "quote_line_item", ProviderContext: "quote_line_item", EntityType: "product_service", Description: "quote product or service", RequiredAction: ActionInvoiceView},
		{Context: "quote.item_picker", ProviderContext: "quote_line_item", EntityType: "product_service", Description: "quote product or service", RequiredAction: ActionInvoiceView},
		{Context: "sales_order_line_item", ProviderContext: "sales_order_line_item", EntityType: "product_service", Description: "sales order product or service", RequiredAction: ActionInvoiceView},
		{Context: "sales_order.item_picker", ProviderContext: "sales_order_line_item", EntityType: "product_service", Description: "sales order product or service", RequiredAction: ActionInvoiceView},
		{Context: "po_line_item", ProviderContext: "po_line_item", EntityType: "product_service", Description: "purchase order product or service", RequiredAction: ActionBillView},
		{Context: "task_form_service_item", ProviderContext: "task_form_service_item", EntityType: "product_service", Description: "task service item", RequiredFeature: models.FeatureKeyTask, RequiredAction: ActionTaskView},
		{Context: "task.service_item_picker", ProviderContext: "task_form_service_item", EntityType: "product_service", Description: "task service item", RequiredFeature: models.FeatureKeyTask, RequiredAction: ActionTaskView},
	} {
		smartPickerContextDefinitions[def.Context] = def
	}
}

func validateSmartPickerContext(entityType, context string) (smartPickerContextDefinition, error) {
	entityType = strings.TrimSpace(entityType)
	context = strings.TrimSpace(context)
	if entityType == "" {
		return smartPickerContextDefinition{}, fmt.Errorf("entity type is required")
	}
	if context == "" {
		context = defaultSmartPickerContext(entityType)
	}
	def, ok := smartPickerContextDefinitions[context]
	if !ok {
		return smartPickerContextDefinition{}, fmt.Errorf("invalid smart picker context: %s", context)
	}
	if def.EntityType != entityType {
		return smartPickerContextDefinition{}, fmt.Errorf("context %s does not allow entity type %s", context, entityType)
	}
	return def, nil
}

func defaultSmartPickerContext(entityType string) string {
	switch strings.TrimSpace(entityType) {
	case "account":
		return "expense_form_category"
	case "customer":
		return "invoice_editor_customer"
	case "vendor":
		return "expense_form_vendor"
	case "product_service":
		return "invoice_line_item"
	case "payment_account":
		return "expense_form_payment"
	case "company":
		return "company.switcher"
	default:
		return ""
	}
}

func normalizeSmartPickerQuery(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(q)), " "))
}

func validSmartPickerEventType(eventType string) bool {
	switch eventType {
	case "search", "impression", "select", "create_new", "no_match", "abandon", "clear", "override":
		return true
	default:
		return false
	}
}
