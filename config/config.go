package config

import "os"

// Config holds the application configuration values.
type Config struct {
	Port            string
	HelixPayBaseURL string
	HelixPayAPIKey  string
}

// Load returns the application configuration.
func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	helixPayBaseURL := os.Getenv("HELIXPAY_BASE_URL")
	if helixPayBaseURL == "" {
		helixPayBaseURL = "https://api.helixpay.io"
	}

	return Config{
		Port:            port,
		HelixPayBaseURL: helixPayBaseURL,
		HelixPayAPIKey:  os.Getenv("HELIXPAY_API_KEY"),
	}
}
