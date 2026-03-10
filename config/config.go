package config

import (
	"os"
	"strings"
	"time"
)

// Config holds the application configuration values.
type Config struct {
	Port            string
	JWTSecret       string
	JWTDefaultTTL   time.Duration
	RevokedTokenIDs []string
}

// Load returns the application configuration.
func Load() Config {
	ttl := 1 * time.Hour
	if raw := os.Getenv("JWT_DEFAULT_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			ttl = d
		}
	}

	var revokedIDs []string
	if raw := os.Getenv("REVOKED_TOKEN_IDS"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				revokedIDs = append(revokedIDs, id)
			}
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	return Config{
		Port:            port,
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTDefaultTTL:   ttl,
		RevokedTokenIDs: revokedIDs,
	}
}
