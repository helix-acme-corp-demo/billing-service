package provider

import (
	"context"
	"fmt"
)

func init() {
	Register("stripe", func(cfg map[string]string) (PaymentProvider, error) {
		return NewStripe(cfg["api_key"])
	})
}

// StripeProvider is a payment provider implementation backed by the Stripe API.
// This is a skeleton — each method returns an error indicating that the real
// Stripe SDK integration is not yet implemented. Replace the TODOs with actual
// stripe-go SDK calls when ready.
type StripeProvider struct {
	APIKey string
}

// NewStripe creates a new StripeProvider. It returns an error if the API key is
// empty so that misconfiguration is caught at startup.
func NewStripe(apiKey string) (*StripeProvider, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Code:    "missing_api_key",
			Message: "stripe API key is required",
		}
	}
	return &StripeProvider{APIKey: apiKey}, nil
}

func (s *StripeProvider) CreateCustomer(_ context.Context, req CreateCustomerRequest) (*Customer, error) {
	// TODO: call stripe customer.New(&stripe.CustomerParams{...})
	return nil, notImplemented("CreateCustomer")
}

func (s *StripeProvider) Charge(_ context.Context, req ChargeRequest) (*PaymentResult, error) {
	// TODO: call stripe paymentintent.New(&stripe.PaymentIntentParams{...})
	return nil, notImplemented("Charge")
}

func (s *StripeProvider) Refund(_ context.Context, req RefundRequest) (*RefundResult, error) {
	// TODO: call stripe refund.New(&stripe.RefundParams{...})
	return nil, notImplemented("Refund")
}

func (s *StripeProvider) CreateSubscription(_ context.Context, req CreateSubscriptionRequest) (*ProviderSubscription, error) {
	// TODO: call stripe subscription.New(&stripe.SubscriptionParams{...})
	return nil, notImplemented("CreateSubscription")
}

func (s *StripeProvider) CancelSubscription(_ context.Context, subscriptionID string) error {
	// TODO: call stripe subscription.Cancel(subscriptionID, nil)
	return notImplemented("CancelSubscription")
}

func (s *StripeProvider) GetPaymentStatus(_ context.Context, chargeID string) (*PaymentResult, error) {
	// TODO: call stripe paymentintent.Get(chargeID, nil)
	return nil, notImplemented("GetPaymentStatus")
}

func notImplemented(method string) *ProviderError {
	return &ProviderError{
		Code:    "not_implemented",
		Message: fmt.Sprintf("stripe: %s is not yet implemented", method),
	}
}
