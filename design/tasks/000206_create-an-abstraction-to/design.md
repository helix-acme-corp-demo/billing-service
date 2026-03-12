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

## Implementation Notes

### What was built

New package `internal/provider/` with 5 files:
- `provider.go` — `PaymentProvider` interface (6 methods), all request/response types, and `ProviderError` type
- `stub.go` — `StubProvider` that returns fake IDs with `stub_` prefixes and always succeeds (except validation errors)
- `stripe.go` — `StripeProvider` skeleton where every method returns a `not_implemented` error; validates API key at construction
- `registry.go` — Global registry with `Register()` and `NewFromConfig(name, cfg)` factory; both providers self-register via `init()`
- `provider_test.go` — 16 unit tests covering stub behavior, registry resolution, unknown provider errors, and `ProviderError` semantics

### Files modified

| File | Change |
|------|--------|
| `internal/domain/billing.go` | Added `ProviderCustomerID`, `ProviderSubscriptionID` to `Subscription`; added `ProviderChargeID` to `Invoice` |
| `config/config.go` | Added `PaymentProvider` and `ProviderConfig` fields; reads `PAYMENT_PROVIDER`, `STRIPE_API_KEY` from env |
| `internal/handler/subscription.go` | `NewSubscription` now takes a `provider.PaymentProvider`; `Create()` calls `CreateCustomer` + `CreateSubscription` (best-effort); `Cancel()` calls `CancelSubscription` (best-effort) |
| `internal/handler/invoice.go` | `NewInvoice` now takes a `provider.PaymentProvider`; `Generate()` calls `Charge` when amount > 0; stores `ProviderChargeID` on invoice |
| `cmd/server/main.go` | Uses `provider.NewFromConfig()` to resolve provider at startup; passes it to handlers; fatals on unknown provider |
| `README.md` | Added "Payment Providers" section with config instructions and "Adding a New Provider" guide |

### Patterns used

- **Self-registering providers via `init()`** — Each provider file registers itself in the global registry. This means adding a new provider is a single-file addition with no changes to the registry or main.go (just import the package if it's in a separate module).
- **Best-effort provider calls in handlers** — Provider errors are logged but don't block the core operation (subscription creation, invoice saving). This prevents a flaky provider from making the entire service unusable.
- **Config via `map[string]string`** — The registry constructor receives a generic config map, so each provider can pull whatever keys it needs without a shared config struct.
- **Compile-time interface checks** — `var _ PaymentProvider = (*StubProvider)(nil)` in the test file ensures both implementations satisfy the interface.

### Decisions made during implementation

- **Did NOT create a separate `internal/domain/payment.go`** — The provider package owns its own request/response types as designed. The domain types only needed 3 new fields (`ProviderCustomerID`, `ProviderSubscriptionID`, `ProviderChargeID`) added to existing structs. A separate domain file would have been redundant.
- **Kept `*store.Store` as a concrete type in handlers** — Converting the store to an interface was out of scope and would have been a larger refactor. Handlers now accept both `*store.Store` and `provider.PaymentProvider`.
- **Config uses `os.Getenv` directly** — The original `config.Load()` returned hardcoded values. Updated it to read from environment variables with the same defaults, which is more flexible.
- **Pricing maps stay in `handler/invoice.go`** — Originally planned to move `planPrices` and `usageCosts` into the stub provider, but they're used for local amount calculation before calling `Charge`. The provider receives the computed amount, not raw plan/usage data. Moving them would have complicated the interface.

### Gotchas

- The `logpipe` logger uses `logpipe.String()` for structured fields but the `Error` method is undocumented — it follows the same pattern as `Info` (verified by reading the handler code).
- Provider constructor in the registry receives `map[string]string` which can be `nil` — the stub handles this fine, but the Stripe constructor accesses `cfg["api_key"]` which returns empty string on nil map (Go's zero value), triggering the validation error correctly.
- The feature branch was created locally from `main` since it didn't exist on the remote yet.