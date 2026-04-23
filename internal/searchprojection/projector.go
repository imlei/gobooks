// 遵循project_guide.md
package searchprojection

import "context"

// NoopProjector is the test / fallback Projector: it validates the input
// shape (same rules as the real projector) and then does nothing. Useful
// when:
//   - ent wiring is not available (e.g. during backfill dry-runs or CLI
//     tools that don't need to write);
//   - unit tests exercise producer code and want to assert "would have
//     projected this document" without involving ent.
type NoopProjector struct{}

func (NoopProjector) Upsert(ctx context.Context, doc Document) error {
	return validateDocument(doc)
}

func (NoopProjector) Delete(ctx context.Context, companyID uint, entityType string, entityID uint) error {
	return validateDeleteArgs(companyID, entityType, entityID)
}

// applyNormalizer materialises the three normalised forms of a document
// into something the persistence layer can store. Pure function — kept
// here so future EntProjector and the test harness share the same
// normalisation pipeline.
type normalised struct {
	TitleNative   string
	TitleLatin    string
	TitleInitials string
	MemoNative    string
}

func applyNormalizer(n Normalizer, doc Document) normalised {
	if n == nil {
		n = AsciiNormalizer{}
	}
	return normalised{
		TitleNative:   n.Native(doc.Title),
		TitleLatin:    n.Latin(doc.Title),
		TitleInitials: n.Initials(doc.Title),
		MemoNative:    n.Native(doc.Memo),
	}
}
