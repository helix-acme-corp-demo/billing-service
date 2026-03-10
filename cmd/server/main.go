package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/helix-acme-corp-demo/authtokens"
	"github.com/helix-acme-corp-demo/cachex"
	"github.com/helix-acme-corp-demo/logpipe"
	_ "github.com/helix-acme-corp-demo/retryx"

	"github.com/helix-acme-corp-demo/billing-service/config"
	"github.com/helix-acme-corp-demo/billing-service/internal/auth"
	"github.com/helix-acme-corp-demo/billing-service/internal/handler"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

func main() {
	cfg := config.Load()
	logger := logpipe.New()

	// Build revocation list from config (comma-separated REVOKED_TOKEN_IDS env var).
	revocationList := auth.NewRevocationList(cfg.RevokedTokenIDs)

	// Build JWT issuer and validators.
	secret := []byte(cfg.JWTSecret)

	issuer := authtokens.NewIssuer(
		authtokens.WithSecret(secret),
		authtokens.WithDefaultTTL(cfg.JWTDefaultTTL),
		authtokens.WithAudience("billing-service"),
	)

	// Base validator: checks signature, expiry, audience, and revocation — no scope requirement.
	baseValidator := authtokens.NewValidator(
		authtokens.WithSecret(secret),
		authtokens.WithAudience("billing-service"),
		authtokens.WithRevocationCheck(revocationList),
	)

	// Read validator: additionally enforces billing:read scope.
	readValidator := authtokens.NewValidator(
		authtokens.WithSecret(secret),
		authtokens.WithAudience("billing-service"),
		authtokens.WithRevocationCheck(revocationList),
		authtokens.WithRequiredScopes("billing:read"),
	)

	// Write validator: additionally enforces billing:write scope.
	writeValidator := authtokens.NewValidator(
		authtokens.WithSecret(secret),
		authtokens.WithAudience("billing-service"),
		authtokens.WithRevocationCheck(revocationList),
		authtokens.WithRequiredScopes("billing:write"),
	)

	cache := cachex.Memory(
		cachex.WithDefaultTTL(5*time.Minute),
		cachex.WithMaxSize(1000),
	)

	billingStore := store.New()
	subHandler := handler.NewSubscription(billingStore, cache, logger)
	usageHandler := handler.NewUsage(billingStore, logger)
	invoiceHandler := handler.NewInvoice(billingStore, logger)
	authHandler := handler.NewAuth(issuer, baseValidator, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(logpipe.Middleware(logger))
	r.Use(middleware.Recoverer)

	// Public — no auth required.
	r.Get("/health", handler.Health())

	// Auth routes — validates token (signature + expiry + revocation) but no billing scopes.
	r.Group(func(r chi.Router) {
		r.Use(authtokens.Middleware(baseValidator))
		r.Post("/auth/refresh", authHandler.Refresh())
	})

	// Read routes — require billing:read scope.
	r.Group(func(r chi.Router) {
		r.Use(authtokens.Middleware(readValidator))
		r.Get("/subscriptions", subHandler.List())
		r.Get("/subscriptions/{id}", subHandler.Get())
		r.Get("/usage", usageHandler.List())
		r.Get("/invoices/{id}", invoiceHandler.Get())
		r.Get("/invoices", invoiceHandler.List())
	})

	// Write routes — require billing:write scope.
	r.Group(func(r chi.Router) {
		r.Use(authtokens.Middleware(writeValidator))
		r.Post("/subscriptions", subHandler.Create())
		r.Post("/subscriptions/{id}/cancel", subHandler.Cancel())
		r.Post("/usage", usageHandler.Record())
		r.Post("/invoices/generate", invoiceHandler.Generate())
	})

	logger.Info("starting billing-service", logpipe.String("port", cfg.Port))
	http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), r)
}
