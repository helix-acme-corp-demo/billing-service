package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
	"github.com/helix-acme-corp-demo/billing-service/internal/middleware"
)

const testSecret = "test-secret-key"

// fakeStore implements the revocationStore interface for tests.
type fakeStore struct {
	revoked map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{revoked: make(map[string]bool)}
}

func (f *fakeStore) IsRevoked(jti string) bool {
	return f.revoked[jti]
}

// okHandler is a simple handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
})

// makeToken creates a signed token with the given claims for testing.
func makeToken(t *testing.T, claims *domain.TokenClaims, secret string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func makeAccessToken(t *testing.T, jti string, scopes []string, ttl time.Duration) string {
	t.Helper()
	now := time.Now().UTC()
	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Scopes: scopes,
		Type:   "access",
	}
	return makeToken(t, claims, testSecret)
}

func responseCode(rr *httptest.ResponseRecorder) int {
	return rr.Code
}

func responseErrorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return body.Error.Code
}

// --- Authenticate middleware tests ---

func TestAuthenticate_MissingHeader(t *testing.T) {
	store := newFakeStore()
	logger := logpipe.New()
	handler := middleware.Authenticate(store, testSecret, logger)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
	if code := responseErrorCode(t, rr); code != "invalid_token" {
		t.Fatalf("expected error code invalid_token, got %s", code)
	}
}

func TestAuthenticate_MalformedHeader(t *testing.T) {
	store := newFakeStore()
	logger := logpipe.New()
	handler := middleware.Authenticate(store, testSecret, logger)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer xyz")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
	if code := responseErrorCode(t, rr); code != "invalid_token" {
		t.Fatalf("expected error code invalid_token, got %s", code)
	}
}

func TestAuthenticate_InvalidSignature(t *testing.T) {
	store := newFakeStore()
	logger := logpipe.New()
	handler := middleware.Authenticate(store, testSecret, logger)(okHandler)

	// Sign with a different secret.
	tokenStr := makeAccessToken(t, "jti-1", []string{"billing:subscriptions:read"}, time.Hour)
	// Tamper: replace the handler's secret expectation by passing wrong secret token
	// Re-sign with a different secret to produce a bad signature.
	wrongToken := makeToken(t, &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ID:        "jti-x",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Type: "access",
	}, "wrong-secret")
	_ = tokenStr // suppress unused

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+wrongToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
	if code := responseErrorCode(t, rr); code != "invalid_token" {
		t.Fatalf("expected error code invalid_token, got %s", code)
	}
}

func TestAuthenticate_ExpiredToken(t *testing.T) {
	store := newFakeStore()
	logger := logpipe.New()
	handler := middleware.Authenticate(store, testSecret, logger)(okHandler)

	// Create a token that expired in the past.
	past := time.Now().Add(-2 * time.Hour)
	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ID:        "jti-exp",
			IssuedAt:  jwt.NewNumericDate(past.Add(-time.Hour)),
			ExpiresAt: jwt.NewNumericDate(past),
		},
		Type: "access",
	}
	tokenStr := makeToken(t, claims, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
	if code := responseErrorCode(t, rr); code != "token_expired" {
		t.Fatalf("expected error code token_expired, got %s", code)
	}
}

func TestAuthenticate_RevokedToken(t *testing.T) {
	store := newFakeStore()
	store.revoked["jti-revoked"] = true
	logger := logpipe.New()
	handler := middleware.Authenticate(store, testSecret, logger)(okHandler)

	tokenStr := makeAccessToken(t, "jti-revoked", []string{"billing:subscriptions:read"}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
	if code := responseErrorCode(t, rr); code != "token_revoked" {
		t.Fatalf("expected error code token_revoked, got %s", code)
	}
}

func TestAuthenticate_ValidToken(t *testing.T) {
	store := newFakeStore()
	logger := logpipe.New()

	var capturedClaims *domain.TokenClaims
	capturingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			t.Error("expected claims in context but got none")
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(store, testSecret, logger)(capturingHandler)

	tokenStr := makeAccessToken(t, "jti-valid", []string{"billing:subscriptions:read"}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
	if capturedClaims == nil {
		t.Fatal("expected claims to be injected into context")
	}
	if capturedClaims.ID != "jti-valid" {
		t.Fatalf("expected jti jti-valid, got %s", capturedClaims.ID)
	}
}

func TestAuthenticate_BearerCaseInsensitive(t *testing.T) {
	store := newFakeStore()
	logger := logpipe.New()
	handler := middleware.Authenticate(store, testSecret, logger)(okHandler)

	tokenStr := makeAccessToken(t, "jti-ci", []string{"billing:subscriptions:read"}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "BEARER "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusOK {
		t.Fatalf("expected 200 for BEARER (case-insensitive), got %d", got)
	}
}

// --- ClaimsFromContext tests ---

func TestClaimsFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	_, ok := middleware.ClaimsFromContext(ctx)
	if ok {
		t.Fatal("expected no claims in empty context")
	}
}

// --- RequireScope middleware tests ---

func makeScopeRequest(t *testing.T, scopes []string) *http.Request {
	t.Helper()
	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ID:        "jti-scope",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Scopes: scopes,
		Type:   "access",
	}
	ctx := context.WithValue(context.Background(), contextKeyForTest(), claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	return req.WithContext(ctx)
}

// contextKeyForTest injects claims directly via the exported helper for scope tests.
// We use the round-trip through Authenticate to avoid depending on the internal key.
func injectClaims(r *http.Request, claims *domain.TokenClaims, secret string) *http.Request {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(secret))
	r.Header.Set("Authorization", "Bearer "+signed)
	return r
}

func contextKeyForTest() struct{ name string } {
	// This helper exists only to document that we can't reach the unexported key
	// from outside the package; we use the Authenticate middleware to inject.
	return struct{ name string }{"unused"}
}

func runWithAuth(t *testing.T, scopes []string, requiredScope string) *httptest.ResponseRecorder {
	t.Helper()
	store := newFakeStore()
	logger := logpipe.New()

	// Chain: Authenticate -> RequireScope -> okHandler
	chain := middleware.Authenticate(store, testSecret, logger)(
		middleware.RequireScope(requiredScope)(okHandler),
	)

	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ID:        "jti-sc",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Scopes: scopes,
		Type:   "access",
	}
	tokenStr := makeToken(t, claims, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)
	return rr
}

func TestRequireScope_ScopePresent(t *testing.T) {
	rr := runWithAuth(t, []string{"billing:subscriptions:read", "billing:usage:read"}, "billing:subscriptions:read")
	if got := responseCode(rr); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
}

func TestRequireScope_ScopeMissing(t *testing.T) {
	rr := runWithAuth(t, []string{"billing:usage:read"}, "billing:subscriptions:read")
	if got := responseCode(rr); got != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", got)
	}
	if code := responseErrorCode(t, rr); code != "insufficient_scope" {
		t.Fatalf("expected error code insufficient_scope, got %s", code)
	}
}

func TestRequireScope_EmptyScopes(t *testing.T) {
	rr := runWithAuth(t, []string{}, "billing:subscriptions:write")
	if got := responseCode(rr); got != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", got)
	}
}

func TestRequireScope_NoClaims(t *testing.T) {
	// Call RequireScope without running Authenticate first — no claims in context.
	handler := middleware.RequireScope("billing:subscriptions:read")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := responseCode(rr); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no claims in context, got %d", got)
	}
}
