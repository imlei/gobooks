// 遵循产品需求 v1.0
package web

import (
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

func journalEntryPageVM(companyID uint, accounts []models.Account, customers []models.Customer, vendors []models.Vendor, formError string, saved bool) pages.JournalEntryVM {
	return pages.JournalEntryVM{
		HasCompany:       true,
		ActiveCompanyID:  companyID,
		Accounts:         accounts,
		AccountsDataJSON: pages.JournalAccountsDataJSON(accounts),
		Customers:        customers,
		Vendors:          vendors,
		FormError:        formError,
		Saved:            saved,
	}
}

func (s *Server) handleJournalEntryForm(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	accounts, err := s.activeAccountsForCompany(companyID)
	if err != nil {
		return pages.JournalEntry(pages.JournalEntryVM{
			HasCompany:       true,
			ActiveCompanyID:  companyID,
			FormError:        "Could not load accounts.",
			AccountsDataJSON: "[]",
		}).Render(c.Context(), c)
	}

	var customers []models.Customer
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&customers).Error
	var vendors []models.Vendor
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&vendors).Error

	return pages.JournalEntry(journalEntryPageVM(companyID, accounts, customers, vendors, "", c.Query("saved") == "1")).Render(c.Context(), c)
}

type postedLine struct {
	AccountID string
	Debit     string
	Credit    string
	Memo      string
	Party     string
}

func (s *Server) handleJournalEntryPost(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	accounts, _ := s.activeAccountsForCompany(companyID)
	var customers []models.Customer
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&customers).Error
	var vendors []models.Vendor
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&vendors).Error

	entryDateRaw := strings.TrimSpace(c.FormValue("entry_date"))
	journalNo := strings.TrimSpace(c.FormValue("journal_no"))

	if entryDateRaw == "" {
		return pages.JournalEntry(journalEntryPageVM(companyID, accounts, customers, vendors, "Date is required.", false)).Render(c.Context(), c)
	}

	entryDate, err := time.Parse("2006-01-02", entryDateRaw)
	if err != nil {
		return pages.JournalEntry(journalEntryPageVM(companyID, accounts, customers, vendors, "Date must be a valid date.", false)).Render(c.Context(), c)
	}

	re := regexp.MustCompile(`^lines\[(\d+)\]\[(account_id|debit|credit|memo|party)\]$`)
	linesMap := map[string]*postedLine{}

	c.Context().PostArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		m := re.FindStringSubmatch(key)
		if len(m) != 3 {
			return
		}

		idx := m[1]
		field := m[2]
		val := strings.TrimSpace(string(v))

		pl := linesMap[idx]
		if pl == nil {
			pl = &postedLine{}
			linesMap[idx] = pl
		}

		switch field {
		case "account_id":
			pl.AccountID = val
		case "debit":
			pl.Debit = val
		case "credit":
			pl.Credit = val
		case "memo":
			pl.Memo = val
		case "party":
			pl.Party = val
		}
	})

	drafts := make([]services.JournalLineDraft, 0, len(linesMap))
	for _, pl := range linesMap {
		drafts = append(drafts, services.JournalLineDraft{
			AccountID: pl.AccountID,
			Debit:     pl.Debit,
			Credit:    pl.Credit,
			Memo:      pl.Memo,
			Party:     pl.Party,
		})
	}

	validLines, err := services.ValidateJournalLines(drafts)
	if err != nil {
		return pages.JournalEntry(journalEntryPageVM(companyID, accounts, customers, vendors, err.Error(), false)).Render(c.Context(), c)
	}

	decimalZero := decimal.NewFromInt(0)

	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	cid := companyID
	uid := user.ID

	var postedJEID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := services.EnsureJournalLineAccountsBelongToCompany(tx, companyID, validLines); err != nil {
			return err
		}

		je := models.JournalEntry{
			CompanyID: companyID,
			EntryDate: entryDate,
			JournalNo: journalNo,
		}
		if err := tx.Create(&je).Error; err != nil {
			return err
		}
		postedJEID = je.ID

		for i := range validLines {
			validLines[i].CompanyID = companyID
			validLines[i].JournalEntryID = je.ID
			if validLines[i].Debit.IsZero() {
				validLines[i].Debit = decimalZero
			}
			if validLines[i].Credit.IsZero() {
				validLines[i].Credit = decimalZero
			}
		}

		return tx.Create(&validLines).Error
	}); err != nil {
		return pages.JournalEntry(journalEntryPageVM(companyID, accounts, customers, vendors, "Could not save journal entry. Please try again.", false)).Render(c.Context(), c)
	}

	services.TryWriteAuditLogWithContext(s.DB, "journal.posted", "journal_entry", postedJEID, actor, map[string]any{
		"journal_no": journalNo,
		"line_count": len(validLines),
		"entry_date": entryDateRaw,
		"company_id": companyID,
	}, &cid, &uid)

	return c.Redirect("/journal-entry?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleJournalEntryList(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	formError := ""
	if c.Query("error") == "already-reversed" {
		formError = "This journal entry is already reversed."
	}

	var entries []models.JournalEntry
	if err := s.DB.Preload("Lines").Where("company_id = ?", companyID).Order("entry_date desc, id desc").Limit(200).Find(&entries).Error; err != nil {
		return pages.JournalEntryList(pages.JournalEntryListVM{
			HasCompany: true,
			Active:     "Journal Entry",
			Items:      []pages.JournalEntryListItem{},
			FormError:  "Could not load journal entries.",
		}).Render(c.Context(), c)
	}

	reversedFromSet := map[uint]bool{}
	for _, e := range entries {
		if e.ReversedFromID != nil {
			reversedFromSet[*e.ReversedFromID] = true
		}
	}

	items := make([]pages.JournalEntryListItem, 0, len(entries))
	for _, e := range entries {
		totalDebit := decimal.Zero
		totalCredit := decimal.Zero
		for _, l := range e.Lines {
			totalDebit = totalDebit.Add(l.Debit)
			totalCredit = totalCredit.Add(l.Credit)
		}
		canReverse := e.ReversedFromID == nil && !reversedFromSet[e.ID]
		reverseHint := ""
		if e.ReversedFromID != nil {
			reverseHint = "This is already a reversal entry."
		} else if reversedFromSet[e.ID] {
			reverseHint = "Already reversed."
		}
		items = append(items, pages.JournalEntryListItem{
			ID:          e.ID,
			EntryDate:   e.EntryDate.Format("2006-01-02"),
			JournalNo:   e.JournalNo,
			LineCount:   len(e.Lines),
			TotalDebit:  pages.Money(totalDebit),
			TotalCredit: pages.Money(totalCredit),
			CanReverse:  canReverse,
			ReverseHint: reverseHint,
		})
	}

	return pages.JournalEntryList(pages.JournalEntryListVM{
		HasCompany: true,
		Active:     "Journal Entry",
		Items:      items,
		FormError:  formError,
		Reversed:   c.Query("reversed") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleJournalEntryReverse(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	idRaw := strings.TrimSpace(c.Params("id"))
	idU64, err := services.ParseUint(idRaw)
	if err != nil || idU64 == 0 {
		return c.Redirect("/journal-entry/list", fiber.StatusSeeOther)
	}

	reverseDate := time.Now()
	reverseDateRaw := strings.TrimSpace(c.FormValue("reverse_date"))
	if reverseDateRaw != "" {
		if d, err := time.Parse("2006-01-02", reverseDateRaw); err == nil {
			reverseDate = d
		}
	}

	var reversedID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		newID, err := services.ReverseJournalEntry(tx, companyID, uint(idU64), reverseDate)
		if err != nil {
			return err
		}
		reversedID = newID
		return nil
	}); err != nil {
		return c.Redirect("/journal-entry/list?error=already-reversed", fiber.StatusSeeOther)
	}

	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	cid := companyID
	uid := user.ID
	services.TryWriteAuditLogWithContext(s.DB, "journal.reversed", "journal_entry", reversedID, actor, map[string]any{
		"original_id":  idU64,
		"reverse_date": reverseDate.Format("2006-01-02"),
		"company_id":   companyID,
	}, &cid, &uid)

	return c.Redirect("/journal-entry/list?reversed=1", fiber.StatusSeeOther)
}
