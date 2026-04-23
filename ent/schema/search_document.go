// 遵循project_guide.md
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SearchDocument is the denormalized search projection. One row per
// (company_id, entity_type, entity_id) tuple — eventually consistent
// with the canonical business tables (invoices, customers, vendors,
// product_services, etc.). Maintained by internal/searchprojection.
//
// Phase 0 owns only the schema. Phase 1+ fills the projector and
// switches the SmartPicker engine to read from this table.
//
// Schema invariants:
//   - company_id is REQUIRED on every row; SmartPicker queries always
//     include it. There is no global / cross-company search.
//   - (company_id, entity_type, entity_id) is UNIQUE — projection is
//     idempotent.
//   - title_native / title_latin / title_initials are produced by the
//     searchprojection.Normalizer interface. Phase 0 default is the
//     ASCII-only impl; future multi-language phases swap implementations
//     and run a reconciler to refill these columns.
//
// Note: ent's Atlas migration is NOT used. Tables are created via the
// standard migrations/NNN_*.sql pipeline. ent only provides the
// type-safe CRUD layer.
type SearchDocument struct {
	ent.Schema
}

func (SearchDocument) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "search_documents"},
	}
}

func (SearchDocument) Fields() []ent.Field {
	return []ent.Field{
		field.Uint("company_id").
			Comment("Tenant scope. Every query filters on this column."),

		field.String("entity_type").
			MaxLen(32).
			Comment("Kind of business object: invoice | bill | quote | sales_order | purchase_order | customer | vendor | product_service | page | etc."),

		field.Uint("entity_id").
			Comment("Foreign reference into the canonical business table. No FK constraint — projection is eventually consistent and may briefly point at a deleted row."),

		field.String("doc_number").
			MaxLen(64).
			Optional().
			Default("").
			Comment("Document number / code: invoice number, customer code, SKU, etc. Used for first-tier exact-match ranking."),

		field.Text("title").
			Comment("Display title. Customer name for contacts; invoice counterparty + number for transactions; product name for catalog."),

		field.Text("subtitle").
			Optional().
			Default("").
			Comment("Secondary line shown under the title in dropdown results. Format: pipe-separated metadata e.g. 'INV-202604 · POSX US INC. · 22/4/26 · $3,600.00'."),

		// Normalized search fields — produced by Normalizer impl.
		field.Text("title_native").
			Comment("Lowercased + punctuation-stripped title. Works for any codepoint. Indexed with pg_trgm for substring/fuzzy match."),

		field.Text("title_latin").
			Optional().
			Default("").
			Comment("Latin-alphabet transliteration. ASCII strings = same as title_native. CJK strings = pinyin (multi-language phase). Indexed with pg_trgm."),

		field.Text("title_initials").
			Optional().
			Default("").
			Comment("First letter(s) of each word/syllable in Latin form. e.g. 'Acme Corp' → 'AC', '李华' → 'LH' (multi-language phase)."),

		field.Text("memo_native").
			Optional().
			Default("").
			Comment("Lowercased memo / description. Lower-priority match field."),

		// Display / ranking signals.
		field.Time("doc_date").
			Optional().
			Nillable().
			Comment("Business date of the document (invoice_date / customer.created_at / etc.). Drives recency ordering."),

		field.String("amount").
			MaxLen(32).
			Optional().
			Default("").
			Comment("Pre-formatted amount string for display in dropdown. Empty for non-money entities (customers, products)."),

		field.String("currency").
			MaxLen(3).
			Optional().
			Default("").
			Comment("ISO currency code; empty when not applicable."),

		field.String("status").
			MaxLen(32).
			Optional().
			Default("").
			Comment("Native status string of the underlying document — used for badge colour + filtering (e.g. 'paid', 'overdue', 'voided')."),

		// Routing.
		field.String("url_path").
			MaxLen(255).
			Comment("Detail-page URL. Used by global-search dropdown to navigate on Enter."),

		// Meta / housekeeping.
		field.Int("projector_version").
			Default(0).
			Comment("Schema version of the projector that wrote this row. Reconciler re-projects rows where this is below the current code constant."),

		field.Time("created_at").
			Default(time.Now).
			Immutable(),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (SearchDocument) Indexes() []ent.Index {
	return []ent.Index{
		// Idempotent upsert key.
		index.Fields("company_id", "entity_type", "entity_id").Unique(),
		// Recency-ordered list (Phase 4 advanced search default ordering).
		index.Fields("company_id", "doc_date"),
		// Exact-code lookup (first-tier match: "INV-202604" → 1 row).
		index.Fields("company_id", "entity_type", "doc_number"),
		// Status filter (e.g. exclude voided in default view).
		index.Fields("company_id", "entity_type", "status"),
	}
	// pg_trgm GIN indexes on title_native / title_latin / title_initials are
	// declared in the SQL migration directly. ent's index DSL doesn't model
	// gin_trgm_ops; the migration owns those.
}
