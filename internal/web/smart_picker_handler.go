package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// handleSmartPickerSearch serves GET /api/smart-picker/search.
// Company and user scope always come from the authenticated session.
func (s *Server) handleSmartPickerSearch(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no active company"})
	}

	entity := c.Query("entity")
	if entity == "" {
		entity = c.Query("entity_type")
	}
	if entity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "entity param required"})
	}

	def, err := validateSmartPickerContext(entity, c.Query("context"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := s.requireSmartPickerContextAccess(c, def); err != nil {
		return smartPickerAccessResponse(c, err)
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

	anchorContext, anchorEntityType, anchorEntityID, err := s.smartPickerAnchorFromQuery(companyID, c)
	if err != nil {
		var usageErr smartPickerUsageError
		if errors.As(err, &usageErr) {
			return c.Status(usageErr.status).JSON(fiber.Map{"error": usageErr.message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := SmartPickerContext{
		CompanyID:        companyID,
		Context:          def.ProviderContext,
		Limit:            limit,
		UserID:           smartPickerUserID(c),
		EntityType:       entity,
		Query:            c.Query("q"),
		AnchorContext:    anchorContext,
		AnchorEntityType: anchorEntityType,
		AnchorEntityID:   anchorEntityID,
		TraceEnabled:     s.Cfg.SmartPickerTraceEnabled,
		TraceSampleRate:  s.Cfg.SmartPickerDecisionTraceSampleRate,
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
		"context", ctx.Context,
		"company_id", companyID,
		"q", c.Query("q"),
		"source", source,
		"count", len(result.Candidates),
		"request_id", requestID,
	)

	return c.JSON(result)
}

// handleSmartPickerUsage serves POST /api/smart-picker/usage.
// It records validated, company-scoped behavior events and updates aggregate
// ranking signals. Legacy select pings remain accepted.
func (s *Server) handleSmartPickerUsage(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no active company"})
	}

	var body smartPickerUsageEventInput
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := s.requireSmartPickerUsageContextAccess(c, body); err != nil {
		return smartPickerAccessResponse(c, err)
	}

	slog.Info("smart_picker.usage",
		"entity", firstNonEmpty(body.EntityType, body.Entity),
		"context", body.Context,
		"event_type", body.EventType,
		"selected_entity_id", firstNonEmpty(body.SelectedEntityID, body.ItemID),
		"request_id", body.RequestID,
		"company_id", companyID,
	)

	sessionID := ""
	if sess := SessionFromCtx(c); sess != nil {
		sessionID = sess.ID.String()
	}
	if err := recordSmartPickerUsageEvent(s.DB, companyID, smartPickerUserID(c), sessionID, body); err != nil {
		var usageErr smartPickerUsageError
		if errors.As(err, &usageErr) {
			return c.Status(usageErr.status).JSON(fiber.Map{"error": usageErr.message})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "usage event failed"})
	}

	return c.JSON(fiber.Map{"ok": true})
}

func smartPickerUserID(c *fiber.Ctx) *uuid.UUID {
	user := UserFromCtx(c)
	if user == nil {
		return nil
	}
	id := user.ID
	return &id
}

func (s *Server) smartPickerAnchorFromQuery(companyID uint, c *fiber.Ctx) (string, string, *uint, error) {
	anchorContext := c.Query("anchor_context")
	anchorEntityType := c.Query("anchor_entity_type")
	anchorIDRaw := c.Query("anchor_entity_id")
	if anchorContext == "" && anchorEntityType == "" && anchorIDRaw == "" {
		return "", "", nil, nil
	}
	anchorID, err := parseSmartPickerEntityID(anchorIDRaw)
	if err != nil {
		return "", "", nil, smartPickerUsageError{status: fiber.StatusBadRequest, message: "invalid anchor_entity_id"}
	}
	if anchorID == nil {
		return "", "", nil, smartPickerUsageError{status: fiber.StatusBadRequest, message: "anchor_entity_id required when anchor context is provided"}
	}
	def, err := validateSmartPickerContext(anchorEntityType, anchorContext)
	if err != nil {
		return "", "", nil, smartPickerUsageError{status: fiber.StatusBadRequest, message: "invalid anchor context"}
	}
	if err := s.requireSmartPickerContextAccess(c, def); err != nil {
		return "", "", nil, err
	}
	if err := validateSmartPickerEntityID(s.DB, companyID, smartPickerUserID(c), def.ProviderContext, anchorEntityType, *anchorID); err != nil {
		return "", "", nil, err
	}
	return def.ProviderContext, anchorEntityType, anchorID, nil
}

func (s *Server) requireSmartPickerUsageContextAccess(c *fiber.Ctx, body smartPickerUsageEventInput) error {
	entityType := firstNonEmpty(body.EntityType, body.Entity)
	def, err := validateSmartPickerContext(entityType, body.Context)
	if err != nil {
		return smartPickerUsageError{status: fiber.StatusBadRequest, message: err.Error()}
	}
	if err := s.requireSmartPickerContextAccess(c, def); err != nil {
		return err
	}
	if body.AnchorContext == "" && body.AnchorEntityType == "" && body.AnchorEntityID == "" {
		return nil
	}
	anchorDef, err := validateSmartPickerContext(body.AnchorEntityType, body.AnchorContext)
	if err != nil {
		return smartPickerUsageError{status: fiber.StatusBadRequest, message: "invalid anchor context"}
	}
	return s.requireSmartPickerContextAccess(c, anchorDef)
}

func (s *Server) requireSmartPickerContextAccess(c *fiber.Ctx, def smartPickerContextDefinition) error {
	if def.RequiredFeature != "" && !s.searchFeatureEnabled(c, def.RequiredFeature) {
		return smartPickerUsageError{status: fiber.StatusForbidden, message: "smart picker context is not enabled"}
	}
	if def.RequiredAction != "" && !CanFromCtx(c, def.RequiredAction) {
		return smartPickerUsageError{status: fiber.StatusForbidden, message: "smart picker context is not permitted"}
	}
	return nil
}

func smartPickerAccessResponse(c *fiber.Ctx, err error) error {
	var usageErr smartPickerUsageError
	if errors.As(err, &usageErr) {
		return c.Status(usageErr.status).JSON(fiber.Map{"error": usageErr.message})
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "smart picker context is not permitted"})
}
