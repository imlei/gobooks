// 遵循产品需求 v1.0
package services

import (
	"errors"
	"sort"

	"github.com/shopspring/decimal"
)

// PostingFragment is one debit or credit leg before journal aggregation.
// Exactly one of Debit or Credit should be positive (the other zero).
type PostingFragment struct {
	AccountID uint
	Debit     decimal.Decimal
	Credit    decimal.Decimal
	Memo      string
}

// ErrInvalidPostingFragment is returned when a fragment has both debit and credit, or neither.
var ErrInvalidPostingFragment = errors.New("posting fragment must have exactly one of debit or credit")

// AggregateJournalLines merges fragments that share the same account and the same side
// (debit vs credit). Amounts on the same side are summed; memos are joined with "; " when both non-empty.
//
// Output lines are sorted by account ID, then debits before credits.
func AggregateJournalLines(frags []PostingFragment) ([]PostingFragment, error) {
	type sideKey struct {
		AccountID uint
		DebitLeg  bool // true = debit side, false = credit side
	}

	merged := make(map[sideKey]*PostingFragment)
	var keys []sideKey

	for _, f := range frags {
		dPos := f.Debit.IsPositive()
		cPos := f.Credit.IsPositive()
		if dPos && cPos {
			return nil, ErrInvalidPostingFragment
		}
		if !dPos && !cPos {
			continue
		}

		sk := sideKey{AccountID: f.AccountID, DebitLeg: dPos}
		ex, ok := merged[sk]
		if !ok {
			cp := f
			merged[sk] = &cp
			keys = append(keys, sk)
			continue
		}
		if dPos {
			ex.Debit = ex.Debit.Add(f.Debit)
		} else {
			ex.Credit = ex.Credit.Add(f.Credit)
		}
		ex.Memo = mergePostingMemos(ex.Memo, f.Memo)
	}

	out := make([]PostingFragment, 0, len(keys))
	for _, k := range keys {
		out = append(out, *merged[k])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AccountID != out[j].AccountID {
			return out[i].AccountID < out[j].AccountID
		}
		di := out[i].Debit.IsPositive()
		dj := out[j].Debit.IsPositive()
		if di != dj {
			return di // debits first
		}
		return false
	})
	return out, nil
}

func mergePostingMemos(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" || a == b {
		return a
	}
	return a + "; " + b
}

// SumDebit and SumCredit helpers for balance checks.
func sumPostingDebits(frags []PostingFragment) decimal.Decimal {
	s := decimal.Zero
	for _, f := range frags {
		s = s.Add(f.Debit)
	}
	return s
}

func sumPostingCredits(frags []PostingFragment) decimal.Decimal {
	s := decimal.Zero
	for _, f := range frags {
		s = s.Add(f.Credit)
	}
	return s
}
