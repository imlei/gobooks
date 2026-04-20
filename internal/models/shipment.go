// 遵循project_guide.md
package models

// shipment.go — Phase I slice I.2 outbound Shipment document
// (sell-side mirror of receipt.go from Phase H slice H.2).
//
// Role in Phase I
// ---------------
// Shipment is the first-class document that, starting in I.3, produces
// issue truth (Dr COGS / Cr Inventory) when posted under
// companies.shipment_required=true, and creates the
// `waiting_for_invoice` operational item that Invoice clears in I.5.
// In I.2 it is a document-layer shell only: Create / Read / Update
// (draft) / Post (status flip) / Void (status flip) / Delete (draft)
// exist, but Post and Void have zero side effects on inventory or GL.
// That wiring lands in I.3.
//
// Relationship to the shipment_required rail
// ------------------------------------------
// I.2 does NOT consult companies.shipment_required. Shipment creation
// and posting are allowed regardless of the flag. The flag's consumer
// is Invoice-side (I.4 — flag=true disables Invoice-forms-COGS) and
// matching (I.5). Shipment itself is flag-agnostic in I.2.
//
// Why there is no UnitCost on ShipmentLine
// ----------------------------------------
// Outbound cost is authoritative from the inventory module, never
// from the business-document layer. FIFO peels specific cost layers;
// moving-average reads the per-item running cost. Either way, the
// Shipment does not *declare* cost — it *consumes* inventory and
// receives back a cost figure at issue time. Adding a UnitCost column
// here would create a silent authority conflict: two sources of truth
// for the same number. I.3's IssueStockFromShipment returns
// unit_cost_base; that is the only value the JE builder may use.

import (
	"time"

	"github.com/shopspring/decimal"
)

// ShipmentStatus tracks the lifecycle of an outbound Shipment.
//
// Lifecycle (I.2 scope):
//
//	draft    → posted   (via PostShipment; requires Status==draft)
//	posted   → voided   (via VoidShipment; requires Status==posted)
//	draft    → deleted  (via DeleteShipment; requires Status==draft)
//
// Terminal states: voided. Deleted rows leave no document trace by
// design — drafts that never posted carry no audit obligation.
//
// I.2 locks the state machine at these transitions. Later slices
// (I.3 post, I.5 matching) bolt behavior onto Post/Void; they do
// NOT add new states without a dedicated slice and doc update.
type ShipmentStatus string

const (
	ShipmentStatusDraft  ShipmentStatus = "draft"
	ShipmentStatusPosted ShipmentStatus = "posted"
	ShipmentStatusVoided ShipmentStatus = "voided"
)

// AllShipmentStatuses returns shipment statuses in logical order.
func AllShipmentStatuses() []ShipmentStatus {
	return []ShipmentStatus{
		ShipmentStatusDraft,
		ShipmentStatusPosted,
		ShipmentStatusVoided,
	}
}

// Shipment is an outbound goods-shipment header.
//
// Identity: (CompanyID, ShipmentNumber) is intended to be unique in
// practice; the uniqueness is not enforced by a DB constraint in I.2
// — the service layer assigns numbers and numbering logic (with
// company number sequences) lands in a later slice. Blank
// ShipmentNumber is allowed for draft shipments that have not yet
// been assigned one.
//
// CustomerID is nullable because a shipment MAY represent an outbound
// event not yet attributed to a specific customer (e.g. a warehouse-
// recorded dispatch awaiting paperwork, or a dropship pick that will
// be attributed at close). I.3 will enforce customer presence at
// post-time if accounting requires it; I.2 leaves it optional to
// mirror Receipt's conservative stance.
//
// WarehouseID is required — a shipment has to leave from somewhere.
//
// SalesOrderID is a nullable reservation field for the Phase I
// SO → Shipment → Invoice identity chain. It is accepted on input
// and stored, but no consumer reads it in I.2.
type Shipment struct {
	ID uint `gorm:"primaryKey"`

	CompanyID uint `gorm:"not null;index"`

	// ShipmentNumber is the human-facing document identifier. Empty
	// string allowed for drafts. Numbering strategy (per-company
	// sequence, prefix) is owned by the service layer; the column
	// itself is just storage.
	ShipmentNumber string `gorm:"not null;default:''"`

	// CustomerID is nullable — shipment may be pre-customer-attribution.
	CustomerID *uint     `gorm:"index"`
	Customer   *Customer `gorm:"foreignKey:CustomerID"`

	// WarehouseID is required — the shipment leaves from here.
	WarehouseID uint       `gorm:"not null;index"`
	Warehouse   *Warehouse `gorm:"foreignKey:WarehouseID"`

	// ShipDate is the effective date of the outbound event (date
	// goods left the warehouse / carrier pickup). Kept separate from
	// CreatedAt so back-dated shipments are possible.
	ShipDate time.Time `gorm:"not null"`

	// Status carries the document lifecycle. Default 'draft' on create.
	Status ShipmentStatus `gorm:"type:text;not null;default:'draft'"`

	// Memo is free-form internal notes.
	Memo string `gorm:"not null;default:''"`

	// Reference is the external reference (carrier waybill / BOL /
	// tracking number) provided by the carrier or customer. Kept
	// separate from ShipmentNumber, which is the company-internal
	// document ID.
	Reference string `gorm:"not null;default:''"`

	// SalesOrderID is a Phase I source-identity reservation. I.2
	// stores but does not read. No FK constraint in I.2.
	SalesOrderID *uint `gorm:"index"`

	// Lifecycle timestamps. PostedAt is set once on the draft→posted
	// transition; VoidedAt is set once on the posted→voided transition.
	// Neither is cleared once set.
	PostedAt *time.Time
	VoidedAt *time.Time

	// JournalEntryID links this Shipment to the JE that booked its
	// issue (Dr COGS / Cr Inventory) at post time. Set only when
	// PostShipment ran under `companies.shipment_required=true` and
	// the shipment had at least one stock-item line. Nil means
	// either (a) posted under flag=false (I.2 byte-identical status
	// flip) or (b) no stock lines on the shipment.
	//
	// VoidShipment will use the presence of this link to decide
	// whether to reverse a JE + movements (non-nil) or to status-flip
	// only (nil). The column is deliberately not FK'd to
	// journal_entries at the schema layer, matching the existing
	// convention used by bills / receipts.
	JournalEntryID *uint         `gorm:"index"`
	JournalEntry   *JournalEntry `gorm:"foreignKey:JournalEntryID"`

	Lines []ShipmentLine `gorm:"foreignKey:ShipmentID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName maps Shipment to the `shipments` table. Explicit to avoid
// any accidental GORM default-pluralization drift and to make the
// binding obvious to readers scanning the model.
func (Shipment) TableName() string {
	return "shipments"
}

// ShipmentLine is one item / qty line on a Shipment.
//
// Qty is the quantity leaving the warehouse on this line. I.2 stores
// it but does not consume it; I.3's IssueStockFromShipment will pass
// it as the issue qty through the inventory OUT verb.
//
// There is deliberately NO UnitCost column: outbound cost is computed
// by the inventory module at issue time, not declared by the document.
// See the file-level comment on shipment.go for the reasoning.
//
// Lot / serial selections are also NOT carried on the line in I.2.
// I.3 will introduce the tracking-selection payload shape when the
// IssueStockFromShipment consumer is actually wired; pre-baking it
// here would commit to a schema before the use site has informed it.
//
// SalesOrderLineID is the line-level Phase I source-identity
// reservation. I.2 stores but does not read.
type ShipmentLine struct {
	ID         uint `gorm:"primaryKey"`
	CompanyID  uint `gorm:"not null;index"`
	ShipmentID uint `gorm:"not null;index"`

	SortOrder int `gorm:"not null;default:0"`

	ProductServiceID uint            `gorm:"not null;index"`
	ProductService   *ProductService `gorm:"foreignKey:ProductServiceID"`

	Description string `gorm:"not null;default:''"`

	Qty  decimal.Decimal `gorm:"type:numeric(18,6);not null;default:0"`
	Unit string          `gorm:"not null;default:''"`

	SalesOrderLineID *uint `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName maps ShipmentLine to the `shipment_lines` table.
func (ShipmentLine) TableName() string {
	return "shipment_lines"
}
