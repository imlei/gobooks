// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ── Suggestion type constants ─────────────────────────────────────────────────

const (
	// SuggTypeOneToOne: one book line corresponds to one bank transaction.
	SuggTypeOneToOne = "one_to_one"
	// SuggTypeOneToMany: one bank transaction is explained by multiple book lines.
	SuggTypeOneToMany = "one_to_many"
	// SuggTypeManyToOne: multiple bank transactions are summarised in one book entry.
	SuggTypeManyToOne = "many_to_one"
	// SuggTypeSplit: a single book line partially matches a larger bank amount.
	SuggTypeSplit = "split"
)

// ── Suggestion status lifecycle ───────────────────────────────────────────────
//
//	pending   → the engine produced this; user has not yet reviewed it
//	accepted  → user confirmed the suggestion; lines are pre-selected for reconciliation
//	rejected  → user dismissed the suggestion; accounting data unchanged
//	expired   → a new auto-match run displaced this suggestion (was pending)
//	archived  → the reconciliation this suggestion was linked to has been voided
//
// Suggestions are never deleted; they are retained for full audit history.

const (
	SuggStatusPending  = "pending"
	SuggStatusAccepted = "accepted"
	SuggStatusRejected = "rejected"
	// SuggStatusExpired: displaced by a newer auto-match run without user action.
	SuggStatusExpired = "expired"
	// SuggStatusArchived: the completed reconciliation it was linked to was voided.
	SuggStatusArchived = "archived"
)

// ── Suggestion line role constants ────────────────────────────────────────────

const (
	// SuggLineRoleMatch: primary match — this line's full amount contributes.
	SuggLineRoleMatch = "match"
	// SuggLineRoleSplit: partial contribution in a split suggestion.
	SuggLineRoleSplit = "split"
	// SuggLineRoleContext: informational context line, not part of the match amount.
	SuggLineRoleContext = "context"
)

// ── ReconciliationMatchSuggestion ─────────────────────────────────────────────

// ReconciliationMatchSuggestion holds one engine-generated candidate match.
//
// The engine proposes; the user confirms. Accepting a suggestion pre-selects
// the referenced journal lines in the reconciliation UI — it does NOT post any
// accounting entry or complete the reconciliation. The user still controls
// "Finish Now" to commit the reconciliation.
//
// Lifecycle:
//
//	AutoMatch run        → status = pending
//	User accepts         → status = accepted;  AcceptedByUserID/AcceptedAt set
//	User rejects         → status = rejected;  RejectedByUserID/RejectedAt set
//	New AutoMatch run    → pending rows → status = expired   (old run displaced)
//	Finish Now completes → ReconciliationID set on accepted rows
//	Reconciliation void  → accepted rows for that reconciliation → status = archived
//
// Rows are NEVER deleted — retained permanently for audit.
type ReconciliationMatchSuggestion struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	AccountID uint `gorm:"not null;index"`

	// ReconciliationID is set after the user completes reconciliation ("Finish Now").
	// Links this suggestion to the committed Reconciliation record for traceability.
	// NULL until the reconciliation is completed.
	ReconciliationID *uint `gorm:"index"`

	// SuggestionType classifies the match cardinality.
	SuggestionType string `gorm:"type:text;not null;default:'one_to_one'"`
	// Status: see SuggStatus* constants above.
	Status string `gorm:"type:text;not null;default:'pending'"`

	// ConfidenceScore is the engine's self-assessed confidence in [0, 1].
	ConfidenceScore decimal.Decimal `gorm:"type:numeric(5,4);not null;default:0"`
	// RankingScore drives ordering; higher = shown first in UI.
	RankingScore decimal.Decimal `gorm:"type:numeric(10,4);not null;default:0"`

	// ExplanationJSON stores a services.MatchExplanation as JSON text.
	// Structured so the UI can render named signals with scores without parsing
	// opaque ML outputs.
	ExplanationJSON string `gorm:"type:text;not null;default:'{}'"`

	// GeneratedBy is the engine version tag. Reserved for future A/B comparison
	// and model versioning (e.g. "engine_v1", "engine_v2_llm").
	GeneratedBy string `gorm:"type:text;not null;default:'engine_v1'"`
	GeneratedAt time.Time

	// Accept audit trail.
	AcceptedByUserID *uuid.UUID `gorm:"type:uuid"`
	AcceptedAt       *time.Time

	// Reject audit trail.
	RejectedByUserID *uuid.UUID `gorm:"type:uuid"`
	RejectedAt       *time.Time

	// ReviewedAt / ReviewedByUserID: legacy combined-action fields retained for
	// backward compatibility. Both are set on accept AND reject alongside the
	// specific fields above.
	ReviewedAt       *time.Time
	ReviewedByUserID *uuid.UUID `gorm:"type:uuid"`

	CreatedAt time.Time
	UpdatedAt time.Time // auto-managed by GORM

	Lines []ReconciliationMatchSuggestionLine `gorm:"foreignKey:SuggestionID"`
}

// ── ReconciliationMatchSuggestionLine ────────────────────────────────────────

// ReconciliationMatchSuggestionLine links a suggestion to one book-side journal
// line (the "book candidate"). A suggestion may reference 1..N lines for
// one_to_many, split, etc. cardinalities.
type ReconciliationMatchSuggestionLine struct {
	ID           uint `gorm:"primaryKey"`
	SuggestionID uint `gorm:"not null;index"`
	// CompanyID mirrors the suggestion's company_id for fast multi-company queries.
	CompanyID uint `gorm:"not null"`
	// JournalLineID references journal_lines.id (the "book candidate").
	JournalLineID uint `gorm:"not null"`

	// AmountApplied is the portion of the line's amount that the suggestion applies.
	// For full one-to-one matches this equals the line's total debit-credit amount.
	// For split suggestions this may be a partial amount.
	// NULL means "apply the full line amount" (safe default for one-to-one).
	AmountApplied *decimal.Decimal `gorm:"type:numeric(18,2)"`

	// Role describes this line's function in the suggestion.
	// See SuggLineRole* constants: "match" | "split" | "context".
	Role string `gorm:"type:text;not null;default:'match'"`

	CreatedAt time.Time
}

// ── ReconciliationMemory ──────────────────────────────────────────────────────

// ReconciliationMemory is the lightweight, explainable learning layer.
//
// Each time a user accepts a suggestion, matched_count for the relevant
// (normalized_book_memo, source_type) patterns is incremented.
// Future auto-match runs award a confidence_boost to lines whose patterns
// appear here — the boost is bounded, auditable, and never silently grows.
//
// Unique constraint: (company_id, account_id, normalized_book_memo, source_type).
// One memory cell per memo+source pattern per account.
type ReconciliationMemory struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	AccountID uint `gorm:"not null;index"`

	// NormalizedBookMemo is the cleaned, lowercase memo after noise removal.
	NormalizedBookMemo string `gorm:"type:text;not null;default:''"`
	// SourceType mirrors journal_entries.source_type for the matched line.
	SourceType string `gorm:"type:text;not null;default:''"`

	// Optional party references — enable richer payee-based matching in future.
	VendorID   *uint
	CustomerID *uint

	// MatchedCount increments on each accept; drives confidence_boost growth.
	MatchedCount int `gorm:"not null;default:1"`
	// LastMatchedAt tracks recency for potential time-decay in future versions.
	LastMatchedAt time.Time

	// ConfidenceBoost is added to the historical_match signal score.
	// Grows at 0.05 per accepted match, hard-capped at 0.30 (6 acceptances).
	// Kept bounded and auditable by design.
	ConfidenceBoost decimal.Decimal `gorm:"type:numeric(5,4);not null;default:0.0500"`

	CreatedAt time.Time
	UpdatedAt time.Time // auto-managed by GORM
}
