package provider

import "context"

// PaymentProvider defines the abstraction for interacting with an external
// payment gateway (e.g. Stripe, Braintree). Implementations must be safe for
// concurrent use.
type PaymentProvider interface {
	// CreateCustomer registers a new customer with the payment provider.
	CreateCustomer(ctx context.Context, req CreateCustomerRequest) (*Customer, error)

	// Charge creates a one-time charge against a customer.
	Charge(ctx context.Context, req ChargeRequest) (*PaymentResult, error)

	// Refund issues a full or partial refund for a previous charge.
	Refund(ctx context.Context, req RefundRequest) (*RefundResult, error)

	// CreateSubscription starts a recurring subscription on the provider side.
	CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*ProviderSubscription, error)

	// CancelSubscription cancels an active subscription on the provider side.
	CancelSubscription(ctx context.Context, subscriptionID string) error

	// GetPaymentStatus retrieves the current status of a charge.
	GetPaymentStatus(ctx context.Context, chargeID string) (*PaymentResult, error)
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// CreateCustomerRequest holds the data needed to create a customer on the
// payment provider.
type CreateCustomerRequest struct {
	UserID string
	Email  string
	Name   string
}

// ChargeRequest holds the data needed to create a charge.
type ChargeRequest struct {
	CustomerID string
	Amount     int64  // amount in smallest currency unit (e.g. cents)
	Currency   string // ISO-4217 code, e.g. "usd"
	InvoiceID  string // internal invoice ID for correlation
}

// RefundRequest holds the data needed to issue a refund.
type RefundRequest struct {
	ChargeID string
	Amount   int64 // 0 means full refund
}

// CreateSubscriptionRequest holds the data needed to create a subscription on
// the provider side.
type CreateSubscriptionRequest struct {
	CustomerID string
	Plan       string
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// Customer represents a customer record on the payment provider.
type Customer struct {
	ProviderID string // the ID assigned by the external provider
}

// PaymentResult represents the outcome of a charge or payment status query.
type PaymentResult struct {
	ChargeID string // provider-assigned charge ID
	Status   string // e.g. "succeeded", "pending", "failed"
}

// RefundResult represents the outcome of a refund request.
type RefundResult struct {
	RefundID string
	Status   string
}

// ProviderSubscription represents a subscription record on the payment
// provider.
type ProviderSubscription struct {
	ProviderID string // provider-assigned subscription ID
	Status     string
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ProviderError wraps errors returned by a payment provider so callers can
// distinguish transient failures from permanent ones.
type ProviderError struct {
	Code      string // machine-readable code, e.g. "card_declined"
	Message   string // human-readable message
	Transient bool   // true if the caller should retry
	Err       error  // underlying error, if any
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
