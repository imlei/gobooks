// 遵循project_guide.md
package models

// Recommendation source tracking (field_recommendation_sources) is for product analytics only.
// The server never treats suggested values as valid because they were recommended: account create
// and edit still run full backend validation on submitted name, code, classification, and GIFI.
// Stored source metadata reflects what the client reports at save time; it is not used for
// authorization or accounting correctness. It is not audit-grade truth—client spoofing is
// acceptable at this lightweight phase. Stronger assurance would require server-side correlation
// with a recent recommendation response or a signed recommendation token (out of scope here).

import (
	"encoding/json"
	"strings"
)

// Field recommendation source at save time (assistive apply vs manual).
const (
	FieldRecoSourceManual = "manual"
	FieldRecoSourceRule   = "rule"
	FieldRecoSourceAI     = "ai"
)

// FieldRecoStatusConfirmed indicates values persisted after a successful save.
// "Suggested" / "applied" are UI-only before submit; DB stores confirmed snapshots.
const FieldRecoStatusConfirmed = "confirmed"

// FieldRecoEntry records how one field was chosen when the account was saved.
type FieldRecoEntry struct {
	Source string `json:"source"` // manual | rule | ai
	Status string `json:"status"` // confirmed
}

// AccountFieldRecommendationSources is stored as JSON in accounts.field_recommendation_sources.
type AccountFieldRecommendationSources struct {
	AccountName FieldRecoEntry `json:"account_name"`
	AccountCode FieldRecoEntry `json:"account_code"`
	GifiCode    FieldRecoEntry `json:"gifi_code"`
}

// ParseFieldRecoSource maps form input to a known source; unknown/empty → manual.
func ParseFieldRecoSource(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case FieldRecoSourceRule, FieldRecoSourceAI:
		return s
	default:
		return FieldRecoSourceManual
	}
}

// BuildAccountFieldRecommendationSourcesJSON builds the persisted JSON for the three tracked fields.
// Always returns a compact JSON object; error only from json.Marshal (should not happen).
func BuildAccountFieldRecommendationSourcesJSON(nameSrc, codeSrc, gifiSrc string) (string, error) {
	o := AccountFieldRecommendationSources{
		AccountName: FieldRecoEntry{Source: ParseFieldRecoSource(nameSrc), Status: FieldRecoStatusConfirmed},
		AccountCode: FieldRecoEntry{Source: ParseFieldRecoSource(codeSrc), Status: FieldRecoStatusConfirmed},
		GifiCode:    FieldRecoEntry{Source: ParseFieldRecoSource(gifiSrc), Status: FieldRecoStatusConfirmed},
	}
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
