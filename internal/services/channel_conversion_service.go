// 遵循project_guide.md
package services

// channel_conversion_service.go — Convert staged channel orders to draft invoices.
//
// The conversion flow:
//   1. Validate eligibility (all lines mapped, not already converted)
//   2. Create draft invoice with lines mapped from channel order lines
//   3. Mark channel order as converted (set converted_invoice_id)
//
// Tax handling (conservative strategy):
//   Line tax is NOT force-mapped from channel order raw tax_amount.
//   Instead, each invoice line uses the mapped item's DefaultTaxCodeID.
//   The Gobooks tax engine recalculates tax on invoice save/post.
//   This prevents corrupting the tax engine with platform-specific tax values.
//
// Discount handling:
//   Current invoice lines have no line-level discount field.
//   Channel order discounts are preserved in the raw order layer only.
//   The user can adjust line prices on the draft invoice if needed.
//
// Revenue account:
//   Each invoice line inherits its mapped item's RevenueAccountID.
//
// Bundle handling:
//   Bundle items are placed on the invoice line as-is.
//   Component-level inventory explode happens at posting time (existing logic).

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gobooks/internal/models"
	"gorm.io/gorm"
)

var (
	ErrOrderAlreadyConverted = errors.New("channel order has already been converted to an invoice")
	ErrOrderNotConvertible   = errors.New("channel order has unmapped or ambiguous lines — resolve all mappings first")
	ErrOrderNoLines          = errors.New("channel order has no lines")
)

// ConvertOptions holds parameters for the conversion.
type ConvertOptions struct {
	CompanyID      uint
	ChannelOrderID uint
	CustomerID     uint   // required — Gobooks invoices must have a customer
	InvoiceNumber  string // if empty, caller must provide or use numbering service
	InvoiceDate    time.Time
	Memo           string
}

// ConvertResult holds the outcome of a successful conversion.
type ConvertResult struct {
	InvoiceID     uint
	InvoiceNumber string
	LineCount     int
}

// ValidateChannelOrderConvertible checks whether a channel order can be converted.
// Returns nil if convertible; descriptive error otherwise.
func ValidateChannelOrderConvertible(db *gorm.DB, companyID, orderID uint) error {
	order, err := GetChannelOrder(db, companyID, orderID)
	if err != nil {
		return fmt.Errorf("order not found")
	}

	if order.ConvertedInvoiceID != nil {
		return ErrOrderAlreadyConverted
	}

	lines, err := GetChannelOrderLines(db, companyID, orderID)
	if err != nil || len(lines) == 0 {
		return ErrOrderNoLines
	}

	for _, l := range lines {
		switch l.MappingStatus {
		case models.MappingStatusMappedExact, models.MappingStatusMappedBundle:
			// OK
		default:
			return fmt.Errorf("%w: line SKU %q is %s",
				ErrOrderNotConvertible, l.ExternalSKU, models.ChannelMappingStatusLabel(l.MappingStatus))
		}
		// Verify mapped item still exists and is active.
		if l.MappedItemID != nil {
			var item models.ProductService
			if err := db.Where("id = ? AND company_id = ? AND is_active = true",
				*l.MappedItemID, companyID).First(&item).Error; err != nil {
				return fmt.Errorf("mapped item for SKU %q is inactive or missing", l.ExternalSKU)
			}
		}
	}

	return nil
}

// ConvertChannelOrderToDraftInvoice creates a draft invoice from a channel order.
// All lines must be mapped. The invoice is NOT posted.
func ConvertChannelOrderToDraftInvoice(db *gorm.DB, opts ConvertOptions) (*ConvertResult, error) {
	// 1. Validate.
	if err := ValidateChannelOrderConvertible(db, opts.CompanyID, opts.ChannelOrderID); err != nil {
		return nil, err
	}

	order, _ := GetChannelOrder(db, opts.CompanyID, opts.ChannelOrderID)
	lines, _ := GetChannelOrderLines(db, opts.CompanyID, opts.ChannelOrderID)

	// 2. Load mapped items for invoice line creation.
	itemCache := map[uint]*models.ProductService{}
	for _, l := range lines {
		if l.MappedItemID == nil {
			continue
		}
		if _, ok := itemCache[*l.MappedItemID]; ok {
			continue
		}
		var item models.ProductService
		if err := db.Where("id = ? AND company_id = ?", *l.MappedItemID, opts.CompanyID).
			First(&item).Error; err == nil {
			itemCache[item.ID] = &item
		}
	}

	// 3. Load customer for snapshots.
	var customer models.Customer
	if err := db.Where("id = ? AND company_id = ?", opts.CustomerID, opts.CompanyID).
		First(&customer).Error; err != nil {
		return nil, fmt.Errorf("customer not found")
	}

	// 4. Build invoice + lines.
	invoiceDate := opts.InvoiceDate
	if invoiceDate.IsZero() {
		if order.OrderDate != nil {
			invoiceDate = *order.OrderDate
		} else {
			invoiceDate = time.Now()
		}
	}

	memo := opts.Memo
	if memo == "" {
		memo = "Converted from channel order " + order.ExternalOrderID
	}

	// 5. Transaction: create invoice, lines, mark order converted.
	var result ConvertResult
	err := db.Transaction(func(tx *gorm.DB) error {
		// Compute line amounts.
		var subtotal, taxTotal decimal.Decimal
		var invoiceLines []models.InvoiceLine

		for i, ol := range lines {
			if ol.MappedItemID == nil {
				continue
			}
			item := itemCache[*ol.MappedItemID]
			if item == nil {
				continue
			}

			unitPrice := decimal.Zero
			if ol.ItemPrice != nil {
				unitPrice = *ol.ItemPrice
			}
			lineNet := ol.Quantity.Mul(unitPrice).RoundBank(2)

			// Tax: use item's default tax code; tax amount recalculated by invoice engine.
			var lineTax decimal.Decimal
			if item.DefaultTaxCodeID != nil {
				var tc models.TaxCode
				if err := tx.Where("id = ?", *item.DefaultTaxCodeID).First(&tc).Error; err == nil {
					taxResults := CalculateTax(lineNet, tc)
					lineTax = SumTaxResults(taxResults)
				}
			}

			invoiceLines = append(invoiceLines, models.InvoiceLine{
				CompanyID:        opts.CompanyID,
				SortOrder:        uint(i + 1),
				ProductServiceID: ol.MappedItemID,
				Description:      item.Name,
				Qty:              ol.Quantity,
				UnitPrice:        unitPrice,
				TaxCodeID:        item.DefaultTaxCodeID,
				LineNet:          lineNet,
				LineTax:          lineTax,
				LineTotal:        lineNet.Add(lineTax),
			})

			subtotal = subtotal.Add(lineNet)
			taxTotal = taxTotal.Add(lineTax)
		}

		amount := subtotal.Add(taxTotal)

		channelOrderID := opts.ChannelOrderID
		inv := models.Invoice{
			CompanyID:               opts.CompanyID,
			InvoiceNumber:           opts.InvoiceNumber,
			CustomerID:              opts.CustomerID,
			InvoiceDate:             invoiceDate,
			Status:                  models.InvoiceStatusDraft,
			ChannelOrderID:          &channelOrderID,
			Subtotal:                subtotal,
			TaxTotal:                taxTotal,
			Amount:                  amount,
			BalanceDue:              amount,
			Memo:                    memo,
			CurrencyCode:            order.CurrencyCode,
			CustomerNameSnapshot:    customer.Name,
			CustomerEmailSnapshot:   customer.Email,
			CustomerAddressSnapshot: customer.FormattedAddress(),
		}

		if err := tx.Create(&inv).Error; err != nil {
			return fmt.Errorf("create invoice: %w", err)
		}

		for j := range invoiceLines {
			invoiceLines[j].InvoiceID = inv.ID
			if err := tx.Create(&invoiceLines[j]).Error; err != nil {
				return fmt.Errorf("create invoice line %d: %w", j+1, err)
			}
		}

		// Mark order as converted.
		if err := tx.Model(&models.ChannelOrder{}).
			Where("id = ? AND company_id = ?", opts.ChannelOrderID, opts.CompanyID).
			Update("converted_invoice_id", inv.ID).Error; err != nil {
			return fmt.Errorf("mark order converted: %w", err)
		}

		result = ConvertResult{
			InvoiceID:     inv.ID,
			InvoiceNumber: inv.InvoiceNumber,
			LineCount:     len(invoiceLines),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConvertedInvoiceID returns the invoice ID if the order has been converted.
func GetConvertedInvoiceID(db *gorm.DB, companyID, orderID uint) *uint {
	var order models.ChannelOrder
	if err := db.Select("id", "converted_invoice_id").
		Where("id = ? AND company_id = ?", orderID, companyID).
		First(&order).Error; err != nil {
		return nil
	}
	return order.ConvertedInvoiceID
}
