package domain

import "time"

// Subscription represents a user's subscription to a billing plan.
type Subscription struct {
	ID                     string    `json:"id"`
	UserID                 string    `json:"user_id"`
	Plan                   string    `json:"plan"`
	Status                 string    `json:"status"`
	CreatedAt              time.Time `json:"created_at"`
	ExpiresAt              time.Time `json:"expires_at"`
	ProviderCustomerID     string    `json:"provider_customer_id,omitempty"`
	ProviderSubscriptionID string    `json:"provider_subscription_id,omitempty"`
}

// UsageRecord represents a single metered usage event.
type UsageRecord struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	Metric         string    `json:"metric"`
	Quantity       int64     `json:"quantity"`
	RecordedAt     time.Time `json:"recorded_at"`
}

// Invoice represents a billing invoice for a subscription.
type Invoice struct {
	ID               string     `json:"id"`
	SubscriptionID   string     `json:"subscription_id"`
	Amount           int64      `json:"amount"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	IssuedAt         time.Time  `json:"issued_at"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	ProviderChargeID string     `json:"provider_charge_id,omitempty"`
}

// CreateSubscriptionRequest is the payload for creating a subscription.
type CreateSubscriptionRequest struct {
	UserID string `json:"user_id"`
	Plan   string `json:"plan"`
}

// RecordUsageRequest is the payload for recording a usage event.
type RecordUsageRequest struct {
	SubscriptionID string `json:"subscription_id"`
	Metric         string `json:"metric"`
	Quantity       int64  `json:"quantity"`
}

// GenerateInvoiceRequest is the payload for generating an invoice.
type GenerateInvoiceRequest struct {
	SubscriptionID string `json:"subscription_id"`
}
