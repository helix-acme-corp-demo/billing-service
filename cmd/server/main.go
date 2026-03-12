package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/helix-acme-corp-demo/cachex"
	"github.com/helix-acme-corp-demo/logpipe"
	_ "github.com/helix-acme-corp-demo/retryx"

	"github.com/helix-acme-corp-demo/billing-service/config"
	"github.com/helix-acme-corp-demo/billing-service/internal/handler"
	"github.com/helix-acme-corp-demo/billing-service/internal/provider"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

func main() {
	cfg := config.Load()
	logger := logpipe.New()

	// Resolve the configured payment provider.
	pp, err := provider.NewFromConfig(cfg.PaymentProvider, cfg.ProviderConfig)
	if err != nil {
		log.Fatalf("failed to initialize payment provider %q: %v", cfg.PaymentProvider, err)
	}
	logger.Info("payment provider initialized", logpipe.String("provider", cfg.PaymentProvider))

	cache := cachex.Memory(
		cachex.WithDefaultTTL(5*time.Minute),
		cachex.WithMaxSize(1000),
	)

	billingStore := store.New()
	subHandler := handler.NewSubscription(billingStore, pp, cache, logger)
	usageHandler := handler.NewUsage(billingStore, logger)
	invoiceHandler := handler.NewInvoice(billingStore, pp, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(logpipe.Middleware(logger))
	r.Use(middleware.Recoverer)

	r.Get("/health", handler.Health())

	r.Post("/subscriptions", subHandler.Create())
	r.Get("/subscriptions", subHandler.List())
	r.Get("/subscriptions/{id}", subHandler.Get())
	r.Post("/subscriptions/{id}/cancel", subHandler.Cancel())

	r.Post("/usage", usageHandler.Record())
	r.Get("/usage", usageHandler.List())

	r.Post("/invoices/generate", invoiceHandler.Generate())
	r.Get("/invoices/{id}", invoiceHandler.Get())
	r.Get("/invoices", invoiceHandler.List())

	logger.Info("starting billing-service", logpipe.String("port", cfg.Port))
	http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), r)
}
