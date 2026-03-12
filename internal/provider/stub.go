package provider

import (
	"context"
	"crypto/rand"
	"fmt"
)

func init() {
	Register("stub", func(cfg map[string]string) (PaymentProvider, error) {
		return NewStub(), nil
	})
}

// StubProvider is an in-memory, no-op payment provider used for development
// and testing. It satisfies the PaymentProvider interface without making any
// external calls.
type StubProvider struct{}

// NewStub creates a new StubProvider.
func NewStub() *StubProvider {
	return &StubProvider{}
}

func (s *StubProvider) CreateCustomer(_ context.Context, req CreateCustomerRequest) (*Customer, error) {
	return &Customer{
		ProviderID: "stub_cus_" + stubID(),
	}, nil
}

func (s *StubProvider) Charge(_ context.Context, req ChargeRequest) (*PaymentResult, error) {
	if req.Amount <= 0 {
		return nil, &ProviderError{
			Code:    "invalid_amount",
			Message: "charge amount must be positive",
		}
	}
	return &PaymentResult{
		ChargeID: "stub_ch_" + stubID(),
		Status:   "succeeded",
	}, nil
}

func (s *StubProvider) Refund(_ context.Context, req RefundRequest) (*RefundResult, error) {
	if req.ChargeID == "" {
		return nil, &ProviderError{
			Code:    "missing_charge_id",
			Message: "charge_id is required for refund",
		}
	}
	return &RefundResult{
		RefundID: "stub_re_" + stubID(),
		Status:   "succeeded",
	}, nil
}

func (s *StubProvider) CreateSubscription(_ context.Context, req CreateSubscriptionRequest) (*ProviderSubscription, error) {
	return &ProviderSubscription{
		ProviderID: "stub_sub_" + stubID(),
		Status:     "active",
	}, nil
}

func (s *StubProvider) CancelSubscription(_ context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return &ProviderError{
			Code:    "missing_subscription_id",
			Message: "subscription_id is required",
		}
	}
	return nil
}

func (s *StubProvider) GetPaymentStatus(_ context.Context, chargeID string) (*PaymentResult, error) {
	if chargeID == "" {
		return nil, &ProviderError{
			Code:    "missing_charge_id",
			Message: "charge_id is required",
		}
	}
	return &PaymentResult{
		ChargeID: chargeID,
		Status:   "succeeded",
	}, nil
}

// stubID generates a short random hex string for deterministic-looking IDs.
func stubID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
