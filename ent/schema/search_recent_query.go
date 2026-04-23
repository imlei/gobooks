// 遵循project_guide.md
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// SearchRecentQuery records a user's recent search inputs so the global
// search dropdown can show "Recent searches" when the input is empty
// and the recent-transactions list isn't enough on its own.
//
// Capped at ~50 rows per (user, company) by an asynchronous trim job;
// no DB-level constraint enforces the cap.
type SearchRecentQuery struct {
	ent.Schema
}

func (SearchRecentQuery) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "search_recent_queries"},
	}
}

func (SearchRecentQuery) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).
			Comment("User who issued the query."),

		field.Uint("company_id").
			Comment("Active company at query time."),

		field.Text("query").
			Comment("Raw query string as the user typed it (preserves case for replay)."),

		field.Time("queried_at").
			Default(time.Now).
			Comment("Wall-clock at time of query."),
	}
}

func (SearchRecentQuery) Indexes() []ent.Index {
	return []ent.Index{
		// Default fetch: most recent for this user + company.
		index.Fields("user_id", "company_id", "queried_at"),
	}
}
