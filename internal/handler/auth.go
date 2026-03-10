package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/middleware"
)

// authStore is the subset of store.Store used by AuthHandler.
type authStore interface {
	RevokeToken(jti string, expiry time.Time)
	IsRevoked(jti string) bool
}

// AuthHandler handles token refresh and revocation.
type AuthHandler struct {
	store      authStore
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
	logger     logpipe.Logger
}

// NewAuth creates a new AuthHandler.
func NewAuth(s authStore, jwtSecret string, accessTTL, refreshTTL time.Duration, l logpipe.Logger) *AuthHandler {
	return &AuthHandler{
		store:      s,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		logger:     l,
	}
}

// Refresh handles POST /auth/refresh — validates a refresh token and issues a new token pair.
func (h *AuthHandler) Refresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			envelope.Write(w, envelope.BadRequest("invalid_body", "refresh_token is required"))
			return
		}

		claims, err := middleware.ParseToken(req.RefreshToken, h.jwtSecret)
		if err != nil {
			envelope.Write(w, authUnauthorized("invalid_token", "refresh token is invalid or expired"))
			return
		}

		if claims.Type != "refresh" {
			envelope.Write(w, authUnauthorized("invalid_token", "token is not a refresh token"))
			return
		}

		jti := claims.ID
		if jti != "" && h.store.IsRevoked(jti) {
			envelope.Write(w, authUnauthorized("token_revoked", "refresh token has been revoked"))
			return
		}

		// Revoke old refresh token.
		if jti != "" {
			expiry, _ := claims.GetExpirationTime()
			if expiry != nil {
				h.store.RevokeToken(jti, expiry.Time)
			}
		}

		pair, err := h.issueTokenPair(claims.Subject)
		if err != nil {
			h.logger.Info("failed to sign token pair", logpipe.String("error", err.Error()))
			envelope.Write(w, envelope.Response{
				Status: http.StatusInternalServerError,
				OK:     false,
				Error:  &envelope.ErrorDetail{Code: "internal_error", Message: "could not issue tokens"},
			})
			return
		}

		h.logger.Info("token pair issued via refresh", logpipe.String("sub", claims.Subject))
		envelope.Write(w, envelope.OK(pair))
	}
}

// Revoke handles POST /auth/revoke — adds the provided token's jti to the revocation list.
func (h *AuthHandler) Revoke() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.RevokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			envelope.Write(w, envelope.BadRequest("invalid_body", "token is required"))
			return
		}

		claims, err := middleware.ParseToken(req.Token, h.jwtSecret)
		if err != nil {
			envelope.Write(w, authUnauthorized("invalid_token", "token is invalid or expired"))
			return
		}

		jti := claims.ID
		if jti == "" {
			envelope.Write(w, envelope.BadRequest("invalid_token", "token has no jti claim"))
			return
		}

		expiry, _ := claims.GetExpirationTime()
		if expiry != nil {
			h.store.RevokeToken(jti, expiry.Time)
		} else {
			// No expiry — revoke with a far-future time so it stays in the list.
			h.store.RevokeToken(jti, time.Now().Add(h.refreshTTL))
		}

		h.logger.Info("token revoked", logpipe.String("jti", jti))
		envelope.Write(w, envelope.OK(map[string]string{"status": "revoked"}))
	}
}

// issueTokenPair creates and signs a fresh access + refresh token pair for the given subject.
func (h *AuthHandler) issueTokenPair(subject string) (*domain.TokenPair, error) {
	now := time.Now().UTC()

	accessClaims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ID:        generateUUID(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.accessTTL)),
		},
		Type: "access",
	}
	accessToken, err := middleware.SignToken(accessClaims, h.jwtSecret)
	if err != nil {
		return nil, err
	}

	refreshClaims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ID:        generateUUID(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.refreshTTL)),
		},
		Type: "refresh",
	}
	refreshToken, err := middleware.SignToken(refreshClaims, h.jwtSecret)
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// authUnauthorized builds a 401 envelope.Response with a specific code.
func authUnauthorized(code, message string) envelope.Response {
	return envelope.Response{
		Status: http.StatusUnauthorized,
		OK:     false,
		Error:  &envelope.ErrorDetail{Code: code, Message: message},
	}
}
