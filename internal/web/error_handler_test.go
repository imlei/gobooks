package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestErrorHandlerDoesNotExposeRaw5xxMessage(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: NewErrorHandler(nil),
	})
	app.Get("/boom", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusInternalServerError, "database exploded with secret details")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	got := string(body[:n])

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
	if got != "Internal Server Error" {
		t.Fatalf("expected generic 5xx body, got %q", got)
	}
}
