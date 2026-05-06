// 遵循project_guide.md
package services

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

// WarehouseQueueItem is a compact work item shown on the warehouse dashboard.
type WarehouseQueueItem struct {
	ID           uint
	Number       string
	Counterparty string
	DueDate      *time.Time
	Status       string
	Amount       decimal.Decimal
	CurrencyCode string
	Href         string
}

// WarehouseQueueSummary groups the inbound and outbound warehouse work queues.
type WarehouseQueueSummary struct {
	WaitingToReceive []WarehouseQueueItem
	WaitingToShip    []WarehouseQueueItem
}

// GetWarehouseQueueSummary returns the open inbound and outbound work that most
// directly affects warehouse operations.
func GetWarehouseQueueSummary(db *gorm.DB, companyID uint, limit int) (*WarehouseQueueSummary, error) {
	if limit <= 0 {
		limit = 5
	}

	var pos []models.PurchaseOrder
	if err := db.Preload("Vendor").
		Where("company_id = ? AND status IN ?", companyID, []models.POStatus{
			models.POStatusConfirmed,
			models.POStatusPartiallyReceived,
		}).
		Order("CASE WHEN expected_date IS NULL THEN 1 ELSE 0 END").
		Order("expected_date ASC").
		Order("id DESC").
		Limit(limit).
		Find(&pos).Error; err != nil {
		return nil, err
	}

	var sos []models.SalesOrder
	if err := db.Preload("Customer").
		Where("company_id = ? AND status IN ?", companyID, []models.SalesOrderStatus{
			models.SalesOrderStatusConfirmed,
			models.SalesOrderStatusPartiallyInvoiced,
		}).
		Order("CASE WHEN required_by IS NULL THEN 1 ELSE 0 END").
		Order("required_by ASC").
		Order("id DESC").
		Limit(limit).
		Find(&sos).Error; err != nil {
		return nil, err
	}

	summary := &WarehouseQueueSummary{
		WaitingToReceive: make([]WarehouseQueueItem, 0, len(pos)),
		WaitingToShip:    make([]WarehouseQueueItem, 0, len(sos)),
	}
	for _, po := range pos {
		summary.WaitingToReceive = append(summary.WaitingToReceive, WarehouseQueueItem{
			ID:           po.ID,
			Number:       po.PONumber,
			Counterparty: po.Vendor.Name,
			DueDate:      po.ExpectedDate,
			Status:       models.POStatusLabel(po.Status),
			Amount:       po.Amount,
			CurrencyCode: po.CurrencyCode,
			Href:         fmt.Sprintf("/purchase-orders/%d", po.ID),
		})
	}
	for _, so := range sos {
		summary.WaitingToShip = append(summary.WaitingToShip, WarehouseQueueItem{
			ID:           so.ID,
			Number:       so.OrderNumber,
			Counterparty: so.Customer.Name,
			DueDate:      so.RequiredBy,
			Status:       models.SalesOrderStatusLabel(so.Status),
			Amount:       so.Total,
			CurrencyCode: so.CurrencyCode,
			Href:         fmt.Sprintf("/sales-orders/%d", so.ID),
		})
	}

	return summary, nil
}
