package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/helix-acme-corp-demo/cachex"
	"github.com/helix-acme-corp-demo/logpipe"
	_ "github.com/helix-acme-corp-demo/retryx"

	"github.com/helix-acme-corp-demo/billing-service/config"
	"github.com/helix-acme-corp-demo/billing-service/internal/handler"
	"github.com/helix-acme-corp-demo/billing-service/internal/middleware"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

func main() {
	cfg := config.Load()
	logger := logpipe.New()

	cache := cachex.Memory(
		cachex.WithDefaultTTL(5*time.Minute),
		cachex.WithMaxSize(1000),
	)

	billingStore := store.New()
	subHandler := handler.NewSubscription(billingStore, cache, logger)
	usageHandler := handler.NewUsage(billingStore, logger)
	invoiceHandler := handler.NewInvoice(billingStore, logger)
	authHandler := handler.NewAuth(billingStore, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, logger)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(logpipe.Middleware(logger))
	r.Use(chimiddleware.Recoverer)

	// Public routes — no authentication required.
	r.Get("/health", handler.Health())
	r.Post("/auth/refresh", authHandler.Refresh())
	r.Post("/auth/revoke", authHandler.Revoke())

	// Protected billing routes — require a valid JWT and the appropriate scope.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(billingStore, cfg.JWTSecret, logger))

		r.With(middleware.RequireScope("billing:subscriptions:write")).Post("/subscriptions", subHandler.Create())
		r.With(middleware.RequireScope("billing:subscriptions:read")).Get("/subscriptions", subHandler.List())
		r.With(middleware.RequireScope("billing:subscriptions:read")).Get("/subscriptions/{id}", subHandler.Get())
		r.With(middleware.RequireScope("billing:subscriptions:write")).Post("/subscriptions/{id}/cancel", subHandler.Cancel())

		r.With(middleware.RequireScope("billing:usage:write")).Post("/usage", usageHandler.Record())
		r.With(middleware.RequireScope("billing:usage:read")).Get("/usage", usageHandler.List())

		r.With(middleware.RequireScope("billing:invoices:write")).Post("/invoices/generate", invoiceHandler.Generate())
		r.With(middleware.RequireScope("billing:invoices:read")).Get("/invoices/{id}", invoiceHandler.Get())
		r.With(middleware.RequireScope("billing:invoices:read")).Get("/invoices", invoiceHandler.List())
	})

	logger.Info("starting billing-service", logpipe.String("port", cfg.Port))
	http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), r)
}
