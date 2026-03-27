// 遵循产品需求 v1.0
package services

import (
	"fmt"
	"strings"
	"time"

	"gobooks/internal/models"

	"gorm.io/gorm"
)

// ReverseJournalEntry creates a new entry with debit/credit swapped for each line.
// It returns the new reversed journal entry ID.
func ReverseJournalEntry(tx *gorm.DB, originalID uint, reverseDate time.Time) (uint, error) {
	if originalID == 0 {
		return 0, fmt.Errorf("invalid journal entry id")
	}

	var original models.JournalEntry
	if err := tx.Preload("Lines").First(&original, originalID).Error; err != nil {
		return 0, err
	}
	if original.ReversedFromID != nil {
		return 0, fmt.Errorf("cannot reverse a reversal entry")
	}
	if len(original.Lines) < 2 {
		return 0, fmt.Errorf("journal entry must have at least 2 lines")
	}

	var existing models.JournalEntry
	if err := tx.Where("reversed_from_id = ?", originalID).First(&existing).Error; err == nil {
		return 0, fmt.Errorf("journal entry already reversed")
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}

	revDesc := fmt.Sprintf("Reversal of JE #%d", original.ID)
	if s := strings.TrimSpace(original.JournalNo); s != "" {
		revDesc = fmt.Sprintf("%s: %s", revDesc, s)
	}
	reversed := models.JournalEntry{
		EntryDate:      reverseDate,
		JournalNo:      revDesc,
		ReversedFromID: &original.ID,
	}
	if err := tx.Create(&reversed).Error; err != nil {
		return 0, err
	}

	lines := make([]models.JournalLine, 0, len(original.Lines))
	for _, l := range original.Lines {
		lines = append(lines, models.JournalLine{
			JournalEntryID: reversed.ID,
			AccountID:      l.AccountID,
			Debit:          l.Credit,
			Credit:         l.Debit,
			Memo:           l.Memo,
			PartyType:      l.PartyType,
			PartyID:        l.PartyID,
		})
	}

	if err := tx.Create(&lines).Error; err != nil {
		return 0, err
	}

	return reversed.ID, nil
}

