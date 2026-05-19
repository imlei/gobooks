package web

import (
	"context"
	"time"

	"balanciz/internal/version"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) handleHealthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"version": version.Version,
	})
}

func (s *Server) handleVersion(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": version.Version,
	})
}

func (s *Server) handleReadyz(c *fiber.Ctx) error {
	checks := fiber.Map{}
	status := "ready"
	code := fiber.StatusOK

	sqlDB, err := s.DB.DB()
	if err != nil {
		checks["database"] = "unavailable"
		status = "not_ready"
		code = fiber.StatusServiceUnavailable
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			checks["database"] = "unavailable"
			status = "not_ready"
			code = fiber.StatusServiceUnavailable
		} else {
			checks["database"] = "ok"
		}
	}

	return c.Status(code).JSON(fiber.Map{
		"status":  status,
		"version": version.Version,
		"checks":  checks,
	})
}
