package config

import "os"

// Config holds the application configuration values.
type Config struct {
	Port string

	// HelixPay credentials
	HelixPayAPIKey        string
	HelixPayMerchantID    string
	HelixPayWebhookSecret string
	HelixPayEnv           string
}

// Load returns the application configuration.
func Load() Config {
	return Config{
		Port:                  getEnv("PORT", "8082"),
		HelixPayAPIKey:        os.Getenv("HELIXPAY_API_KEY"),
		HelixPayMerchantID:    os.Getenv("HELIXPAY_MERCHANT_ID"),
		HelixPayWebhookSecret: os.Getenv("HELIXPAY_WEBHOOK_SECRET"),
		HelixPayEnv:           getEnv("HELIXPAY_ENV", "sandbox"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
