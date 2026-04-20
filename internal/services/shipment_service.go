// 遵循project_guide.md
package services

// shipment_service.go — Phase I slice I.2 CRUD + lifecycle for the
// outbound Shipment document (sell-side mirror of receipt_service.go
// at its H.2 shape).
//
// Scope lock (I.2)
// ----------------
// This file implements the document-layer surface for Shipment: it
// persists, reads, updates (draft only), lists, and flips status
// (draft→posted, posted→voided). It does NOT:
//
//   - Produce inventory movements
//   - Consume cost layers / read moving-average cost
//   - Call IssueStock / any inventory OUT verb
//   - Write a JournalEntry
//   - Touch COGS / Inventory accounts
//   - Create a waiting_for_invoice operational item
//   - Read companies.shipment_required
//   - Couple with Invoice or SalesOrder beyond storing reservation IDs
//   - Enforce source-identity linkage (SO references are stored only)
//
// PostShipment / VoidShipment in I.2 are pure status flips with audit.
// The consumer that turns Post into actual issue truth + COGS JE is
// IssueStockFromShipment in I.3; until then, posting a Shipment
// leaves inventory_* tables and the GL completely untouched. Tests
// in shipment_service_test.go lock this boundary at the CI level —
// any accidental I.3 slip shows up as a failed "no inventory effect"
// assertion.
//
// Audit surface
// -------------
// Post and Void each write exactly one audit row. Actions:
//   - `shipment.posted`   (draft → posted)
//   - `shipment.voided`   (posted → voided)
// Create / Update / Delete do NOT write audit in I.2 — they are
// standard document-layer CRUD on a pre-posting draft, with no
// cross-module state change worth recording. If a later slice needs
// draft-level audit (e.g. for regulated-industry compliance), it can
// bolt on without reshaping this surface.

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

// Error sentinels for the outbound Shipment document. Named plainly
// (`ErrShipment*`) because there is no existing Shipment-domain name
// to collide with — unlike Phase H's Receipt which had to namespace
// away from the AR-side CustomerReceipt family.
var (
	// ErrShipmentNotFound — lookup by (CompanyID, ID) missed.
	// Returned wrapped for errors.Is matching; handlers map to 404.
	ErrShipmentNotFound = errors.New("shipment: not found")

	// ErrShipmentNotDraft — attempted a draft-only operation
	// (update, delete) on a shipment that has moved out of draft.
	ErrShipmentNotDraft = errors.New("shipment: operation requires status=draft")

	// ErrShipmentNotPosted — attempted to void a shipment that is
	// not currently in posted status.
	ErrShipmentNotPosted = errors.New("shipment: void requires status=posted")

	// ErrShipmentAlreadyPosted — attempted to post a shipment that
	// is not in draft status (already posted, or already voided).
	ErrShipmentAlreadyPosted = errors.New("shipment: post requires status=draft")

	// ErrShipmentWarehouseRequired — a shipment has to leave from
	// somewhere; WarehouseID is required on create.
	ErrShipmentWarehouseRequired = errors.New("shipment: WarehouseID required")

	// ErrShipmentDateRequired — ShipDate required so that back-dated
	// shipments are deliberate rather than silent.
	ErrShipmentDateRequired = errors.New("shipment: ShipDate required")

	// ErrShipmentLineProductRequired — every line names a
	// product/service.
	ErrShipmentLineProductRequired = errors.New("shipment: line requires ProductServiceID")

	// ErrShipmentCrossCompanyReference — an ID on the input
	// (customer / warehouse / product_service / sales_order /
	// sales_order_line) resolves to a row that belongs to a
	// different company than the shipment being created or updated.
	// Rejected before any write so that the Shipment document cannot
	// establish a cross-tenant reference that I.3 / I.5 would later
	// have to detect after the fact. No FK constraint enforces this
	// at the schema layer (SO reservation is accepted without FK per
	// the I.2 scope lock), so the service layer is the boundary.
	ErrShipmentCrossCompanyReference = errors.New("shipment: referenced entity belongs to a different company")
)

// CreateShipmentInput captures the fields a caller may set when
// creating a new Shipment in draft state. Fields not listed here are
// populated by the service (ID, CreatedAt/UpdatedAt, Status='draft').
type CreateShipmentInput struct {
	CompanyID      uint
	ShipmentNumber string
	CustomerID     *uint
	WarehouseID    uint
	ShipDate       time.Time
	Memo           string
	Reference      string
	SalesOrderID   *uint

	Lines []CreateShipmentLineInput

	// Audit actor (only consumed by Post/Void; stored here for
	// callers that prefer to thread actor through the input struct
	// uniformly).
	Actor       string
	ActorUserID *uuid.UUID
}

// CreateShipmentLineInput captures the fields a caller may set on a
// shipment line at creation time. Note the absence of UnitCost — see
// the file-level comment and models/shipment.go on why outbound cost
// is authoritative from the inventory module, never declared here.
type CreateShipmentLineInput struct {
	SortOrder        int
	ProductServiceID uint
	Description      string
	Qty              decimal.Decimal
	Unit             string
	SalesOrderLineID *uint
}

// UpdateShipmentInput captures the mutable fields on a draft Shipment.
// Lines are replaced wholesale when ReplaceLines is true (nil Lines
// with ReplaceLines=true clears all lines); ReplaceLines=false leaves
// lines untouched regardless of the Lines field. Status is not a
// mutable field — Post/Void own the status transitions.
type UpdateShipmentInput struct {
	ShipmentNumber *string
	CustomerID     *uint
	WarehouseID    *uint
	ShipDate       *time.Time
	Memo           *string
	Reference      *string
	SalesOrderID   *uint
	Lines          []CreateShipmentLineInput
	ReplaceLines   bool
}

// ListShipmentsFilter narrows a company's shipments by status and
// date window. All fields optional; zero-values mean "unfiltered on
// this dimension".
type ListShipmentsFilter struct {
	Status     ShipmentStatus
	FromDate   *time.Time
	ToDate     *time.Time
	CustomerID *uint
	Limit      int
	Offset     int
}

// ShipmentStatus mirrors models.ShipmentStatus to keep service callers
// from needing to import models directly for the filter struct.
type ShipmentStatus = models.ShipmentStatus

// CreateShipment persists a new Shipment and its lines in draft state.
// Runs in a single transaction. Returns the created Shipment with
// lines populated.
func CreateShipment(db *gorm.DB, in CreateShipmentInput) (*models.Shipment, error) {
	if in.CompanyID == 0 {
		return nil, fmt.Errorf("services.CreateShipment: CompanyID required")
	}
	if in.WarehouseID == 0 {
		return nil, ErrShipmentWarehouseRequired
	}
	if in.ShipDate.IsZero() {
		return nil, ErrShipmentDateRequired
	}
	for i, ln := range in.Lines {
		if ln.ProductServiceID == 0 {
			return nil, fmt.Errorf("%w: line[%d]", ErrShipmentLineProductRequired, i)
		}
	}

	var created models.Shipment
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := validateShipmentHeaderScope(tx, in.CompanyID,
			in.CustomerID, in.WarehouseID, in.SalesOrderID); err != nil {
			return err
		}
		if err := validateShipmentLinesScope(tx, in.CompanyID, in.Lines); err != nil {
			return err
		}

		s := models.Shipment{
			CompanyID:      in.CompanyID,
			ShipmentNumber: in.ShipmentNumber,
			CustomerID:     in.CustomerID,
			WarehouseID:    in.WarehouseID,
			ShipDate:       in.ShipDate,
			Status:         models.ShipmentStatusDraft,
			Memo:           in.Memo,
			Reference:      in.Reference,
			SalesOrderID:   in.SalesOrderID,
		}
		if err := tx.Create(&s).Error; err != nil {
			return fmt.Errorf("create shipment: %w", err)
		}

		for _, ln := range in.Lines {
			sl := models.ShipmentLine{
				CompanyID:        in.CompanyID,
				ShipmentID:       s.ID,
				SortOrder:        ln.SortOrder,
				ProductServiceID: ln.ProductServiceID,
				Description:      ln.Description,
				Qty:              ln.Qty,
				Unit:             ln.Unit,
				SalesOrderLineID: ln.SalesOrderLineID,
			}
			if err := tx.Create(&sl).Error; err != nil {
				return fmt.Errorf("create shipment line: %w", err)
			}
		}
		created = s
		return tx.Preload("Lines").First(&created, s.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// GetShipment loads a Shipment by (CompanyID, ID) with its lines
// preloaded. The company scope is enforced — a shipment from a
// different company returns ErrShipmentNotFound even if the ID
// matches, preventing cross-tenant leakage.
func GetShipment(db *gorm.DB, companyID, id uint) (*models.Shipment, error) {
	var s models.Shipment
	err := db.Preload("Lines").
		Where("company_id = ? AND id = ?", companyID, id).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrShipmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load shipment: %w", err)
	}
	return &s, nil
}

// UpdateShipment mutates a draft Shipment. Any field left nil on the
// input is preserved. Post-draft shipments are refused with
// ErrShipmentNotDraft — the state machine guards that editing a
// posted or voided document requires its own reversal path.
func UpdateShipment(db *gorm.DB, companyID, id uint, in UpdateShipmentInput) (*models.Shipment, error) {
	var updated models.Shipment
	err := db.Transaction(func(tx *gorm.DB) error {
		s, err := loadShipmentForUpdate(tx, companyID, id)
		if err != nil {
			return err
		}
		if s.Status != models.ShipmentStatusDraft {
			return fmt.Errorf("%w: current=%s", ErrShipmentNotDraft, s.Status)
		}

		if in.ShipmentNumber != nil {
			s.ShipmentNumber = *in.ShipmentNumber
		}
		if in.CustomerID != nil {
			s.CustomerID = in.CustomerID
		}
		if in.WarehouseID != nil {
			if *in.WarehouseID == 0 {
				return ErrShipmentWarehouseRequired
			}
			s.WarehouseID = *in.WarehouseID
		}
		if in.ShipDate != nil {
			if in.ShipDate.IsZero() {
				return ErrShipmentDateRequired
			}
			s.ShipDate = *in.ShipDate
		}
		if in.Memo != nil {
			s.Memo = *in.Memo
		}
		if in.Reference != nil {
			s.Reference = *in.Reference
		}
		if in.SalesOrderID != nil {
			s.SalesOrderID = in.SalesOrderID
		}

		if err := validateShipmentHeaderScope(tx, companyID,
			s.CustomerID, s.WarehouseID, s.SalesOrderID); err != nil {
			return err
		}
		if in.ReplaceLines {
			if err := validateShipmentLinesScope(tx, companyID, in.Lines); err != nil {
				return err
			}
		}

		if err := tx.Save(s).Error; err != nil {
			return fmt.Errorf("save shipment: %w", err)
		}

		if in.ReplaceLines {
			if err := tx.Where("shipment_id = ?", s.ID).
				Delete(&models.ShipmentLine{}).Error; err != nil {
				return fmt.Errorf("delete old lines: %w", err)
			}
			for _, ln := range in.Lines {
				if ln.ProductServiceID == 0 {
					return ErrShipmentLineProductRequired
				}
				sl := models.ShipmentLine{
					CompanyID:        companyID,
					ShipmentID:       s.ID,
					SortOrder:        ln.SortOrder,
					ProductServiceID: ln.ProductServiceID,
					Description:      ln.Description,
					Qty:              ln.Qty,
					Unit:             ln.Unit,
					SalesOrderLineID: ln.SalesOrderLineID,
				}
				if err := tx.Create(&sl).Error; err != nil {
					return fmt.Errorf("create shipment line: %w", err)
				}
			}
		}

		updated = *s
		return tx.Preload("Lines").First(&updated, s.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// ListShipments returns a company's shipments ordered by ShipDate
// descending, then ID descending. Lines are NOT preloaded — the list
// surface is header-level. Callers needing line data call GetShipment
// per row.
func ListShipments(db *gorm.DB, companyID uint, filter ListShipmentsFilter) ([]models.Shipment, error) {
	if companyID == 0 {
		return nil, fmt.Errorf("services.ListShipments: CompanyID required")
	}
	q := db.Model(&models.Shipment{}).
		Where("company_id = ?", companyID)
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.FromDate != nil {
		q = q.Where("ship_date >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		q = q.Where("ship_date <= ?", *filter.ToDate)
	}
	if filter.CustomerID != nil {
		q = q.Where("customer_id = ?", *filter.CustomerID)
	}
	q = q.Order("ship_date DESC, id DESC")
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	var rows []models.Shipment
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list shipments: %w", err)
	}
	return rows, nil
}

// PostShipment flips a draft Shipment to posted.
//
// I.2 scope: pure status flip + audit. No inventory effect, no GL
// effect, no waiting_for_invoice item. The full Shipment-first flow
// (IssueStockFromShipment + Dr COGS / Cr Inventory JE +
// waiting_for_invoice creation) lands in I.3 and will branch on
// companies.shipment_required. Until I.3 ships, PostShipment is
// byte-identical regardless of the shipment_required flag — which is
// exactly the I.2 boundary contract.
//
// Writes exactly one audit row (`shipment.posted`).
func PostShipment(db *gorm.DB, companyID, id uint, actor string, actorUserID *uuid.UUID) (*models.Shipment, error) {
	var out models.Shipment
	err := db.Transaction(func(tx *gorm.DB) error {
		s, err := loadShipmentForUpdate(tx, companyID, id)
		if err != nil {
			return err
		}
		if s.Status != models.ShipmentStatusDraft {
			return fmt.Errorf("%w: current=%s", ErrShipmentAlreadyPosted, s.Status)
		}

		now := time.Now().UTC()
		s.Status = models.ShipmentStatusPosted
		s.PostedAt = &now
		if err := tx.Save(s).Error; err != nil {
			return fmt.Errorf("save shipment: %w", err)
		}
		cid := companyID
		TryWriteAuditLogWithContextDetails(
			tx,
			"shipment.posted",
			"shipment",
			s.ID,
			actorOrSystem(actor),
			map[string]any{"shipment_number": s.ShipmentNumber},
			&cid,
			actorUserID,
			map[string]any{"status": string(models.ShipmentStatusDraft)},
			map[string]any{"status": string(models.ShipmentStatusPosted)},
		)
		out = *s
		return tx.Preload("Lines").First(&out, s.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// VoidShipment flips a posted Shipment to voided.
//
// I.2 scope: pure status flip + audit, regardless of whether the
// shipment was posted under shipment_required=true or false. I.3 will
// bolt on movement-reversal + JE-reversal semantics when JE linkage
// becomes possible; in I.2 there is no JE and no movement to reverse,
// so the void path is trivial.
//
// Writes exactly one audit row (`shipment.voided`).
func VoidShipment(db *gorm.DB, companyID, id uint, actor string, actorUserID *uuid.UUID) (*models.Shipment, error) {
	var out models.Shipment
	err := db.Transaction(func(tx *gorm.DB) error {
		s, err := loadShipmentForUpdate(tx, companyID, id)
		if err != nil {
			return err
		}
		if s.Status != models.ShipmentStatusPosted {
			return fmt.Errorf("%w: current=%s", ErrShipmentNotPosted, s.Status)
		}

		now := time.Now().UTC()
		s.Status = models.ShipmentStatusVoided
		s.VoidedAt = &now
		if err := tx.Save(s).Error; err != nil {
			return fmt.Errorf("save shipment: %w", err)
		}
		cid := companyID
		TryWriteAuditLogWithContextDetails(
			tx,
			"shipment.voided",
			"shipment",
			s.ID,
			actorOrSystem(actor),
			map[string]any{"shipment_number": s.ShipmentNumber},
			&cid,
			actorUserID,
			map[string]any{"status": string(models.ShipmentStatusPosted)},
			map[string]any{"status": string(models.ShipmentStatusVoided)},
		)
		out = *s
		return tx.Preload("Lines").First(&out, s.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteShipment removes a draft shipment and its lines. Non-draft
// shipments (posted, voided) are refused — their trace must stay for
// audit continuity.
func DeleteShipment(db *gorm.DB, companyID, id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		s, err := loadShipmentForUpdate(tx, companyID, id)
		if err != nil {
			return err
		}
		if s.Status != models.ShipmentStatusDraft {
			return fmt.Errorf("%w: current=%s", ErrShipmentNotDraft, s.Status)
		}
		if err := tx.Where("shipment_id = ?", s.ID).
			Delete(&models.ShipmentLine{}).Error; err != nil {
			return fmt.Errorf("delete shipment lines: %w", err)
		}
		if err := tx.Delete(s).Error; err != nil {
			return fmt.Errorf("delete shipment: %w", err)
		}
		return nil
	})
}

// validateShipmentHeaderScope verifies that every non-nil reference
// ID on the Shipment header resolves to a row belonging to the same
// company. No FK constraints enforce this at the DB layer (SO is
// reservation-only in I.2, and multi-company joins are legal at the
// schema level for legacy reasons), so the service is the boundary.
//
// Checks:
//   - Customer (optional): customers.company_id == companyID
//   - Warehouse (required): warehouses.company_id == companyID
//   - SalesOrder (optional): sales_orders.company_id == companyID
//
// Returns ErrShipmentCrossCompanyReference on mismatch, wrapped with
// the offending entity name so logs pinpoint the fault.
func validateShipmentHeaderScope(tx *gorm.DB, companyID uint, customerID *uint, warehouseID uint, salesOrderID *uint) error {
	if customerID != nil && *customerID != 0 {
		if err := requireShipmentSameCompany(tx, &models.Customer{}, "customer",
			*customerID, companyID); err != nil {
			return err
		}
	}
	if warehouseID != 0 {
		if err := requireShipmentSameCompany(tx, &models.Warehouse{}, "warehouse",
			warehouseID, companyID); err != nil {
			return err
		}
	}
	if salesOrderID != nil && *salesOrderID != 0 {
		if err := requireShipmentSameCompany(tx, &models.SalesOrder{}, "sales_order",
			*salesOrderID, companyID); err != nil {
			return err
		}
	}
	return nil
}

// validateShipmentLinesScope applies the same company-scope rule to
// each line's referenced IDs. Runs one query per distinct reference
// — small input sets so N+1 is acceptable; optimisable if lines grow
// into the hundreds.
//
// Checks:
//   - ProductService (required): product_services.company_id == companyID
//   - SalesOrderLine (optional): resolved via parent sales_orders.company_id
//     because SalesOrderLine itself does not carry a company_id column
//     in its schema (mirrors the one-sided-join pattern used elsewhere
//     in the sell-side stack).
func validateShipmentLinesScope(tx *gorm.DB, companyID uint, lines []CreateShipmentLineInput) error {
	for i, ln := range lines {
		if ln.ProductServiceID != 0 {
			if err := requireShipmentSameCompany(tx, &models.ProductService{}, "product_service",
				ln.ProductServiceID, companyID); err != nil {
				return fmt.Errorf("line[%d]: %w", i, err)
			}
		}
		if ln.SalesOrderLineID != nil && *ln.SalesOrderLineID != 0 {
			if err := requireSalesOrderLineCompany(tx, *ln.SalesOrderLineID, companyID); err != nil {
				return fmt.Errorf("line[%d]: %w", i, err)
			}
		}
	}
	return nil
}

// requireShipmentSameCompany loads the row identified by id and
// confirms its CompanyID matches the expected value. Shipment
// analogue of receipt_service.requireSameCompany; kept file-local so
// the error wrapping can use shipment-specific sentinels without
// cross-domain refactor.
func requireShipmentSameCompany(tx *gorm.DB, model any, entity string, id, companyID uint) error {
	var found uint
	err := tx.Model(model).
		Select("company_id").
		Where("id = ?", id).
		Limit(1).
		Scan(&found).Error
	if err != nil {
		return fmt.Errorf("validate %s scope: %w", entity, err)
	}
	if found == 0 {
		return fmt.Errorf("%w: %s id=%d not found", ErrShipmentNotFound, entity, id)
	}
	if found != companyID {
		return fmt.Errorf("%w: %s id=%d belongs to company=%d, shipment company=%d",
			ErrShipmentCrossCompanyReference, entity, id, found, companyID)
	}
	return nil
}

// requireSalesOrderLineCompany verifies a SalesOrderLine's parent SO
// belongs to the expected company. SalesOrderLine has no CompanyID
// column, so the join through sales_orders is the only authoritative
// path. Returns ErrShipmentNotFound if the line is absent or its
// parent missing, ErrShipmentCrossCompanyReference on tenant mismatch.
func requireSalesOrderLineCompany(tx *gorm.DB, lineID, companyID uint) error {
	var found uint
	err := tx.Table("sales_order_lines").
		Select("sales_orders.company_id").
		Joins("JOIN sales_orders ON sales_orders.id = sales_order_lines.sales_order_id").
		Where("sales_order_lines.id = ?", lineID).
		Limit(1).
		Scan(&found).Error
	if err != nil {
		return fmt.Errorf("validate sales_order_line scope: %w", err)
	}
	if found == 0 {
		return fmt.Errorf("%w: sales_order_line id=%d not found", ErrShipmentNotFound, lineID)
	}
	if found != companyID {
		return fmt.Errorf("%w: sales_order_line id=%d belongs to company=%d, shipment company=%d",
			ErrShipmentCrossCompanyReference, lineID, found, companyID)
	}
	return nil
}

// loadShipmentForUpdate fetches a Shipment scoped to the company and
// takes a row-level write lock (`SELECT ... FOR UPDATE` on PostgreSQL;
// no-op on SQLite — test DBs are single-writer anyway). Used by every
// lifecycle-mutating operation (Update, Post, Void, Delete) so
// concurrent flips on the same shipment serialise and cross-state
// races (e.g. two simultaneous PostShipment calls) are rejected
// deterministically by the status check that immediately follows.
//
// In I.2 the mutations are document-layer only, but lifecycle truth
// itself deserves concurrency protection so that I.3 lands on a
// race-free foundation rather than inherit one.
//
// Returns ErrShipmentNotFound when the row does not exist or belongs
// to another tenant (company scope enforced).
func loadShipmentForUpdate(tx *gorm.DB, companyID, id uint) (*models.Shipment, error) {
	var s models.Shipment
	err := applyLockForUpdate(tx.Where("company_id = ? AND id = ?", companyID, id)).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrShipmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load shipment: %w", err)
	}
	return &s, nil
}
