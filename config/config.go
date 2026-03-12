package config

import "os"

// Config holds the application configuration values.
type Config struct {
	Port            string
	PaymentProvider string
	ProviderConfig  map[string]string
}

// Load returns the application configuration.
func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	provider := os.Getenv("PAYMENT_PROVIDER")
	if provider == "" {
		provider = "stub"
	}

	providerCfg := map[string]string{}

	// Stripe configuration (only relevant when PAYMENT_PROVIDER=stripe).
	if key := os.Getenv("STRIPE_API_KEY"); key != "" {
		providerCfg["api_key"] = key
	}

	return Config{
		Port:            port,
		PaymentProvider: provider,
		ProviderConfig:  providerCfg,
	}
}
