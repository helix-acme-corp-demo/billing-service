package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helix-acme-corp-demo/cachex"
	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// SubscriptionHandler handles subscription-related HTTP requests.
type SubscriptionHandler struct {
	store  *store.Store
	cache  cachex.Cache
	logger logpipe.Logger
}

// NewSubscription creates a new SubscriptionHandler.
func NewSubscription(s *store.Store, c cachex.Cache, l logpipe.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{
		store:  s,
		cache:  c,
		logger: l,
	}
}

// Create returns an HTTP handler that creates a new subscription.
func (h *SubscriptionHandler) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.CreateSubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			envelope.Write(w, envelope.BadRequest("invalid_body", "invalid request body"))
			return
		}

		if req.UserID == "" || req.Plan == "" {
			envelope.Write(w, envelope.BadRequest("missing_fields", "user_id and plan are required"))
			return
		}

		now := time.Now().UTC()
		sub := &domain.Subscription{
			ID:        generateUUID(),
			UserID:    req.UserID,
			Plan:      req.Plan,
			Status:    "active",
			CreatedAt: now,
			ExpiresAt: now.Add(30 * 24 * time.Hour),
		}

		h.store.SaveSubscription(sub)
		h.logger.Info("subscription created",
			logpipe.String("subscription_id", sub.ID),
			logpipe.String("user_id", sub.UserID),
			logpipe.String("plan", sub.Plan),
		)

		envelope.Write(w, envelope.Created(sub))
	}
}

// Get returns an HTTP handler that retrieves a subscription by ID.
func (h *SubscriptionHandler) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Check cache first.
		cached, err := h.cache.Get(context.Background(), "subscription:"+id)
		if err == nil {
			var sub domain.Subscription
			if jsonErr := json.Unmarshal(cached, &sub); jsonErr == nil {
				envelope.Write(w, envelope.OK(&sub))
				return
			}
		}

		sub, ok := h.store.FindSubscription(id)
		if !ok {
			envelope.Write(w, envelope.NotFound("subscription not found"))
			return
		}

		// Populate cache.
		if data, marshalErr := json.Marshal(sub); marshalErr == nil {
			h.cache.Set(context.Background(), "subscription:"+id, data, 5*time.Minute)
		}

		envelope.Write(w, envelope.OK(sub))
	}
}

// List returns an HTTP handler that lists all subscriptions.
func (h *SubscriptionHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subscriptions := h.store.AllSubscriptions()
		envelope.Write(w, envelope.OK(subscriptions))
	}
}

// Cancel returns an HTTP handler that cancels a subscription.
func (h *SubscriptionHandler) Cancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		sub, ok := h.store.FindSubscription(id)
		if !ok {
			envelope.Write(w, envelope.NotFound("subscription not found"))
			return
		}

		sub.Status = "canceled"
		h.store.SaveSubscription(sub)

		// Invalidate cache.
		h.cache.Delete(context.Background(), "subscription:"+id)

		h.logger.Info("subscription canceled",
			logpipe.String("subscription_id", sub.ID),
		)

		envelope.Write(w, envelope.OK(sub))
	}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
