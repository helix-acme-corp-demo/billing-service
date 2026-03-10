package auth

import (
	"testing"
)

func TestNewRevocationList_Empty(t *testing.T) {
	rl := NewRevocationList(nil)
	if rl.IsRevoked("any-id") {
		t.Error("empty list should not report any ID as revoked")
	}
}

func TestNewRevocationList_PrePopulated(t *testing.T) {
	rl := NewRevocationList([]string{"tok-1", "tok-2"})

	if !rl.IsRevoked("tok-1") {
		t.Error("tok-1 should be revoked")
	}
	if !rl.IsRevoked("tok-2") {
		t.Error("tok-2 should be revoked")
	}
	if rl.IsRevoked("tok-3") {
		t.Error("tok-3 should not be revoked")
	}
}

func TestNewRevocationList_SkipsEmptyStrings(t *testing.T) {
	rl := NewRevocationList([]string{"", "tok-1", ""})
	if rl.IsRevoked("") {
		t.Error("empty string should not be treated as a revoked ID")
	}
	if !rl.IsRevoked("tok-1") {
		t.Error("tok-1 should be revoked")
	}
}

func TestRevoke_AddsID(t *testing.T) {
	rl := NewRevocationList(nil)

	if rl.IsRevoked("new-tok") {
		t.Fatal("new-tok should not be revoked before Revoke() is called")
	}

	rl.Revoke("new-tok")

	if !rl.IsRevoked("new-tok") {
		t.Error("new-tok should be revoked after Revoke() is called")
	}
}

func TestRevoke_DoesNotAffectOthers(t *testing.T) {
	rl := NewRevocationList([]string{"tok-existing"})
	rl.Revoke("tok-new")

	if !rl.IsRevoked("tok-existing") {
		t.Error("tok-existing should still be revoked")
	}
	if !rl.IsRevoked("tok-new") {
		t.Error("tok-new should be revoked")
	}
	if rl.IsRevoked("tok-unrelated") {
		t.Error("tok-unrelated should not be revoked")
	}
}

func TestIsRevoked_NonRevoked(t *testing.T) {
	rl := NewRevocationList([]string{"tok-a", "tok-b"})
	if rl.IsRevoked("tok-c") {
		t.Error("tok-c should not be revoked")
	}
}
