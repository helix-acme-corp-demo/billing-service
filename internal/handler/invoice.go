package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// Plan base prices in cents.
var planPrices = map[string]int64{
	"free":       0,
	"pro":        4999,
	"enterprise": 19999,
}

// Per-unit usage costs in cents.
var usageCosts = map[string]int64{
	"api_calls":     1,
	"storage_gb":    50,
	"compute_hours": 100,
}

// InvoiceHandler handles invoice-related HTTP requests.
type InvoiceHandler struct {
	store  *store.Store
	logger logpipe.Logger
}

// NewInvoice creates a new InvoiceHandler.
func NewInvoice(s *store.Store, l logpipe.Logger) *InvoiceHandler {
	return &InvoiceHandler{
		store:  s,
		logger: l,
	}
}

// Generate returns an HTTP handler that generates an invoice for a subscription.
func (h *InvoiceHandler) Generate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.GenerateInvoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			envelope.Write(w, envelope.BadRequest("invalid_body", "invalid request body"))
			return
		}

		if req.SubscriptionID == "" {
			envelope.Write(w, envelope.BadRequest("missing_fields", "subscription_id is required"))
			return
		}

		sub, ok := h.store.FindSubscription(req.SubscriptionID)
		if !ok {
			envelope.Write(w, envelope.NotFound("subscription not found"))
			return
		}

		// Calculate base price from plan.
		basePrice, known := planPrices[sub.Plan]
		if !known {
			basePrice = 0
		}

		// Sum up usage costs.
		var usageTotal int64
		records := h.store.UsageBySubscription(req.SubscriptionID)
		for _, rec := range records {
			cost, hasCost := usageCosts[rec.Metric]
			if hasCost {
				usageTotal += rec.Quantity * cost
			}
		}

		invoice := &domain.Invoice{
			ID:             generateUUID(),
			SubscriptionID: req.SubscriptionID,
			Amount:         basePrice + usageTotal,
			Currency:       "usd",
			Status:         "draft",
			IssuedAt:       time.Now().UTC(),
		}

		h.store.SaveInvoice(invoice)
		h.logger.Info("invoice generated",
			logpipe.String("invoice_id", invoice.ID),
			logpipe.String("subscription_id", invoice.SubscriptionID),
		)

		envelope.Write(w, envelope.Created(invoice))
	}
}

// Get returns an HTTP handler that retrieves an invoice by ID.
func (h *InvoiceHandler) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		invoice, ok := h.store.FindInvoice(id)
		if !ok {
			envelope.Write(w, envelope.NotFound("invoice not found"))
			return
		}

		envelope.Write(w, envelope.OK(invoice))
	}
}

// List returns an HTTP handler that lists invoices for a subscription.
func (h *InvoiceHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subID := r.URL.Query().Get("subscription_id")
		if subID == "" {
			envelope.Write(w, envelope.BadRequest("missing_param", "subscription_id query parameter is required"))
			return
		}

		invoices := h.store.InvoicesBySubscription(subID)
		envelope.Write(w, envelope.OK(invoices))
	}
}
