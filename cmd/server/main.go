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

	helixpay "github.com/helix-acme-corp-demo/helix-pay-go"

	"github.com/helix-acme-corp-demo/billing-service/config"
	"github.com/helix-acme-corp-demo/billing-service/internal/handler"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

func main() {
	cfg := config.Load()
	logger := logpipe.New()

	// Fail fast if required HelixPay credentials are missing.
	if cfg.HelixPayAPIKey == "" {
		log.Fatal("HELIXPAY_API_KEY is required but not set")
	}
	if cfg.HelixPayMerchantID == "" {
		log.Fatal("HELIXPAY_MERCHANT_ID is required but not set")
	}

	// Resolve the HelixPay environment.
	hpEnv := helixpay.Sandbox
	if cfg.HelixPayEnv == "production" {
		hpEnv = helixpay.Production
	}

	// Initialise the HelixPay client.
	helixClient, err := helixpay.Dial(
		cfg.HelixPayAPIKey,
		helixpay.WithEnvironment(hpEnv),
		helixpay.WithMerchantID(cfg.HelixPayMerchantID),
		helixpay.WithWebhookSecret(cfg.HelixPayWebhookSecret),
	)
	if err != nil {
		log.Fatalf("failed to initialise HelixPay client: %v", err)
	}

	cache := cachex.Memory(
		cachex.WithDefaultTTL(5*time.Minute),
		cachex.WithMaxSize(1000),
	)

	billingStore := store.New()
	subHandler := handler.NewSubscription(billingStore, cache, logger)
	usageHandler := handler.NewUsage(billingStore, logger)
	invoiceHandler := handler.NewInvoice(billingStore, logger)
	paymentHandler := handler.NewPayment(billingStore, helixClient, logger)
	webhookHandler := handler.NewHelixPayWebhookHandler(billingStore, helixClient, logger)

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
	r.Post("/invoices/{id}/pay", paymentHandler.Pay())

	r.Mount("/webhooks/helixpay", webhookHandler)

	logger.Info("starting billing-service", logpipe.String("port", cfg.Port))
	http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), r)
}
