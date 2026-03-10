package auth

import "sync"

// RevocationList is an in-memory implementation of authtokens.RevocationChecker.
// It stores revoked JWT IDs (jti claims) and can be queried or updated concurrently.
type RevocationList struct {
	mu      sync.RWMutex
	revoked map[string]struct{}
}

// NewRevocationList creates a RevocationList pre-populated with the given token IDs.
func NewRevocationList(ids []string) *RevocationList {
	r := &RevocationList{
		revoked: make(map[string]struct{}, len(ids)),
	}
	for _, id := range ids {
		if id != "" {
			r.revoked[id] = struct{}{}
		}
	}
	return r
}

// IsRevoked reports whether the token with the given ID has been revoked.
// It satisfies the authtokens.RevocationChecker interface.
func (r *RevocationList) IsRevoked(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.revoked[id]
	return ok
}

// Revoke adds the given token ID to the revocation list.
func (r *RevocationList) Revoke(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked[id] = struct{}{}
}
