// 遵循project_guide.md
package services

// test_helpers_unique_tag_test.go — shared test helper for
// generating monotonically-unique string tags within a single
// test process.
//
// Why this file exists
// --------------------
// Several gateway / payout / dispute tests generate "unique"
// identifiers for seeded rows via `fmt.Sprintf("%d",
// time.Now().UnixNano())`. On fast hardware two back-to-back calls
// can return the SAME nanosecond value, producing identical tags
// across consecutive seed calls. That collision was the root
// cause of the intermittent `TestGatewayPayout_SettlementAlreadyBridged`
// failure surfaced in the CI cleanup pass — the second `defaultPayoutInput`
// call in that test generated the same ProviderPayoutID as the
// first, masking the expected `ErrPayoutSettlementAlreadyBridged`
// with a duplicate-ID error.
//
// Fix: append a per-process atomic counter to the timestamp. Even
// on infinite-speed hardware, every call gets a guaranteed-
// distinct tag.
//
// Having this in `_test.go` keeps it out of the production binary
// while being available to every test file in the `services`
// package.

import (
	"fmt"
	"sync/atomic"
	"time"
)

// uniqueTestTagCounter is a per-process monotonic counter used to
// disambiguate timestamps generated in the same nanosecond.
var uniqueTestTagCounter int64

// uniqueTestTag returns a string that is guaranteed to differ
// between any two calls within the same process. Callers can use
// it as a stable-but-unique suffix for seeded customer names,
// invoice numbers, provider IDs, etc.
//
// Format: "<unix_nano>_<counter>" — both components monotonic,
// easily pasted into error messages for debugging.
func uniqueTestTag() string {
	return fmt.Sprintf("%d_%d",
		time.Now().UnixNano(),
		atomic.AddInt64(&uniqueTestTagCounter, 1),
	)
}
