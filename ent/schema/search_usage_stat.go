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

// SearchUsageStat is a per-(company, entity_type, entity_id) click counter
// used to bias the SmartPicker ranker toward items the company actually
// uses. Increments fire from the dropdown's select handler — no per-user
// breakdown to keep storage bounded and avoid unnecessary PII trails.
//
// Phase 0 owns only the schema; the bumper + ranker integration land in
// the relevance-tuning slice (post Phase 4).
type SearchUsageStat struct {
	ent.Schema
}

func (SearchUsageStat) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "search_usage_stats"},
	}
}

func (SearchUsageStat) Fields() []ent.Field {
	return []ent.Field{
		field.Uint("company_id"),

		field.String("entity_type").
			MaxLen(32),

		field.Uint("entity_id"),

		field.Int("click_count").
			Default(0).
			Comment("Total click-through count from the global search dropdown."),

		field.Time("last_clicked_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("Wall-clock of the most recent click — recency tie-breaker for the ranker."),
	}
}

func (SearchUsageStat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "entity_type", "entity_id").Unique(),
		// Ranker join key — pulled per-candidate to weight the result list.
		index.Fields("company_id", "entity_type"),
	}
}
