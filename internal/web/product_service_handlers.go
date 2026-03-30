// 遵循project_guide.md
package web

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

func (s *Server) handleProductServices(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	vm := pages.ProductServicesVM{
		HasCompany: true,
		Created:    c.Query("created") == "1",
		Updated:    c.Query("updated") == "1",
		InactiveOK: c.Query("inactive") == "1",
	}

	if c.Query("new") == "1" {
		vm.DrawerOpen = true
		vm.DrawerMode = "create"
	}

	if editRaw := strings.TrimSpace(c.Query("edit")); editRaw != "" {
		id64, err := strconv.ParseUint(editRaw, 10, 64)
		if err == nil && id64 > 0 {
			var item models.ProductService
			if err := s.DB.Where("id = ? AND company_id = ?", uint(id64), companyID).First(&item).Error; err == nil {
				vm.DrawerOpen = true
				vm.DrawerMode = "edit"
				vm.EditingID = uint(id64)
				vm.Name = item.Name
				vm.Type = string(item.Type)
				vm.Description = item.Description
				vm.DefaultPrice = item.DefaultPrice.StringFixed(2)
				vm.RevenueAccountID = strconv.FormatUint(uint64(item.RevenueAccountID), 10)
				if item.DefaultTaxCodeID != nil {
					vm.DefaultTaxCodeID = strconv.FormatUint(uint64(*item.DefaultTaxCodeID), 10)
				}
			}
		}
	}

	if err := s.loadProductServicesDropdowns(companyID, &vm); err != nil {
		vm.FormError = "Could not load dropdown data."
	}

	var items []models.ProductService
	if err := s.DB.Preload("RevenueAccount").
		Where("company_id = ?", companyID).
		Order("name asc").
		Find(&items).Error; err != nil {
		vm.FormError = "Could not load items."
		vm.Items = []models.ProductService{}
	} else {
		vm.Items = items
	}

	return pages.ProductServices(vm).Render(c.Context(), c)
}

func (s *Server) handleProductServiceCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	name := strings.TrimSpace(c.FormValue("name"))
	typeRaw := strings.TrimSpace(c.FormValue("type"))
	description := strings.TrimSpace(c.FormValue("description"))
	priceRaw := strings.TrimSpace(c.FormValue("default_price"))
	revenueIDRaw := strings.TrimSpace(c.FormValue("revenue_account_id"))
	taxCodeIDRaw := strings.TrimSpace(c.FormValue("default_tax_code_id"))

	vm := pages.ProductServicesVM{
		HasCompany:       true,
		DrawerMode:       "create",
		DrawerOpen:       true,
		Name:             name,
		Type:             typeRaw,
		Description:      description,
		DefaultPrice:     priceRaw,
		RevenueAccountID: revenueIDRaw,
		DefaultTaxCodeID: taxCodeIDRaw,
	}
	_ = s.loadProductServicesDropdowns(companyID, &vm)

	if name == "" {
		vm.NameError = "Name is required."
	}
	psType, typeErr := models.ParseProductServiceType(typeRaw)
	if typeErr != nil {
		vm.TypeError = "Type is required."
	}

	price := decimal.Zero
	if priceRaw != "" {
		var parseErr error
		price, parseErr = decimal.NewFromString(priceRaw)
		if parseErr != nil || price.IsNegative() {
			vm.DefaultPriceError = "Enter a valid non-negative amount (e.g. 150.00)."
		}
	}

	revenueID64, ridErr := strconv.ParseUint(revenueIDRaw, 10, 64)
	if ridErr != nil || revenueID64 == 0 {
		vm.RevenueAccountIDError = "Revenue account is required."
	}

	if vm.NameError != "" || vm.TypeError != "" || vm.DefaultPriceError != "" || vm.RevenueAccountIDError != "" {
		s.loadItemsForVM(companyID, &vm)
		return pages.ProductServices(vm).Render(c.Context(), c)
	}

	// Check duplicate name within company.
	var count int64
	if err := s.DB.Model(&models.ProductService{}).
		Where("company_id = ? AND lower(name) = lower(?)", companyID, name).
		Count(&count).Error; err != nil {
		vm.FormError = "Could not validate item name."
		s.loadItemsForVM(companyID, &vm)
		return pages.ProductServices(vm).Render(c.Context(), c)
	}
	if count > 0 {
		vm.NameError = "An item with this name already exists for this company."
		s.loadItemsForVM(companyID, &vm)
		return pages.ProductServices(vm).Render(c.Context(), c)
	}

	var taxCodeID *uint
	if taxCodeIDRaw != "" {
		id64, err := strconv.ParseUint(taxCodeIDRaw, 10, 64)
		if err == nil && id64 > 0 {
			id := uint(id64)
			taxCodeID = &id
		}
	}

	item := models.ProductService{
		CompanyID:          companyID,
		Name:               name,
		Type:               psType,
		Description:        description,
		DefaultPrice:       price,
		RevenueAccountID:   uint(revenueID64),
		DefaultTaxCodeID:   taxCodeID,
		IsActive:           true,
	}
	if err := s.DB.Create(&item).Error; err != nil {
		vm.FormError = "Could not create item. Please try again."
		s.loadItemsForVM(companyID, &vm)
		return pages.ProductServices(vm).Render(c.Context(), c)
	}

	cid := companyID
	uid := user.ID
	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	services.TryWriteAuditLogWithContext(s.DB, "product_service.created", "product_service", item.ID, actor, map[string]any{
		"name":       name,
		"type":       typeRaw,
		"company_id": companyID,
	}, &cid, &uid)

	return c.Redirect("/products-services?created=1", fiber.StatusSeeOther)
}

func (s *Server) handleProductServiceUpdate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	idRaw := strings.TrimSpace(c.FormValue("item_id"))
	id64, idErr := strconv.ParseUint(idRaw, 10, 64)
	if idErr != nil || id64 == 0 {
		return c.Redirect("/products-services", fiber.StatusSeeOther)
	}
	itemID := uint(id64)

	var existing models.ProductService
	if err := s.DB.Where("id = ? AND company_id = ?", itemID, companyID).First(&existing).Error; err != nil {
		return c.Redirect("/products-services", fiber.StatusSeeOther)
	}

	name := strings.TrimSpace(c.FormValue("name"))
	typeRaw := strings.TrimSpace(c.FormValue("type"))
	description := strings.TrimSpace(c.FormValue("description"))
	priceRaw := strings.TrimSpace(c.FormValue("default_price"))
	revenueIDRaw := strings.TrimSpace(c.FormValue("revenue_account_id"))
	taxCodeIDRaw := strings.TrimSpace(c.FormValue("default_tax_code_id"))

	vm := pages.ProductServicesVM{
		HasCompany:       true,
		DrawerMode:       "edit",
		DrawerOpen:       true,
		EditingID:        itemID,
		Name:             name,
		Type:             typeRaw,
		Description:      description,
		DefaultPrice:     priceRaw,
		RevenueAccountID: revenueIDRaw,
		DefaultTaxCodeID: taxCodeIDRaw,
	}
	_ = s.loadProductServicesDropdowns(companyID, &vm)

	if name == "" {
		vm.NameError = "Name is required."
	}
	psType, typeErr := models.ParseProductServiceType(typeRaw)
	if typeErr != nil {
		vm.TypeError = "Type is required."
	}

	price := decimal.Zero
	if priceRaw != "" {
		var parseErr error
		price, parseErr = decimal.NewFromString(priceRaw)
		if parseErr != nil || price.IsNegative() {
			vm.DefaultPriceError = "Enter a valid non-negative amount (e.g. 150.00)."
		}
	}

	revenueID64, ridErr := strconv.ParseUint(revenueIDRaw, 10, 64)
	if ridErr != nil || revenueID64 == 0 {
		vm.RevenueAccountIDError = "Revenue account is required."
	}

	if vm.NameError != "" || vm.TypeError != "" || vm.DefaultPriceError != "" || vm.RevenueAccountIDError != "" {
		s.loadItemsForVM(companyID, &vm)
		return pages.ProductServices(vm).Render(c.Context(), c)
	}

	var taxCodeID *uint
	if taxCodeIDRaw != "" {
		tcid64, err := strconv.ParseUint(taxCodeIDRaw, 10, 64)
		if err == nil && tcid64 > 0 {
			id := uint(tcid64)
			taxCodeID = &id
		}
	}

	existing.Name = name
	existing.Type = psType
	existing.Description = description
	existing.DefaultPrice = price
	existing.RevenueAccountID = uint(revenueID64)
	existing.DefaultTaxCodeID = taxCodeID

	if err := s.DB.Save(&existing).Error; err != nil {
		vm.FormError = "Could not update item. Please try again."
		s.loadItemsForVM(companyID, &vm)
		return pages.ProductServices(vm).Render(c.Context(), c)
	}

	cid := companyID
	uid := user.ID
	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	services.TryWriteAuditLogWithContext(s.DB, "product_service.updated", "product_service", existing.ID, actor, map[string]any{
		"name":       name,
		"type":       typeRaw,
		"company_id": companyID,
	}, &cid, &uid)

	return c.Redirect("/products-services?updated=1", fiber.StatusSeeOther)
}

func (s *Server) handleProductServiceInactive(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	idRaw := strings.TrimSpace(c.FormValue("item_id"))
	id64, idErr := strconv.ParseUint(idRaw, 10, 64)
	if idErr != nil || id64 == 0 {
		return c.Redirect("/products-services", fiber.StatusSeeOther)
	}
	itemID := uint(id64)

	var item models.ProductService
	if err := s.DB.Where("id = ? AND company_id = ?", itemID, companyID).First(&item).Error; err != nil {
		return c.Redirect("/products-services", fiber.StatusSeeOther)
	}
	if !item.IsActive {
		return c.Redirect("/products-services", fiber.StatusSeeOther)
	}

	if err := s.DB.Model(&item).Update("is_active", false).Error; err != nil {
		return c.Redirect("/products-services", fiber.StatusSeeOther)
	}

	cid := companyID
	uid := user.ID
	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	services.TryWriteAuditLogWithContext(s.DB, "product_service.deactivated", "product_service", item.ID, actor, map[string]any{
		"name":       item.Name,
		"company_id": companyID,
	}, &cid, &uid)

	return c.Redirect("/products-services?inactive=1", fiber.StatusSeeOther)
}

// loadProductServicesDropdowns fills RevenueAccounts and TaxCodes on the VM.
func (s *Server) loadProductServicesDropdowns(companyID uint, vm *pages.ProductServicesVM) error {
	if err := s.DB.
		Where("company_id = ? AND is_active = true AND root_account_type = 'revenue'", companyID).
		Order("code asc").
		Find(&vm.RevenueAccounts).Error; err != nil {
		return err
	}
	if err := s.DB.
		Where("company_id = ? AND is_active = true", companyID).
		Order("name asc").
		Find(&vm.TaxCodes).Error; err != nil {
		return err
	}
	return nil
}

// loadItemsForVM fetches the item list for re-rendering on validation errors.
func (s *Server) loadItemsForVM(companyID uint, vm *pages.ProductServicesVM) {
	var items []models.ProductService
	if err := s.DB.Preload("RevenueAccount").
		Where("company_id = ?", companyID).
		Order("name asc").
		Find(&items).Error; err == nil {
		vm.Items = items
	}
}
