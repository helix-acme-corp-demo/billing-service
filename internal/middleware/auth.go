package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helix-acme-corp-demo/envelope"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
)

// contextKey is an unexported type to avoid context key collisions.
type contextKey struct{}

// claimsKey is the key used to store TokenClaims in a request context.
var claimsKey = contextKey{}

// revocationStore is the subset of store.Store used by the middleware.
type revocationStore interface {
	IsRevoked(jti string) bool
}

// ClaimsFromContext retrieves the TokenClaims stored in the given context.
// Returns false if no claims are present.
func ClaimsFromContext(ctx context.Context) (*domain.TokenClaims, bool) {
	claims, ok := ctx.Value(claimsKey).(*domain.TokenClaims)
	return claims, ok
}

// authError builds an envelope.Response for authentication/authorisation failures.
func authError(status int, code, message string) envelope.Response {
	return envelope.Response{
		Status: status,
		OK:     false,
		Error: &envelope.ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

// Authenticate returns a chi-compatible middleware that validates the JWT
// in the Authorization header, checks the revocation list, and injects the
// parsed claims into the request context.
func Authenticate(store revocationStore, jwtSecret string, logger logpipe.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				envelope.Write(w, authError(http.StatusUnauthorized, "invalid_token", "authorization header is required"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				envelope.Write(w, authError(http.StatusUnauthorized, "invalid_token", "authorization header must be Bearer <token>"))
				return
			}

			tokenStr := parts[1]
			claims, err := ParseToken(tokenStr, jwtSecret)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					logger.Info("rejected expired token")
					envelope.Write(w, authError(http.StatusUnauthorized, "token_expired", "token has expired"))
					return
				}
				logger.Info("rejected invalid token", logpipe.String("error", err.Error()))
				envelope.Write(w, authError(http.StatusUnauthorized, "invalid_token", "token is invalid"))
				return
			}

			jti := claims.ID // jwt.RegisteredClaims.ID == "jti"
			if jti != "" && store.IsRevoked(jti) {
				logger.Info("rejected revoked token", logpipe.String("jti", jti))
				envelope.Write(w, authError(http.StatusUnauthorized, "token_revoked", "token has been revoked"))
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope returns a chi-compatible middleware that checks that the
// authenticated caller's token contains the specified scope.
// Authenticate must run before RequireScope in the middleware chain.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				envelope.Write(w, authError(http.StatusUnauthorized, "invalid_token", "no authentication claims found"))
				return
			}

			for _, s := range claims.Scopes {
				if s == scope {
					next.ServeHTTP(w, r)
					return
				}
			}

			envelope.Write(w, authError(http.StatusForbidden, "insufficient_scope", "token does not have required scope: "+scope))
		})
	}
}

// SignToken signs a TokenClaims struct using HS256 and returns the token string.
func SignToken(claims *domain.TokenClaims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken parses and validates a JWT string, returning the embedded TokenClaims.
func ParseToken(tokenStr, secret string) (*domain.TokenClaims, error) {
	claims := &domain.TokenClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithLeeway(0*time.Second))
	if err != nil {
		return nil, err
	}
	return claims, nil
}
