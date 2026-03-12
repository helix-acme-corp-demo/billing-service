package provider

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// StubProvider tests
// ---------------------------------------------------------------------------

func TestStubProvider_CreateCustomer(t *testing.T) {
	stub := NewStub()
	cust, err := stub.CreateCustomer(context.Background(), CreateCustomerRequest{
		UserID: "user-1",
		Email:  "user@example.com",
		Name:   "Test User",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cust.ProviderID == "" {
		t.Fatal("expected non-empty ProviderID")
	}
	if !strings.HasPrefix(cust.ProviderID, "stub_cus_") {
		t.Fatalf("expected ProviderID to start with stub_cus_, got %q", cust.ProviderID)
	}
}

func TestStubProvider_Charge_Success(t *testing.T) {
	stub := NewStub()
	result, err := stub.Charge(context.Background(), ChargeRequest{
		CustomerID: "cus_123",
		Amount:     5000,
		Currency:   "usd",
		InvoiceID:  "inv_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("expected status succeeded, got %q", result.Status)
	}
	if !strings.HasPrefix(result.ChargeID, "stub_ch_") {
		t.Fatalf("expected ChargeID to start with stub_ch_, got %q", result.ChargeID)
	}
}

func TestStubProvider_Charge_InvalidAmount(t *testing.T) {
	stub := NewStub()
	_, err := stub.Charge(context.Background(), ChargeRequest{
		CustomerID: "cus_123",
		Amount:     0,
		Currency:   "usd",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Code != "invalid_amount" {
		t.Fatalf("expected code invalid_amount, got %q", pe.Code)
	}
}

func TestStubProvider_Refund_Success(t *testing.T) {
	stub := NewStub()
	result, err := stub.Refund(context.Background(), RefundRequest{
		ChargeID: "ch_123",
		Amount:   1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("expected status succeeded, got %q", result.Status)
	}
	if !strings.HasPrefix(result.RefundID, "stub_re_") {
		t.Fatalf("expected RefundID to start with stub_re_, got %q", result.RefundID)
	}
}

func TestStubProvider_Refund_MissingChargeID(t *testing.T) {
	stub := NewStub()
	_, err := stub.Refund(context.Background(), RefundRequest{})
	if err == nil {
		t.Fatal("expected error for missing charge ID")
	}
}

func TestStubProvider_CreateSubscription(t *testing.T) {
	stub := NewStub()
	sub, err := stub.CreateSubscription(context.Background(), CreateSubscriptionRequest{
		CustomerID: "cus_123",
		Plan:       "pro",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Status != "active" {
		t.Fatalf("expected status active, got %q", sub.Status)
	}
	if !strings.HasPrefix(sub.ProviderID, "stub_sub_") {
		t.Fatalf("expected ProviderID to start with stub_sub_, got %q", sub.ProviderID)
	}
}

func TestStubProvider_CancelSubscription_Success(t *testing.T) {
	stub := NewStub()
	err := stub.CancelSubscription(context.Background(), "sub_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStubProvider_CancelSubscription_MissingID(t *testing.T) {
	stub := NewStub()
	err := stub.CancelSubscription(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty subscription ID")
	}
}

func TestStubProvider_GetPaymentStatus_Success(t *testing.T) {
	stub := NewStub()
	result, err := stub.GetPaymentStatus(context.Background(), "ch_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ChargeID != "ch_123" {
		t.Fatalf("expected ChargeID ch_123, got %q", result.ChargeID)
	}
	if result.Status != "succeeded" {
		t.Fatalf("expected status succeeded, got %q", result.Status)
	}
}

func TestStubProvider_GetPaymentStatus_MissingID(t *testing.T) {
	stub := NewStub()
	_, err := stub.GetPaymentStatus(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty charge ID")
	}
}

// Verify that StubProvider satisfies the interface at compile time.
var _ PaymentProvider = (*StubProvider)(nil)

// Verify that StripeProvider satisfies the interface at compile time.
var _ PaymentProvider = (*StripeProvider)(nil)

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestNewFromConfig_Stub(t *testing.T) {
	p, err := NewFromConfig("stub", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := p.(*StubProvider); !ok {
		t.Fatalf("expected *StubProvider, got %T", p)
	}
}

func TestNewFromConfig_Stripe_MissingKey(t *testing.T) {
	_, err := NewFromConfig("stripe", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing Stripe API key")
	}
}

func TestNewFromConfig_Stripe_WithKey(t *testing.T) {
	p, err := NewFromConfig("stripe", map[string]string{"api_key": "sk_test_123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*StripeProvider); !ok {
		t.Fatalf("expected *StripeProvider, got %T", p)
	}
}

func TestNewFromConfig_Unknown(t *testing.T) {
	_, err := NewFromConfig("bogus_provider", nil)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown payment provider") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ProviderError tests
// ---------------------------------------------------------------------------

func TestProviderError_Error(t *testing.T) {
	pe := &ProviderError{Code: "test", Message: "something went wrong"}
	if pe.Error() != "something went wrong" {
		t.Fatalf("unexpected error string: %q", pe.Error())
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	inner := &ProviderError{Code: "inner", Message: "root cause"}
	pe := &ProviderError{Code: "outer", Message: "wrapper", Err: inner}
	if pe.Unwrap() != inner {
		t.Fatal("Unwrap did not return inner error")
	}
	if !strings.Contains(pe.Error(), "root cause") {
		t.Fatalf("expected wrapped error in message, got %q", pe.Error())
	}
}
