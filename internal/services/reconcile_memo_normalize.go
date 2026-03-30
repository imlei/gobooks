// 遵循project_guide.md
package services

import (
	"regexp"
	"strings"
)

// reBankNoise strips common bank-statement noise words that add no signal value
// for memo similarity matching.
var reBankNoise = regexp.MustCompile(
	`(?i)\b(pos|ach|ref|debit|credit|wire|xfer|transfer|deposit|withdrawal|fee|charge|payment|online|mobile|check|chk|trn|trans|purchase|sale)\b`,
)

// reLongNumbers removes standalone numeric sequences of 4+ digits (dates, ref IDs,
// account suffixes) that vary per transaction and hurt similarity scores.
var reLongNumbers = regexp.MustCompile(`\b\d{4,}\b`)

// rePunct replaces any character that is not alphanumeric or space with a space.
var rePunct = regexp.MustCompile(`[^a-z0-9 ]+`)

// reSpaces collapses multiple consecutive spaces into one.
var reSpaces = regexp.MustCompile(`\s{2,}`)

// NormalizeMemo returns a cleaned, lowercase version of a transaction memo suitable
// for storage in reconciliation_memory and fuzzy similarity comparison.
//
// Pipeline:
//  1. Lowercase
//  2. Remove bank noise words
//  3. Remove long numeric tokens (ref numbers, dates)
//  4. Remove punctuation
//  5. Collapse whitespace
func NormalizeMemo(s string) string {
	s = strings.ToLower(s)
	s = reBankNoise.ReplaceAllString(s, " ")
	s = reLongNumbers.ReplaceAllString(s, " ")
	s = rePunct.ReplaceAllString(s, " ")
	s = strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
	return s
}

// MemoSimilarity returns a Jaccard similarity coefficient [0, 1] between two
// already-normalised memo strings, computed over word tokens of length ≥ 3.
// Returns 0 if both strings are empty or share no tokens.
func MemoSimilarity(normA, normB string) float64 {
	if normA == "" || normB == "" {
		return 0
	}
	setA := tokenSet(normA)
	setB := tokenSet(normB)
	return jaccardSets(setA, setB)
}

// tokenSet returns a deduplicated set of word tokens of length ≥ 3.
func tokenSet(s string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, t := range strings.Fields(s) {
		if len(t) >= 3 {
			tokens[t] = struct{}{}
		}
	}
	return tokens
}

// jaccardSets computes |A ∩ B| / |A ∪ B|.
func jaccardSets(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
