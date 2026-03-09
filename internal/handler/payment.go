package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	helixpay "github.com/helix-acme-corp-demo/helix-pay-go"
	"github.com/helix-acme-corp-demo/helix-pay-go/charges"
	"github.com/helix-acme-corp-demo/helix-pay-go/currency"
	"github.com/helix-acme-corp-demo/helix-pay-go/customers"
	"github.com/helix-acme-corp-demo/helix-pay-go/idempotency"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// PaymentHandler handles invoice payment via HelixPay.
type PaymentHandler struct {
	store       *store.Store
	helixClient *helixpay.Client
	logger      logpipe.Logger
}

// NewPayment creates a new PaymentHandler.
func NewPayment(s *store.Store, c *helixpay.Client, l logpipe.Logger) *PaymentHandler {
	return &PaymentHandler{
		store:       s,
		helixClient: c,
		logger:      l,
	}
}

// Pay returns an HTTP handler that pays an invoice via HelixPay.
// The request body must contain either:
//   - customer_id: an existing HelixPay customer ID, or
//   - email + name: used to auto-register a new HelixPay customer.
//
// On success the handler responds 202 Accepted with the updated invoice
// (status "pending_payment"). Final settlement is confirmed via webhook.
func (h *PaymentHandler) Pay() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoiceID := chi.URLParam(r, "id")

		var req domain.PayInvoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			envelope.Write(w, envelope.BadRequest("invalid_body", "invalid request body"))
			return
		}

		// Validate: need either customer_id OR email (+ name).
		if req.CustomerID == "" && req.Email == "" {
			envelope.Write(w, envelope.BadRequest("missing_customer_identity", "provide either customer_id or email and name"))
			return
		}

		// Look up the invoice.
		invoice, ok := h.store.FindInvoice(invoiceID)
		if !ok {
			envelope.Write(w, envelope.NotFound("invoice not found"))
			return
		}

		// Only draft invoices can be paid.
		if invoice.Status != "draft" {
			envelope.Write(w, envelope.BadRequest("invoice_not_payable", fmt.Sprintf("invoice status is %q, only draft invoices can be paid", invoice.Status)))
			return
		}

		// Resolve the HelixPay customer ID.
		customerID := req.CustomerID
		if customerID == "" {
			reg := customers.NewRegistration(req.Email, req.Name)
			customer, err := h.helixClient.Customers.Register(r.Context(), reg)
			if err != nil {
				h.logger.Error("helixpay: failed to register customer",
					logpipe.String("email", req.Email),
					logpipe.String("error", err.Error()),
				)
				envelope.Write(w, envelope.InternalError("failed to register customer with HelixPay"))
				return
			}
			customerID = customer.ID
			h.logger.Info("helixpay: customer registered",
				logpipe.String("customer_id", customerID),
				logpipe.String("email", req.Email),
			)
		}

		// Parse the invoice currency for HelixPay.
		cur := currency.ParseCode(invoice.Currency)

		// Build charge request with a deterministic idempotency key so retries
		// don't create duplicate charges within the 24-hour dedup window.
		idemKey := idempotency.FromComponents("invoice", invoiceID)
		chargeReq, err := charges.NewBuilder(customerID, invoice.Amount, cur).
			WithIdempotencyKey(idemKey.String()).
			WithMetadata("invoice_id", invoiceID).
			WithMetadata("subscription_id", invoice.SubscriptionID).
			Build()
		if err != nil {
			h.logger.Error("helixpay: failed to build charge request",
				logpipe.String("invoice_id", invoiceID),
				logpipe.String("error", err.Error()),
			)
			envelope.Write(w, envelope.BadRequest("invalid_charge", err.Error()))
			return
		}

		// Initiate the charge — HelixPay processes asynchronously.
		charge, err := h.helixClient.Charges.Initiate(r.Context(), chargeReq)
		if err != nil {
			h.logger.Error("helixpay: failed to initiate charge",
				logpipe.String("invoice_id", invoiceID),
				logpipe.String("customer_id", customerID),
				logpipe.String("error", err.Error()),
			)
			envelope.Write(w, envelope.InternalError("failed to initiate charge with HelixPay"))
			return
		}

		// Update invoice state and persist.
		invoice.Status = "pending_payment"
		invoice.HelixPayChargeID = charge.ID
		invoice.PaymentMethod = "helixpay"
		h.store.SaveInvoice(invoice)

		h.logger.Info("helixpay: charge initiated",
			logpipe.String("invoice_id", invoiceID),
			logpipe.String("charge_id", charge.ID),
			logpipe.String("customer_id", customerID),
		)

		// Respond 202 Accepted — settlement is async via webhook.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(envelope.OK(invoice))
	}
}
