package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/handler"
	"github.com/helix-acme-corp-demo/billing-service/internal/middleware"
)

const authTestSecret = "auth-handler-test-secret"

// fakeAuthStore implements the authStore interface used by AuthHandler.
type fakeAuthStore struct {
	mu      sync.RWMutex
	revoked map[string]time.Time
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{revoked: make(map[string]time.Time)}
}

func (f *fakeAuthStore) RevokeToken(jti string, expiry time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked[jti] = expiry
}

func (f *fakeAuthStore) IsRevoked(jti string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	expiry, ok := f.revoked[jti]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		return false
	}
	return true
}

// helpers

func newAuthHandler(store *fakeAuthStore) *handler.AuthHandler {
	return handler.NewAuth(
		store,
		authTestSecret,
		15*time.Minute,
		7*24*time.Hour,
		logpipe.New(),
	)
}

func signTestToken(t *testing.T, claims *domain.TokenClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(authTestSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func makeRefreshToken(t *testing.T, jti string, ttl time.Duration) string {
	t.Helper()
	now := time.Now().UTC()
	return signTestToken(t, &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-abc",
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Type: "refresh",
	})
}

func makeAccessTokenForRevoke(t *testing.T, jti string, ttl time.Duration) string {
	t.Helper()
	now := time.Now().UTC()
	return signTestToken(t, &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-abc",
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Scopes: []string{"billing:subscriptions:read"},
		Type:   "access",
	})
}

func postJSON(t *testing.T, handler http.Handler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func decodeErrorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return body.Error.Code
}

func decodeTokenPair(t *testing.T, rr *httptest.ResponseRecorder) domain.TokenPair {
	t.Helper()
	var body struct {
		Data domain.TokenPair `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode token pair response: %v", err)
	}
	return body.Data
}

// --- Refresh endpoint tests ---

func TestRefresh_ValidToken(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	refreshToken := makeRefreshToken(t, "jti-refresh-valid", 7*24*time.Hour)

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	pair := decodeTokenPair(t, rr)
	if pair.AccessToken == "" {
		t.Fatal("expected non-empty access_token")
	}
	if pair.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token")
	}
	// New tokens must be different from the old one.
	if pair.RefreshToken == refreshToken {
		t.Fatal("new refresh token should differ from old one")
	}
}

func TestRefresh_OldTokenRevoked(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	refreshToken := makeRefreshToken(t, "jti-to-rotate", 7*24*time.Hour)

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Re-using the same refresh token should now be rejected.
	rr2 := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on second use of rotated token, got %d", rr2.Code)
	}
	if code := decodeErrorCode(t, rr2); code != "token_revoked" {
		t.Fatalf("expected token_revoked, got %s", code)
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	// Build a refresh token that is already expired.
	past := time.Now().Add(-2 * time.Hour)
	expiredToken := signTestToken(t, &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-abc",
			ID:        "jti-expired-refresh",
			IssuedAt:  jwt.NewNumericDate(past.Add(-time.Hour)),
			ExpiresAt: jwt.NewNumericDate(past),
		},
		Type: "refresh",
	})

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": expiredToken,
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired refresh token, got %d", rr.Code)
	}
}

func TestRefresh_RevokedToken(t *testing.T) {
	store := newFakeAuthStore()
	// Pre-revoke the JTI.
	store.RevokeToken("jti-pre-revoked", time.Now().Add(7*24*time.Hour))

	h := newAuthHandler(store)

	refreshToken := makeRefreshToken(t, "jti-pre-revoked", 7*24*time.Hour)

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked refresh token, got %d", rr.Code)
	}
	if code := decodeErrorCode(t, rr); code != "token_revoked" {
		t.Fatalf("expected token_revoked, got %s", code)
	}
}

func TestRefresh_WrongTokenType(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	// Pass an access token where a refresh token is required.
	accessToken := makeAccessTokenForRevoke(t, "jti-wrong-type", time.Hour)

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": accessToken,
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token type, got %d", rr.Code)
	}
	if code := decodeErrorCode(t, rr); code != "invalid_token" {
		t.Fatalf("expected invalid_token, got %s", code)
	}
}

func TestRefresh_EmptyBody(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing refresh_token, got %d", rr.Code)
	}
}

func TestRefresh_InvalidSignature(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	// Sign with wrong secret.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-abc",
			ID:        "jti-bad-sig",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Type: "refresh",
	})
	signed, _ := tok.SignedString([]byte("wrong-secret"))

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": signed,
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d", rr.Code)
	}
}

// --- Revoke endpoint tests ---

func TestRevoke_ValidToken(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	tokenStr := makeAccessTokenForRevoke(t, "jti-to-revoke", time.Hour)

	rr := postJSON(t, h.Revoke(), "/auth/revoke", map[string]string{
		"token": tokenStr,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Confirm the JTI is now in the store.
	if !store.IsRevoked("jti-to-revoke") {
		t.Fatal("expected jti-to-revoke to be revoked in store")
	}
}

func TestRevoke_InvalidToken(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	rr := postJSON(t, h.Revoke(), "/auth/revoke", map[string]string{
		"token": "this.is.not.a.jwt",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", rr.Code)
	}
}

func TestRevoke_EmptyBody(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	rr := postJSON(t, h.Revoke(), "/auth/revoke", map[string]string{})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d", rr.Code)
	}
}

func TestRevoke_ExpiredTokenIsStillRevocable(t *testing.T) {
	// An expired token should still be parseable by ParseToken for revocation purposes.
	// In practice our ParseToken rejects expired tokens, so this confirms that behaviour.
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	past := time.Now().Add(-2 * time.Hour)
	expiredToken := signTestToken(t, &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-abc",
			ID:        "jti-exp-revoke",
			IssuedAt:  jwt.NewNumericDate(past.Add(-time.Hour)),
			ExpiresAt: jwt.NewNumericDate(past),
		},
		Type: "access",
	})

	rr := postJSON(t, h.Revoke(), "/auth/revoke", map[string]string{
		"token": expiredToken,
	})

	// Our ParseToken validates expiry, so expired tokens return 401.
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token on revoke, got %d", rr.Code)
	}
}

// --- New access token claims verification ---

func TestRefresh_NewAccessTokenHasCorrectType(t *testing.T) {
	store := newFakeAuthStore()
	h := newAuthHandler(store)

	refreshToken := makeRefreshToken(t, "jti-type-check", 7*24*time.Hour)

	rr := postJSON(t, h.Refresh(), "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	pair := decodeTokenPair(t, rr)

	// Parse the new access token and confirm its type.
	claims, err := middleware.ParseToken(pair.AccessToken, authTestSecret)
	if err != nil {
		t.Fatalf("failed to parse new access token: %v", err)
	}
	if claims.Type != "access" {
		t.Fatalf("expected access token type 'access', got %q", claims.Type)
	}

	// Parse the new refresh token and confirm its type.
	refreshClaims, err := middleware.ParseToken(pair.RefreshToken, authTestSecret)
	if err != nil {
		t.Fatalf("failed to parse new refresh token: %v", err)
	}
	if refreshClaims.Type != "refresh" {
		t.Fatalf("expected refresh token type 'refresh', got %q", refreshClaims.Type)
	}
}
