package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/helix-acme-corp-demo/authtokens"
	"github.com/helix-acme-corp-demo/cachex"
	"github.com/helix-acme-corp-demo/logpipe"

	"github.com/helix-acme-corp-demo/billing-service/internal/auth"
	"github.com/helix-acme-corp-demo/billing-service/internal/handler"
	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// buildTestRouter mirrors the main() router setup using the provided validators.
func buildTestRouter(
	issuer authtokens.Issuer,
	baseValidator authtokens.Validator,
	readValidator authtokens.Validator,
	writeValidator authtokens.Validator,
) http.Handler {
	logger := logpipe.New()

	cache := cachex.Memory(
		cachex.WithDefaultTTL(5*time.Minute),
		cachex.WithMaxSize(1000),
	)

	billingStore := store.New()
	subHandler := handler.NewSubscription(billingStore, cache, logger)
	usageHandler := handler.NewUsage(billingStore, logger)
	invoiceHandler := handler.NewInvoice(billingStore, logger)
	authHandler := handler.NewAuth(issuer, baseValidator, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Public
	r.Get("/health", handler.Health())

	// Auth routes — base validation only, no billing scopes
	r.Group(func(r chi.Router) {
		r.Use(authtokens.Middleware(baseValidator))
		r.Post("/auth/refresh", authHandler.Refresh())
	})

	// Read routes — require billing:read
	r.Group(func(r chi.Router) {
		r.Use(authtokens.Middleware(readValidator))
		r.Get("/subscriptions", subHandler.List())
		r.Get("/subscriptions/{id}", subHandler.Get())
		r.Get("/usage", usageHandler.List())
		r.Get("/invoices/{id}", invoiceHandler.Get())
		r.Get("/invoices", invoiceHandler.List())
	})

	// Write routes — require billing:write
	r.Group(func(r chi.Router) {
		r.Use(authtokens.Middleware(writeValidator))
		r.Post("/subscriptions", subHandler.Create())
		r.Post("/subscriptions/{id}/cancel", subHandler.Cancel())
		r.Post("/usage", usageHandler.Record())
		r.Post("/invoices/generate", invoiceHandler.Generate())
	})

	return r
}

func setupTestServer(t *testing.T) (http.Handler, authtokens.Issuer) {
	t.Helper()

	secret := []byte("integration-test-secret")
	revocationList := auth.NewRevocationList(nil)

	issuer := authtokens.NewIssuer(
		authtokens.WithSecret(secret),
		authtokens.WithDefaultTTL(1*time.Hour),
		authtokens.WithAudience("billing-service"),
	)

	baseValidator := authtokens.NewValidator(
		authtokens.WithSecret(secret),
		authtokens.WithAudience("billing-service"),
		authtokens.WithRevocationCheck(revocationList),
	)
	readValidator := authtokens.NewValidator(
		authtokens.WithSecret(secret),
		authtokens.WithAudience("billing-service"),
		authtokens.WithRevocationCheck(revocationList),
		authtokens.WithRequiredScopes("billing:read"),
	)
	writeValidator := authtokens.NewValidator(
		authtokens.WithSecret(secret),
		authtokens.WithAudience("billing-service"),
		authtokens.WithRevocationCheck(revocationList),
		authtokens.WithRequiredScopes("billing:write"),
	)

	router := buildTestRouter(issuer, baseValidator, readValidator, writeValidator)
	return router, issuer
}

func issueToken(t *testing.T, issuer authtokens.Issuer, scopes string) string {
	t.Helper()
	token, err := issuer.Issue(authtokens.Claims{
		Subject:   "user:test",
		Audience:  "billing-service",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Extra:     map[string]string{"scopes": scopes},
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return token.Raw
}

// --- Health ---

func TestIntegration_Health_NoAuth(t *testing.T) {
	router, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- Read routes ---

func TestIntegration_GetSubscriptions_WithReadScope_Succeeds(t *testing.T) {
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:read")

	req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /subscriptions with billing:read status = %d, want %d; body: %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestIntegration_GetSubscriptions_WithWriteScopeOnly_Fails(t *testing.T) {
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:write")

	req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /subscriptions with billing:write-only status = %d, want %d; body: %s",
			rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestIntegration_GetSubscriptions_NoToken_Fails(t *testing.T) {
	router, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /subscriptions without token status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestIntegration_GetSubscriptions_WithBothScopes_Succeeds(t *testing.T) {
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:read billing:write")

	req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /subscriptions with both scopes status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- Write routes ---

func TestIntegration_PostSubscriptions_WithWriteScope_Succeeds(t *testing.T) {
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:write")

	body := `{"user_id":"u1","plan":"pro"}`
	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// 201 Created or 200 — either is acceptable for a successful create
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Errorf("POST /subscriptions with billing:write status = %d, want 200 or 201; body: %s",
			rec.Code, rec.Body.String())
	}
}

func TestIntegration_PostSubscriptions_WithReadScopeOnly_Fails(t *testing.T) {
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:read")

	body := `{"user_id":"u1","plan":"pro"}`
	req := httptest.NewRequest(http.MethodPost, "/subscriptions",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /subscriptions with billing:read-only status = %d, want %d",
			rec.Code, http.StatusUnauthorized)
	}
}

// --- Auth refresh ---

func TestIntegration_Refresh_ValidToken_ReturnsNewToken(t *testing.T) {
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:read")

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /auth/refresh status = %d, want %d; body: %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}

	var respBody map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&respBody); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if respBody["token"] == "" {
		t.Error("refresh response should contain a non-empty token")
	}
}

func TestIntegration_Refresh_NoToken_Fails(t *testing.T) {
	router, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /auth/refresh without token status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestIntegration_Refresh_WriteOnlyScopeToken_Succeeds(t *testing.T) {
	// /auth/refresh does NOT require a billing scope — any valid token should work.
	router, issuer := setupTestServer(t)
	token := issueToken(t, issuer, "billing:write")

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /auth/refresh with write-only token status = %d, want %d; body: %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}
}
