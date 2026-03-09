package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/payment"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// PaymentHandler handles payment-related HTTP requests.
type PaymentHandler struct {
	store          *store.Store
	helixPayClient *payment.Client
	logger         logpipe.Logger
}

// NewPayment creates a new PaymentHandler.
func NewPayment(s *store.Store, c *payment.Client, l logpipe.Logger) *PaymentHandler {
	return &PaymentHandler{
		store:          s,
		helixPayClient: c,
		logger:         l,
	}
}

// paymentRequired writes a 402 Payment Required JSON response using the
// envelope shape, since the envelope package has no PaymentRequired helper.
func paymentRequired(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	json.NewEncoder(w).Encode(envelope.Response{
		OK: false,
		Error: &envelope.ErrorDetail{
			Code:    "payment_declined",
			Message: message,
		},
	})
}

// Pay returns an HTTP handler that pays an invoice via HelixPay.
//
// POST /invoices/{id}/pay
// Body: { "token": "<helixpay_payment_token>" }
//
// Responses:
//
//	200 - invoice paid, returns updated Invoice
//	400 - missing token or invoice already paid
//	402 - HelixPay declined the payment
//	404 - invoice not found
//	500 - unexpected error during payment processing
func (h *PaymentHandler) Pay() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req domain.PayInvoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			envelope.Write(w, envelope.BadRequest("invalid_body", "invalid request body"))
			return
		}

		if req.Token == "" {
			envelope.Write(w, envelope.BadRequest("missing_fields", "token is required"))
			return
		}

		invoice, ok := h.store.FindInvoice(id)
		if !ok {
			envelope.Write(w, envelope.NotFound("invoice not found"))
			return
		}

		if invoice.Status == "paid" {
			envelope.Write(w, envelope.BadRequest("already_paid", "invoice has already been paid"))
			return
		}

		err := h.helixPayClient.Charge(r.Context(), req.Token, invoice.Amount, invoice.Currency, invoice.ID)
		if err != nil {
			var payErr *payment.PaymentError
			if errors.As(err, &payErr) {
				h.logger.Info("helixpay payment declined",
					logpipe.String("invoice_id", invoice.ID),
					logpipe.String("error", payErr.Message),
				)
				paymentRequired(w, payErr.Message)
				return
			}

			h.logger.Info("helixpay payment error",
				logpipe.String("invoice_id", invoice.ID),
				logpipe.String("error", err.Error()),
			)
			envelope.Write(w, envelope.InternalError("payment processing failed"))
			return
		}

		now := time.Now().UTC()
		invoice.Status = "paid"
		invoice.PaidAt = &now
		invoice.PaymentMethod = "helixpay"
		h.store.SaveInvoice(invoice)

		h.logger.Info("invoice paid via helixpay",
			logpipe.String("invoice_id", invoice.ID),
			logpipe.String("subscription_id", invoice.SubscriptionID),
		)

		envelope.Write(w, envelope.OK(invoice))
	}
}
