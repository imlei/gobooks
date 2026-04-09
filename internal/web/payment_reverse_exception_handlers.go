// 遵循project_guide.md
package web

// payment_reverse_exception_handlers.go — Batch 23: Payment reverse exception UI.
//
// Routes (registered in routes.go):
//
//   GET  /settings/payment-gateways/reverse-exceptions
//          — list all payment reverse exceptions for the active company
//   GET  /settings/payment-gateways/reverse-exceptions/:id
//          — exception detail + status action forms + linked transaction summaries
//   POST /settings/payment-gateways/reverse-exceptions/:id/review
//          — transition open → reviewed
//   POST /settings/payment-gateways/reverse-exceptions/:id/dismiss
//          — transition to dismissed (terminal)
//   POST /settings/payment-gateways/reverse-exceptions/:id/resolve
//          — transition to resolved (terminal)

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

const reverseExceptionListBase = "/settings/payment-gateways/reverse-exceptions"

// handlePaymentReverseExceptionList renders the exception list page.
// GET /settings/payment-gateways/reverse-exceptions
func (s *Server) handlePaymentReverseExceptionList(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	exceptions, _ := services.ListPaymentReverseExceptions(s.DB, companyID)
	vm := pages.PaymentReverseExceptionListVM{
		HasCompany:   true,
		Exceptions:   exceptions,
		JustActioned: c.Query("actioned") == "1",
		FormError:    strings.TrimSpace(c.Query("error")),
	}
	return pages.PaymentReverseExceptionList(vm).Render(c.Context(), c)
}

// handlePaymentReverseExceptionDetail renders the exception detail page.
// GET /settings/payment-gateways/reverse-exceptions/:id
func (s *Server) handlePaymentReverseExceptionDetail(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	id64, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id64 == 0 {
		return c.Redirect(reverseExceptionListBase, fiber.StatusSeeOther)
	}

	ex, err := services.GetPaymentReverseException(s.DB, companyID, uint(id64))
	if err != nil {
		return c.Redirect(reverseExceptionListBase, fiber.StatusSeeOther)
	}

	vm := pages.PaymentReverseExceptionDetailVM{
		HasCompany:   true,
		Exception:    ex,
		JustActioned: c.Query("actioned") == "1",
		ActionError:  strings.TrimSpace(c.Query("error")),
	}

	// Load linked transaction summaries.
	if ex.ReverseTxnID != nil {
		var reverseTxn models.PaymentTransaction
		if err := s.DB.Where("id = ? AND company_id = ?", *ex.ReverseTxnID, companyID).First(&reverseTxn).Error; err == nil {
			vm.ReverseTxn = &reverseTxn
		}
	}
	if ex.OriginalTxnID != nil {
		var originalTxn models.PaymentTransaction
		if err := s.DB.Where("id = ? AND company_id = ?", *ex.OriginalTxnID, companyID).First(&originalTxn).Error; err == nil {
			vm.OriginalTxn = &originalTxn
		}
	}

	return pages.PaymentReverseExceptionDetail(vm).Render(c.Context(), c)
}

// handlePaymentReverseExceptionReview processes the review transition.
// POST /settings/payment-gateways/reverse-exceptions/:id/review
func (s *Server) handlePaymentReverseExceptionReview(c *fiber.Ctx) error {
	return s.applyReverseExceptionStatusAction(c, func(companyID, id uint, actor string) error {
		return services.ReviewPaymentReverseException(s.DB, companyID, id, actor)
	})
}

// handlePaymentReverseExceptionDismiss processes the dismiss transition.
// POST /settings/payment-gateways/reverse-exceptions/:id/dismiss
func (s *Server) handlePaymentReverseExceptionDismiss(c *fiber.Ctx) error {
	note := strings.TrimSpace(c.FormValue("note"))
	return s.applyReverseExceptionStatusAction(c, func(companyID, id uint, actor string) error {
		return services.DismissPaymentReverseException(s.DB, companyID, id, actor, note)
	})
}

// handlePaymentReverseExceptionResolve processes the resolve transition.
// POST /settings/payment-gateways/reverse-exceptions/:id/resolve
func (s *Server) handlePaymentReverseExceptionResolve(c *fiber.Ctx) error {
	note := strings.TrimSpace(c.FormValue("note"))
	return s.applyReverseExceptionStatusAction(c, func(companyID, id uint, actor string) error {
		return services.ResolvePaymentReverseException(s.DB, companyID, id, actor, note)
	})
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (s *Server) applyReverseExceptionStatusAction(
	c *fiber.Ctx,
	action func(companyID, id uint, actor string) error,
) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	id64, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id64 == 0 {
		return c.Redirect(reverseExceptionListBase, fiber.StatusSeeOther)
	}
	detailBase := reverseExceptionListBase + "/" + c.Params("id")

	if err := action(companyID, uint(id64), exceptionActor(c)); err != nil {
		return redirectWithQuery(c, detailBase, "error", reverseExceptionActionErrMessage(err))
	}
	return redirectWithQuery(c, detailBase, "actioned", "1")
}

// reverseExceptionActionErrMessage translates status-transition errors into user-facing messages.
func reverseExceptionActionErrMessage(err error) string {
	switch {
	case errors.Is(err, services.ErrPRExceptionAlreadyClosed):
		return "This exception is already in a terminal state and cannot be changed."
	case errors.Is(err, services.ErrPRExceptionTransitionInvalid):
		return "Invalid status transition."
	case errors.Is(err, services.ErrPRExceptionDismissNote):
		return "Dismissal note is required."
	case errors.Is(err, services.ErrPRExceptionNotFound):
		return "Exception not found."
	default:
		return "Action failed: " + err.Error()
	}
}
