package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/helix-acme-corp-demo/authtokens"
	"github.com/helix-acme-corp-demo/logpipe"
)

func TestAuthRefresh_ValidToken(t *testing.T) {
	secret := []byte("test-secret-key")
	issuer := authtokens.NewIssuer(
		authtokens.WithSecret(secret),
		authtokens.WithDefaultTTL(1*time.Hour),
	)
	validator := authtokens.NewValidator(authtokens.WithSecret(secret))
	logger := logpipe.New()

	original, err := issuer.Issue(authtokens.Claims{
		Subject:   "user:42",
		IssuedAt:  time.Now().Add(-30 * time.Minute),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	h := NewAuth(issuer, validator, logger)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+original.Raw)
	rec := httptest.NewRecorder()

	h.Refresh().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["token"] == "" {
		t.Error("response should contain a non-empty token")
	}
	if body["token"] == original.Raw {
		t.Error("refreshed token should differ from the original")
	}
}

func TestAuthRefresh_MissingHeader(t *testing.T) {
	secret := []byte("test-secret-key")
	issuer := authtokens.NewIssuer(authtokens.WithSecret(secret))
	validator := authtokens.NewValidator(authtokens.WithSecret(secret))
	logger := logpipe.New()

	h := NewAuth(issuer, validator, logger)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rec := httptest.NewRecorder()

	h.Refresh().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] == "" {
		t.Error("response should contain an error message")
	}
}

func TestAuthRefresh_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key")
	issuer := authtokens.NewIssuer(
		authtokens.WithSecret(secret),
		authtokens.WithDefaultTTL(1*time.Hour),
	)
	validator := authtokens.NewValidator(authtokens.WithSecret(secret))
	logger := logpipe.New()

	expired, err := issuer.Issue(authtokens.Claims{
		Subject:   "user:42",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	h := NewAuth(issuer, validator, logger)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+expired.Raw)
	rec := httptest.NewRecorder()

	h.Refresh().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] == "" {
		t.Error("response should contain an error message")
	}
}

func TestAuthRefresh_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key")
	issuer := authtokens.NewIssuer(authtokens.WithSecret(secret))
	validator := authtokens.NewValidator(authtokens.WithSecret(secret))
	logger := logpipe.New()

	h := NewAuth(issuer, validator, logger)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	rec := httptest.NewRecorder()

	h.Refresh().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRefresh_PreservesSubjectAndExtra(t *testing.T) {
	secret := []byte("test-secret-key")
	issuer := authtokens.NewIssuer(
		authtokens.WithSecret(secret),
		authtokens.WithDefaultTTL(1*time.Hour),
	)
	validator := authtokens.NewValidator(authtokens.WithSecret(secret))
	logger := logpipe.New()

	original, err := issuer.Issue(authtokens.Claims{
		Subject:   "user:99",
		Audience:  "billing-service",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Extra:     map[string]string{"scopes": "billing:read billing:write"},
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	h := NewAuth(issuer, validator, logger)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+original.Raw)
	rec := httptest.NewRecorder()

	h.Refresh().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	newToken := body["token"]
	if newToken == "" {
		t.Fatal("expected token in response")
	}

	// Validate the refreshed token and check claims are preserved.
	claims, err := validator.Validate(newToken)
	if err != nil {
		t.Fatalf("Validate() on refreshed token error = %v", err)
	}
	if claims.Subject != "user:99" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user:99")
	}
	if claims.Extra["scopes"] != "billing:read billing:write" {
		t.Errorf("scopes = %q, want %q", claims.Extra["scopes"], "billing:read billing:write")
	}
}
