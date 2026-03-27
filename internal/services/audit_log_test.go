package services

import (
	"encoding/json"
	"testing"
)

func TestMergeDetailsWithBeforeAfter_mapAndNil(t *testing.T) {
	out := mergeDetailsWithBeforeAfter(map[string]any{"a": 1}, "old", "new")
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["a"].(float64) != 1 {
		t.Fatalf("want a=1, got %v", m["a"])
	}
	if m["before"] != "old" || m["after"] != "new" {
		t.Fatalf("before/after: %#v", m)
	}
}

func TestMergeDetailsWithBeforeAfter_noDelta(t *testing.T) {
	out := mergeDetailsWithBeforeAfter(map[string]any{"x": true}, nil, nil)
	if om, ok := out.(map[string]any); !ok || !om["x"].(bool) {
		t.Fatalf("expected passthrough map, got %#v", out)
	}
}
