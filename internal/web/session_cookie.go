// 遵循产品需求 v1.0
package web

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"gobooks/internal/config"
)

// SessionCookieName is the HTTP-only cookie holding the opaque session token.
const SessionCookieName = "gobooks_session"

// SessionCookieMaxAgeSec is the default browser cookie lifetime (30 days).
const SessionCookieMaxAgeSec = 30 * 24 * 3600

// setSessionCookie stores the opaque session token in a browser cookie.
func setSessionCookie(c *fiber.Ctx, cfg config.Config, rawToken string, maxAgeSeconds int) {
	sec := strings.EqualFold(cfg.Env, "production") || strings.EqualFold(cfg.Env, "prod")
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   sec,
		MaxAge:   maxAgeSeconds,
	})
}

// clearSessionCookie removes the session cookie from the browser.
func clearSessionCookie(c *fiber.Ctx, cfg config.Config) {
	sec := strings.EqualFold(cfg.Env, "production") || strings.EqualFold(cfg.Env, "prod")
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   sec,
		MaxAge:   -1,
	})
}
