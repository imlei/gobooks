// 遵循project_guide.md
package web

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gobooks/internal/models"
)

// handleSmartPickerSearch serves GET /api/smart-picker/search.
// Query params:
//
//	entity  — required; maps to SmartPickerProvider.EntityType() (e.g. "account")
//	context — optional; narrows purpose within the entity (e.g. "expense_form_category")
//	q       — optional search string
//	limit   — optional integer 1–20 (default 20; hard cap 20)
//
// companyID is always sourced from the authenticated session — never from query params.
func (s *Server) handleSmartPickerSearch(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no active company"})
	}

	entity := c.Query("entity")
	if entity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "entity param required"})
	}

	provider, ok := defaultSmartPickerRegistry.get(entity)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unknown entity: " + entity})
	}

	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 20 {
		limit = 20
	}

	ctx := SmartPickerContext{
		CompanyID: companyID,
		Context:   c.Query("context"),
		Limit:     limit,
	}

	requestID := c.Query("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	result, source, err := s.SPAcceleration.Search(s.DB, provider, ctx, c.Query("q"))
	if err != nil {
		slog.Error("smart_picker.search_error",
			"entity", entity,
			"company_id", companyID,
			"request_id", requestID,
			"error", err,
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "search failed"})
	}

	result.RequestID = requestID
	result.Source = source
	result.RequiresBackendValidation = true

	slog.Info("smart_picker.search",
		"entity", entity,
		"context", c.Query("context"),
		"company_id", companyID,
		"q", c.Query("q"),
		"source", source,
		"count", len(result.Candidates),
		"request_id", requestID,
	)

	return c.JSON(result)
}

// handleSmartPickerUsage serves POST /api/smart-picker/usage.
// Called by the frontend (fire-and-forget) when the user selects an item from
// the picker. Used for future ranking/popularity signals; does not affect
// correctness or authorization. Always returns 204.
//
// JSON body: {"entity": "account", "context": "...", "item_id": "42", "request_id": "uuid"}
func (s *Server) handleSmartPickerUsage(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.SendStatus(fiber.StatusNoContent)
	}

	var body struct {
		Entity    string `json:"entity"`
		Context   string `json:"context"`
		ItemID    string `json:"item_id"`
		RequestID string `json:"request_id"`
	}
	// Ignore parse errors — usage pings are best-effort.
	_ = c.BodyParser(&body)

	slog.Info("smart_picker.usage",
		"entity", body.Entity,
		"context", body.Context,
		"item_id", body.ItemID,
		"request_id", body.RequestID,
		"company_id", companyID,
	)

	// Persist selection event for future ranking. Failures are non-fatal.
	if itemID64, err := strconv.ParseUint(body.ItemID, 10, 64); err == nil && itemID64 > 0 {
		usage := models.SmartPickerUsage{
			CompanyID: companyID,
			Entity:    body.Entity,
			Context:   body.Context,
			ItemID:    uint(itemID64),
			RequestID: body.RequestID,
		}
		if err := s.DB.Create(&usage).Error; err != nil {
			slog.Warn("smart_picker.usage_persist_failed",
				"entity", body.Entity,
				"item_id", body.ItemID,
				"error", err,
			)
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}
