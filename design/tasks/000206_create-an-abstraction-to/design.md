# Design: Payment Provider Abstraction

## Context

The billing-service currently has no payment provider integration. Invoice generation, subscription management, and usage billing are handled entirely in-memory with hardcoded pricing logic inside `internal/handler/invoice.go`. To support real payment processing (e.g., Stripe, Braintree, or future providers), we need an abstraction layer that decouples the billing handlers from any specific provider.

### Codebase Observations

- **Go service** using `chi` router, `logpipe` logger, `cachex` cache, `envelope` HTTP responses.
- **Project layout:** `cmd/server/`, `config/`, `internal/domain/`, `internal/handler/`, `internal/store/`.
- **Handlers are tightly coupled** to `*store.Store` (concrete type). No interfaces are used for dependencies.
- **Hardcoded pricing** lives in `internal/handler/invoice.go` (`planPrices`, `usageCosts` maps).
- **No external payment calls** exist today — everything is in-memory.
- **Go 1.22**, dependencies: `chi/v5`, `cachex`, `envelope`, `logpipe`, `retryx`.

## Architecture

### New Package: `internal/provider`

Introduce a `PaymentProvider` interface in a new `internal/provider` package. This is the core abstraction.

```
internal/provider/
├── provider.go          # Interface definition + common types
├── stub.go              # In-memory stub (current behavior, for dev/test)
└── stripe.go            # Stripe implementation (example real provider)
```

### PaymentProvider Interface

```go
type PaymentProvider interface {
    CreateCustomer(ctx context.Context, req CreateCustomerRequest) (*Customer, error)
    CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*Subscription, error)
    CancelSubscription(ctx context.Context, subscriptionID string) error
    CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*Invoice, error)
    ChargeInvoice(ctx context.Context, invoiceID string) (*PaymentResult, error)
    RecordUsage(ctx context.Context, req RecordUsageRequest) error
}
```

This interface covers the five operations the billing-service currently performs or will need: customer management, subscription lifecycle, invoicing, payment collection, and metered usage reporting.

### Integration Points

1. **Handlers receive the interface, not a concrete type.** `SubscriptionHandler`, `InvoiceHandler`, and `UsageHandler` will accept a `provider.PaymentProvider` alongside the existing `store.Store`.
2. **Config selects the provider.** A new `PaymentProvider` field in `config.Config` (e.g., `"stub"`, `"stripe"`) determines which implementation is instantiated in `main.go`.
3. **Domain types stay.** Existing `domain.Subscription`, `domain.Invoice`, and `domain.UsageRecord` remain the internal models. The provider package defines its own request/response types and maps to/from domain types as needed.

### Flow Example: Create Subscription

```
HTTP Request
  → SubscriptionHandler.Create()
    → provider.CreateSubscription(ctx, req)  // delegate to provider
    → store.SaveSubscription(sub)            // persist locally
    → HTTP Response
```

### Stub Provider

The stub provider preserves today's behavior exactly — it generates UUIDs, applies hardcoded pricing, and returns success. This ensures zero regression when the abstraction is introduced. The hardcoded `planPrices` and `usageCosts` maps move from `handler/invoice.go` into `provider/stub.go`.

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| **Single interface, not per-operation interfaces** | The billing operations are cohesive; splitting would add complexity without benefit at this scale. |
| **Provider package owns its own request/response types** | Keeps the interface self-contained. Handlers map from `domain` types. Avoids coupling domain models to provider APIs. |
| **Stub as default, config-driven selection** | No breaking change to current behavior. New providers are opt-in via config. |
| **Provider errors wrapped in a `ProviderError` type** | Allows handlers to distinguish transient vs. permanent failures and return appropriate HTTP status codes. |
| **No webhook handling in this task** | Webhooks (e.g., Stripe webhook for async payment confirmation) are a follow-up concern. This task focuses on the outbound abstraction. |

## Constraints / Gotchas

- **Handlers use `*store.Store` directly** (concrete type, not an interface). This task should also make the store dependency injectable via an interface, or at minimum accept both store and provider.
- **`retryx` is imported but unused** (`_` import in `main.go`). It could be useful for wrapping provider calls with retries — worth considering but not required.
- **No tests exist today.** The stub provider should be straightforward to test. Provider implementations should be testable via the interface.
- **API keys for real providers** must come from environment variables or a secrets manager — never hardcoded. The `config.Config` struct will need fields for provider credentials.