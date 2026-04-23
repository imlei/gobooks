// 遵循project_guide.md
package searchprojection

import (
	"strings"
	"unicode"
)

// Normalizer produces the searchable variants of a source string. Phase 0
// ships only the ASCII implementation (no transliteration); future
// multi-language phases plug in CJK / Hangul / Kana romanisation by
// substituting an alternate Normalizer at the call site — schema, projector,
// and engine query code do not change.
//
// The split into three methods exists so storage cost is paid once and
// queries can target the most appropriate column:
//
//   - Native    → trigram match across any codepoint (works for both
//                 Latin "Acme" and CJK "李华" verbatim)
//   - Latin     → trigram match against romanised form (so a user typing
//                 "lihua" finds "李华")
//   - Initials  → power-user shortcut ("ac" → "Acme Corp", "lh" → "李华")
//
// Implementations MUST be deterministic and stateless — they are called
// during projection upserts in any goroutine.
type Normalizer interface {
	// Native returns lowercased + punctuation-stripped form preserving
	// the original script. ASCII falls through unchanged (just lowered).
	Native(s string) string

	// Latin returns a Latin-alphabet transliteration. For pure-ASCII
	// input this equals Native. CJK implementations return pinyin
	// (without tone marks); Hangul implementations return romaja; etc.
	Latin(s string) string

	// Initials returns the first letter of each word/syllable, in Latin
	// form. "Acme Corp" → "ac"; "李华 Trading" → "lh t" (CJK impl);
	// in the ASCII default the CJK case yields just "t".
	Initials(s string) string
}

// AsciiNormalizer is the Phase 0 default. Pure ASCII handling — never
// transliterates; non-ASCII passes through Native unchanged and is
// stripped from Latin / Initials. Always-safe fallback.
//
// When the multi-language slice lands, swap the Normalizer instance
// passed to the projector and run a reconciler to rewrite existing
// rows' title_latin / title_initials columns. Schema, projector, and
// engine code do not change.
type AsciiNormalizer struct{}

// Native lowercases s and strips punctuation, preserving every codepoint
// regardless of script. CJK / accented Latin / etc. pass through (just
// lowered) so trigram matches on title_native still work for inputs in
// the original language.
func (AsciiNormalizer) Native(s string) string {
	return lowerStripPunct(s)
}

// Latin in the ASCII implementation is just the Native form with
// non-ASCII codepoints removed — gives an empty string for pure-CJK
// inputs (acceptable; the Native column carries those). Multi-language
// implementations override to return real transliterations.
func (AsciiNormalizer) Latin(s string) string {
	return asciiOnly(lowerStripPunct(s))
}

// Initials returns the first ASCII letter of each word in s. Words are
// separated by whitespace OR punctuation (so "Acme,Corp" → "ac"); we
// route through lowerStripPunct first to get that consistent splitting
// behaviour. Non-ASCII words contribute nothing in the ASCII impl; the
// CJK impl will romanise first, then take per-syllable initials.
func (AsciiNormalizer) Initials(s string) string {
	var out strings.Builder
	for _, word := range strings.Fields(lowerStripPunct(s)) {
		for _, r := range word {
			if r >= 'a' && r <= 'z' {
				out.WriteRune(r)
				break
			}
		}
	}
	return out.String()
}

// lowerStripPunct lowercases s and removes Unicode punctuation /
// symbols / control chars, while keeping letters, digits, and a single
// space as a word separator. Whitespace runs collapse to one space.
func lowerStripPunct(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true // suppress leading whitespace
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			// Punctuation / symbols / control — drop entirely. Treat as
			// a word boundary so "Acme,Corp" → "acme corp" not "acmecorp".
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}
	out := b.String()
	return strings.TrimRight(out, " ")
}

// asciiOnly removes non-ASCII codepoints from s. Used in the ASCII
// Normalizer's Latin() to discard CJK / accented characters.
func asciiOnly(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
