package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/helix-acme-corp-demo/authtokens"
	"github.com/helix-acme-corp-demo/logpipe"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	issuer    authtokens.Issuer
	validator authtokens.Validator
	logger    logpipe.Logger
}

// NewAuth creates a new AuthHandler.
func NewAuth(issuer authtokens.Issuer, validator authtokens.Validator, logger logpipe.Logger) *AuthHandler {
	return &AuthHandler{
		issuer:    issuer,
		validator: validator,
		logger:    logger,
	}
}

// Refresh returns an HTTP handler that issues a fresh token from a valid existing one.
// The request must carry a valid Bearer token in the Authorization header.
// On success it returns {"token": "<raw>"} with 200 OK.
func (h *AuthHandler) Refresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization token not provided"})
			return
		}

		raw := strings.TrimPrefix(header, "Bearer ")

		refreshed, err := h.issuer.Refresh(raw, h.validator)
		if err != nil {
			h.logger.Info("token refresh failed", logpipe.String("error", err.Error()))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		h.logger.Info("token refreshed", logpipe.String("subject", refreshed.Claims.Subject))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"token": refreshed.Raw})
	}
}
