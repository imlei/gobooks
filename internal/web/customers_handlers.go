// 遵循project_guide.md
package web

import (
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

var rePostalCode = regexp.MustCompile(`^[A-Za-z0-9 \-]*$`)

func (s *Server) handleCustomers(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	var customers []models.Customer
	if err := s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&customers).Error; err != nil {
		return pages.Customers(pages.CustomersVM{
			HasCompany: true,
			FormError:  "Could not load customers.",
			Customers:  []models.Customer{},
		}).Render(c.Context(), c)
	}

	return pages.Customers(pages.CustomersVM{
		HasCompany: true,
		Customers:  customers,
		Created:    c.Query("created") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleCustomerNew(c *fiber.Ctx) error {
	_, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	return pages.CustomerNew(pages.CustomerNewVM{HasCompany: true}).Render(c.Context(), c)
}

func (s *Server) handleCustomerCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	name           := strings.TrimSpace(c.FormValue("name"))
	email          := strings.TrimSpace(c.FormValue("email"))
	paymentTerm    := strings.TrimSpace(c.FormValue("payment_term"))
	addrStreet1    := strings.TrimSpace(c.FormValue("addr_street1"))
	addrStreet2    := strings.TrimSpace(c.FormValue("addr_street2"))
	addrCity       := strings.TrimSpace(c.FormValue("addr_city"))
	addrProvince   := strings.TrimSpace(c.FormValue("addr_province"))
	addrPostalCode := strings.TrimSpace(c.FormValue("addr_postal_code"))
	addrCountry    := strings.TrimSpace(c.FormValue("addr_country"))

	vm := pages.CustomerNewVM{
		HasCompany:     true,
		Name:           name,
		Email:          email,
		PaymentTerm:    paymentTerm,
		AddrStreet1:    addrStreet1,
		AddrStreet2:    addrStreet2,
		AddrCity:       addrCity,
		AddrProvince:   addrProvince,
		AddrPostalCode: addrPostalCode,
		AddrCountry:    addrCountry,
	}

	// ── Validation ────────────────────────────────────────────────────────────
	if name == "" {
		vm.NameError = "Name is required."
	} else if len(name) > 200 {
		vm.NameError = "Name must be 200 characters or fewer."
	}
	if len(email) > 200 {
		vm.FormError = "Email must be 200 characters or fewer."
	}
	if len(paymentTerm) > 100 {
		vm.FormError = "Payment term must be 100 characters or fewer."
	}
	if len(addrStreet1) > 200 || len(addrStreet2) > 200 {
		vm.FormError = "Street address must be 200 characters or fewer."
	}
	if len(addrCity) > 100 || len(addrProvince) > 100 || len(addrCountry) > 100 {
		vm.FormError = "City, province, and country must be 100 characters or fewer."
	}
	if len(addrPostalCode) > 20 {
		vm.FormError = "Postal code must be 20 characters or fewer."
	} else if addrPostalCode != "" && !rePostalCode.MatchString(addrPostalCode) {
		vm.FormError = "Postal code may only contain letters, numbers, spaces, and hyphens."
	}

	if vm.NameError != "" || vm.FormError != "" {
		return pages.CustomerNew(vm).Render(c.Context(), c)
	}

	// ── Duplicate name check ──────────────────────────────────────────────────
	var count int64
	if err := s.DB.Model(&models.Customer{}).
		Where("company_id = ? AND lower(name) = lower(?)", companyID, name).
		Count(&count).Error; err != nil {
		vm.FormError = "Could not validate customer name."
		return pages.CustomerNew(vm).Render(c.Context(), c)
	}
	if count > 0 {
		vm.NameError = "A customer with this name already exists for this company."
		return pages.CustomerNew(vm).Render(c.Context(), c)
	}

	customer := models.Customer{
		CompanyID:      companyID,
		Name:           name,
		Email:          email,
		PaymentTerm:    paymentTerm,
		AddrStreet1:    addrStreet1,
		AddrStreet2:    addrStreet2,
		AddrCity:       addrCity,
		AddrProvince:   addrProvince,
		AddrPostalCode: addrPostalCode,
		AddrCountry:    addrCountry,
	}
	if err := s.DB.Create(&customer).Error; err != nil {
		vm.FormError = "Could not create customer. Please try again."
		return pages.CustomerNew(vm).Render(c.Context(), c)
	}

	cid := companyID
	uid := user.ID
	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	services.TryWriteAuditLogWithContext(s.DB, "customer.created", "customer", customer.ID, actor, map[string]any{
		"name":       name,
		"company_id": companyID,
	}, &cid, &uid)

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/customers?created=1")
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect("/customers?created=1", fiber.StatusSeeOther)
}

func (s *Server) customersForCompany(companyID uint) ([]models.Customer, error) {
	var customers []models.Customer
	err := s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&customers).Error
	return customers, err
}
