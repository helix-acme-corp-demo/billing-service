package config

import (
	"log"
	"os"
	"time"
)

// Config holds the application configuration values.
type Config struct {
	Port            string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load returns the application configuration.
func Load() Config {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	return Config{
		Port:            "8082",
		JWTSecret:       secret,
		AccessTokenTTL:  parseDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: parseDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
	}
}

// parseDuration reads a duration from an env var, falling back to the provided default.
func parseDuration(key string, defaultVal time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultVal
	}
	return d
}
