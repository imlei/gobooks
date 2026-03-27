package models

import (
	"encoding/json"
	"testing"
)

func TestParseFieldRecoSource(t *testing.T) {
	t.Parallel()
	if ParseFieldRecoSource("") != FieldRecoSourceManual {
		t.Fatal("empty")
	}
	if ParseFieldRecoSource("RULE") != FieldRecoSourceRule {
		t.Fatal("rule")
	}
	if ParseFieldRecoSource("ai") != FieldRecoSourceAI {
		t.Fatal("ai")
	}
	if ParseFieldRecoSource("bogus") != FieldRecoSourceManual {
		t.Fatal("bogus → manual")
	}
}

func TestBuildAccountFieldRecommendationSourcesJSON(t *testing.T) {
	t.Parallel()
	s, err := BuildAccountFieldRecommendationSourcesJSON("rule", "ai", "")
	if err != nil {
		t.Fatal(err)
	}
	var got AccountFieldRecommendationSources
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountName.Source != FieldRecoSourceRule || got.AccountName.Status != FieldRecoStatusConfirmed {
		t.Fatalf("name: %+v", got.AccountName)
	}
	if got.AccountCode.Source != FieldRecoSourceAI {
		t.Fatalf("code: %+v", got.AccountCode)
	}
	if got.GifiCode.Source != FieldRecoSourceManual {
		t.Fatalf("gifi: %+v", got.GifiCode)
	}
}
