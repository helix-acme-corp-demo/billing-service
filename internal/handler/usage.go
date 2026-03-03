package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// UsageHandler handles usage-related HTTP requests.
type UsageHandler struct {
	store  *store.Store
	logger logpipe.Logger
}

// NewUsage creates a new UsageHandler.
func NewUsage(s *store.Store, l logpipe.Logger) *UsageHandler {
	return &UsageHandler{
		store:  s,
		logger: l,
	}
}

// Record returns an HTTP handler that records a usage event.
func (h *UsageHandler) Record() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.RecordUsageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			envelope.Write(w, envelope.BadRequest("invalid_body", "invalid request body"))
			return
		}

		if req.SubscriptionID == "" || req.Metric == "" {
			envelope.Write(w, envelope.BadRequest("missing_fields", "subscription_id and metric are required"))
			return
		}

		record := &domain.UsageRecord{
			ID:             generateUUID(),
			SubscriptionID: req.SubscriptionID,
			Metric:         req.Metric,
			Quantity:       req.Quantity,
			RecordedAt:     time.Now().UTC(),
		}

		h.store.SaveUsage(record)
		h.logger.Info("usage recorded",
			logpipe.String("usage_id", record.ID),
			logpipe.String("subscription_id", record.SubscriptionID),
			logpipe.String("metric", record.Metric),
		)

		envelope.Write(w, envelope.Created(record))
	}
}

// List returns an HTTP handler that lists usage records for a subscription.
func (h *UsageHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subID := r.URL.Query().Get("subscription_id")
		if subID == "" {
			envelope.Write(w, envelope.BadRequest("missing_param", "subscription_id query parameter is required"))
			return
		}

		records := h.store.UsageBySubscription(subID)
		envelope.Write(w, envelope.OK(records))
	}
}
