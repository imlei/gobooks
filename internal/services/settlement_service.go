// 遵循project_guide.md
package services

// settlement_service.go — Platform-agnostic settlement/fee staging layer.
// Provides CRUD for settlements, accounting mapping CRUD, and the suggested
// account resolver that auto-maps settlement line types to GL accounts.

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gobooks/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ── Channel order workflow status (derived) ──────────────────────────────────

type OrderWorkflowStatus string

const (
	OrderWorkflowConverted OrderWorkflowStatus = "converted"
	OrderWorkflowReady     OrderWorkflowStatus = "ready"
	OrderWorkflowBlocked   OrderWorkflowStatus = "blocked"
)

// DeriveOrderWorkflowStatus computes the workflow state from an order + its lines.
func DeriveOrderWorkflowStatus(order models.ChannelOrder, lines []models.ChannelOrderLine) OrderWorkflowStatus {
	if order.ConvertedInvoiceID != nil {
		return OrderWorkflowConverted
	}
	if len(lines) == 0 {
		return OrderWorkflowBlocked
	}
	for _, l := range lines {
		if l.MappingStatus != models.MappingStatusMappedExact && l.MappingStatus != models.MappingStatusMappedBundle {
			return OrderWorkflowBlocked
		}
	}
	return OrderWorkflowReady
}

// ── Accounting Mapping CRUD ──────────────────────────────────────────────────

func GetAccountingMapping(db *gorm.DB, companyID, channelAccountID uint) (*models.ChannelAccountingMapping, error) {
	var m models.ChannelAccountingMapping
	err := db.
		Preload("ClearingAccount").Preload("FeeExpenseAccount").
		Preload("RefundAccount").Preload("ShippingIncomeAccount").
		Preload("ShippingExpenseAccount").Preload("MarketplaceTaxAccount").
		Where("company_id = ? AND channel_account_id = ?", companyID, channelAccountID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func SaveAccountingMapping(db *gorm.DB, m *models.ChannelAccountingMapping) error {
	var existing models.ChannelAccountingMapping
	err := db.Where("company_id = ? AND channel_account_id = ?", m.CompanyID, m.ChannelAccountID).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(m).Error
	}
	if err != nil {
		return err
	}
	m.ID = existing.ID
	return db.Save(m).Error
}

// ── Settlement CRUD ──────────────────────────────────────────────────────────

func ListSettlements(db *gorm.DB, companyID uint, limit int) ([]models.ChannelSettlement, error) {
	if limit <= 0 {
		limit = 50
	}
	var settlements []models.ChannelSettlement
	err := db.Preload("ChannelAccount").
		Where("company_id = ?", companyID).
		Order("created_at DESC").
		Limit(limit).
		Find(&settlements).Error
	return settlements, err
}

func GetSettlement(db *gorm.DB, companyID, id uint) (*models.ChannelSettlement, error) {
	var s models.ChannelSettlement
	err := db.Preload("ChannelAccount").
		Where("id = ? AND company_id = ?", id, companyID).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func GetSettlementLines(db *gorm.DB, companyID, settlementID uint) ([]models.ChannelSettlementLine, error) {
	var lines []models.ChannelSettlementLine
	err := db.Preload("MappedAccount").
		Where("company_id = ? AND settlement_id = ?", companyID, settlementID).
		Order("id ASC").
		Find(&lines).Error
	return lines, err
}

// CreateSettlementWithLines creates a settlement and its lines in a transaction.
// Lines are auto-mapped to GL accounts using the channel accounting mapping.
// Header totals (gross/fee/net) are auto-recomputed from lines for consistency.
func CreateSettlementWithLines(db *gorm.DB, settlement *models.ChannelSettlement, lines []models.ChannelSettlementLine) error {
	return db.Transaction(func(tx *gorm.DB) error {
		settlement.CreatedAt = time.Now()
		settlement.UpdatedAt = time.Now()
		if err := tx.Create(settlement).Error; err != nil {
			return fmt.Errorf("create settlement: %w", err)
		}

		// Load accounting mapping for auto-assign.
		mapping, _ := GetAccountingMapping(tx, settlement.CompanyID, settlement.ChannelAccountID)

		for i := range lines {
			lines[i].CompanyID = settlement.CompanyID
			lines[i].SettlementID = settlement.ID
			lines[i].CreatedAt = time.Now()
			if lines[i].RawPayload == nil {
				lines[i].RawPayload = datatypes.JSON("{}")
			}

			// Auto-map account based on line type + accounting mapping.
			if lines[i].MappedAccountID == nil && mapping != nil {
				lines[i].MappedAccountID = SuggestAccountForLineType(mapping, lines[i].LineType)
			}

			if err := tx.Create(&lines[i]).Error; err != nil {
				return fmt.Errorf("create settlement line %d: %w", i+1, err)
			}
		}

		// Auto-recompute header totals from lines.
		var gross, fees decimal.Decimal
		for _, l := range lines {
			switch l.LineType {
			case models.SettlementLineSale:
				gross = gross.Add(l.Amount)
			case models.SettlementLineFee, models.SettlementLineShippingFee:
				fees = fees.Add(l.Amount.Abs())
			}
		}
		return tx.Model(&models.ChannelSettlement{}).
			Where("id = ?", settlement.ID).
			Updates(map[string]any{
				"gross_amount": gross,
				"fee_amount":   fees,
				"net_amount":   gross.Sub(fees),
			}).Error
	})
}

// ── Suggested account mapping ────────────────────────────────────────────────

// SuggestAccountForLineType returns the GL account ID that should be used for a
// given settlement line type, based on the channel's accounting mapping config.
func SuggestAccountForLineType(mapping *models.ChannelAccountingMapping, lineType models.SettlementLineType) *uint {
	if mapping == nil {
		return nil
	}
	switch lineType {
	case models.SettlementLineSale:
		return mapping.ClearingAccountID
	case models.SettlementLineFee:
		return mapping.FeeExpenseAccountID
	case models.SettlementLineShippingFee:
		return mapping.ShippingExpenseAccountID
	case models.SettlementLineRefund:
		return mapping.RefundAccountID
	case models.SettlementLinePayout:
		return mapping.ClearingAccountID
	case models.SettlementLineAdjustment:
		return mapping.ClearingAccountID
	case models.SettlementLineReserve:
		return mapping.ClearingAccountID
	default:
		return nil
	}
}

// CountUnmappedLines returns the number of settlement lines without a mapped account.
func CountUnmappedLines(db *gorm.DB, companyID, settlementID uint) int64 {
	var count int64
	db.Model(&models.ChannelSettlementLine{}).
		Where("company_id = ? AND settlement_id = ? AND mapped_account_id IS NULL", companyID, settlementID).
		Count(&count)
	return count
}
