// 遵循project_guide.md
package web

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

func (s *Server) handleAccountingBooksGet(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/setup", fiber.StatusSeeOther)
	}

	books, err := services.ListAccountingBooks(s.DB, companyID)
	if err != nil {
		books = []models.AccountingBook{}
	}

	profiles, _ := services.ListStandardProfiles(s.DB)

	vm := pages.AccountingBooksVM{
		HasCompany: true,
		Books:      books,
		Profiles:   profiles,
		DrawerOpen: c.Query("drawer") == "create",
		Breadcrumb: []pages.SettingsBreadcrumbPart{
			{Label: "Settings"},
			{Label: "Accounting Books"},
		},
	}
	return pages.AccountingBooksHub(vm).Render(c.Context(), c)
}

func (s *Server) handleAccountingBooksCreate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/setup", fiber.StatusSeeOther)
	}

	bookType := models.AccountingBookType(strings.TrimSpace(c.FormValue("book_type")))
	currency := strings.ToUpper(strings.TrimSpace(c.FormValue("currency")))
	profileCode := models.AccountingStandardProfileCode(strings.TrimSpace(c.FormValue("profile_code")))

	_, err := services.CreateAccountingBook(s.DB, services.CreateAccountingBookInput{
		CompanyID:              companyID,
		BookType:               bookType,
		FunctionalCurrencyCode: currency,
		StandardProfileCode:    profileCode,
	})

	books, _ := services.ListAccountingBooks(s.DB, companyID)
	profiles, _ := services.ListStandardProfiles(s.DB)

	vm := pages.AccountingBooksVM{
		HasCompany:       true,
		Books:            books,
		Profiles:         profiles,
		FieldBookType:    string(bookType),
		FieldCurrency:    currency,
		FieldProfileCode: string(profileCode),
		Breadcrumb: []pages.SettingsBreadcrumbPart{
			{Label: "Settings"},
			{Label: "Accounting Books"},
		},
	}

	if err != nil {
		vm.DrawerOpen = true
		if errors.Is(err, services.ErrBookAlreadyExists) {
			vm.FormError = "A book with this type, currency, and standard already exists."
		} else {
			vm.FormError = err.Error()
		}
		return pages.AccountingBooksHub(vm).Render(c.Context(), c)
	}

	return c.Redirect("/settings/accounting-books?saved=1", fiber.StatusSeeOther)
}
