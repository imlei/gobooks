// 遵循project_guide.md
//
// ent code generation entry point. Run `go generate ./ent/...` to refresh.
//
// Why ent at all (Phase 0 of the search-engine projection plan):
//   - GORM stays the canonical ORM for business writes.
//   - ent owns ONLY the search subdomain: search_documents,
//     search_recent_queries, search_usage_stats.
//   - Tables are created via the standard migrations/NNN_*.sql pipeline.
//     We do NOT call client.Schema.Create() at runtime — Atlas / ent's
//     auto-migration is intentionally bypassed.
//   - The generated code in this directory provides type-safe queries
//     for the projector + engine; that's the only reason ent is here.
package ent

// --feature sql/upsert enables the generated OnConflict()/UpdateNewValues()
// builders used by the projection code path (ENT `INSERT ... ON CONFLICT DO
// UPDATE`). Required because the projector upserts on
// (company_id, entity_type, entity_id), not on primary key.
//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/upsert ./schema
