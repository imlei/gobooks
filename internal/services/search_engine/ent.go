// 遵循project_guide.md
package search_engine

import (
	"context"
	"errors"
)

// EntEngine reads from the search_documents projection (populated by
// internal/searchprojection.Projector) using the ent-generated client.
//
// Phase 0 ships only the type — the actual query implementation lands
// in Phase 4 when the projector has had enough time in dual mode to
// guarantee data parity with the legacy fan-out.
type EntEngine struct{}

// NewEntEngine returns the Phase 0 stub.
func NewEntEngine() *EntEngine { return &EntEngine{} }

func (*EntEngine) Mode() Mode { return ModeEnt }

func (*EntEngine) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	return nil, errors.New("search_engine: EntEngine.Search not yet implemented (Phase 4)")
}
