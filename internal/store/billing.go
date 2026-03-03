package store

import (
	"sync"

	"github.com/helix-acme-corp-demo/billing-service/internal/domain"
)

// Store is an in-memory store for billing data.
type Store struct {
	mu            sync.RWMutex
	subscriptions map[string]*domain.Subscription
	usageRecords  map[string]*domain.UsageRecord
	invoices      map[string]*domain.Invoice
}

// New creates a new empty Store.
func New() *Store {
	return &Store{
		subscriptions: make(map[string]*domain.Subscription),
		usageRecords:  make(map[string]*domain.UsageRecord),
		invoices:      make(map[string]*domain.Invoice),
	}
}

// SaveSubscription persists a subscription.
func (s *Store) SaveSubscription(sub *domain.Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[sub.ID] = sub
}

// FindSubscription returns a subscription by ID.
func (s *Store) FindSubscription(id string) (*domain.Subscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subscriptions[id]
	return sub, ok
}

// SubscriptionsByUser returns all subscriptions for a given user.
func (s *Store) SubscriptionsByUser(userID string) []*domain.Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Subscription
	for _, sub := range s.subscriptions {
		if sub.UserID == userID {
			result = append(result, sub)
		}
	}
	return result
}

// AllSubscriptions returns every subscription in the store.
func (s *Store) AllSubscriptions() []*domain.Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		result = append(result, sub)
	}
	return result
}

// SaveUsage persists a usage record.
func (s *Store) SaveUsage(u *domain.UsageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usageRecords[u.ID] = u
}

// UsageBySubscription returns all usage records for a subscription.
func (s *Store) UsageBySubscription(subID string) []*domain.UsageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.UsageRecord
	for _, u := range s.usageRecords {
		if u.SubscriptionID == subID {
			result = append(result, u)
		}
	}
	return result
}

// SaveInvoice persists an invoice.
func (s *Store) SaveInvoice(i *domain.Invoice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invoices[i.ID] = i
}

// FindInvoice returns an invoice by ID.
func (s *Store) FindInvoice(id string) (*domain.Invoice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.invoices[id]
	return inv, ok
}

// InvoicesBySubscription returns all invoices for a subscription.
func (s *Store) InvoicesBySubscription(subID string) []*domain.Invoice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Invoice
	for _, inv := range s.invoices {
		if inv.SubscriptionID == subID {
			result = append(result, inv)
		}
	}
	return result
}
