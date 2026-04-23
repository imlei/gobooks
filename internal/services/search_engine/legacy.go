// 遵循project_guide.md
package search_engine

import (
	"context"
	"errors"
)

// LegacyEngine is the placeholder Phase 0 wrapper around the existing
// SmartPicker fan-out. It does NOT call SmartPicker today — Phase 0
// keeps the existing /api/smart-picker/search HTTP handler untouched —
// but exists so the Selector compiles end-to-end and Phase 4 only has
// to switch the wiring rather than introduce new types.
//
// When Phase 4 lands, this engine becomes the bridge that lets the
// new global-search dropdown talk through search_engine.Selector
// while still being backed by the original SmartPicker providers
// during the dual-run validation window.
type LegacyEngine struct{}

// NewLegacyEngine returns the Phase 0 stub.
func NewLegacyEngine() *LegacyEngine { return &LegacyEngine{} }

func (*LegacyEngine) Mode() Mode { return ModeLegacy }

// Search returns an explicit "not implemented" error in Phase 0.
// Nothing in the codebase calls Selector.Search yet — this method only
// exists to satisfy the Engine interface and force Phase 4 to consciously
// wire it up rather than accidentally inherit a stub.
func (*LegacyEngine) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	return nil, errors.New("search_engine: LegacyEngine.Search is a Phase 0 stub; wire SmartPicker dispatch in Phase 4")
}
