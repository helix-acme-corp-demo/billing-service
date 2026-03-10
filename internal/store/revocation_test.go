package store_test

import (
	"testing"
	"time"

	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

func TestIsRevoked_NotRevoked(t *testing.T) {
	s := store.New()
	if s.IsRevoked("jti-unknown") {
		t.Fatal("expected jti-unknown to not be revoked")
	}
}

func TestIsRevoked_Revoked(t *testing.T) {
	s := store.New()
	expiry := time.Now().Add(time.Hour)
	s.RevokeToken("jti-active", expiry)

	if !s.IsRevoked("jti-active") {
		t.Fatal("expected jti-active to be revoked")
	}
}

func TestIsRevoked_LazyPruneExpired(t *testing.T) {
	s := store.New()
	// Add a JTI that already expired.
	pastExpiry := time.Now().Add(-time.Second)
	s.RevokeToken("jti-expired", pastExpiry)

	// IsRevoked should return false and silently prune the entry.
	if s.IsRevoked("jti-expired") {
		t.Fatal("expected expired jti to not be considered revoked")
	}

	// Calling again should still return false (entry was pruned).
	if s.IsRevoked("jti-expired") {
		t.Fatal("expected pruned jti to remain not revoked on second call")
	}
}

func TestIsRevoked_MultipleTokens(t *testing.T) {
	s := store.New()

	s.RevokeToken("jti-a", time.Now().Add(time.Hour))
	s.RevokeToken("jti-b", time.Now().Add(time.Hour))
	s.RevokeToken("jti-c", time.Now().Add(-time.Second)) // already expired

	if !s.IsRevoked("jti-a") {
		t.Fatal("expected jti-a to be revoked")
	}
	if !s.IsRevoked("jti-b") {
		t.Fatal("expected jti-b to be revoked")
	}
	// jti-c has expired — should be pruned and return false.
	if s.IsRevoked("jti-c") {
		t.Fatal("expected jti-c (expired) to not be revoked")
	}
	if s.IsRevoked("jti-d") {
		t.Fatal("expected unknown jti-d to not be revoked")
	}
}

func TestRevokeToken_OverwriteExpiry(t *testing.T) {
	s := store.New()

	// Revoke with a past expiry first (so IsRevoked returns false / prunes it).
	s.RevokeToken("jti-overwrite", time.Now().Add(-time.Second))
	if s.IsRevoked("jti-overwrite") {
		t.Fatal("expected jti-overwrite (expired) to not be revoked before overwrite")
	}

	// Revoke again with a future expiry.
	s.RevokeToken("jti-overwrite", time.Now().Add(time.Hour))
	if !s.IsRevoked("jti-overwrite") {
		t.Fatal("expected jti-overwrite to be revoked after re-adding with future expiry")
	}
}
