# Requirements: Payment Provider Abstraction

## Background

The billing-service currently has no integration with external payment providers. Invoice generation, subscription management, and pricing logic are all handled in-memory with hardcoded values in `internal/handler/invoice.go`. To support real payment processing (e.g., Stripe, Braintree, or future providers), the service needs a provider abstraction layer so that new payment providers can be added without modifying existing handler or business logic.

### Codebase Context

- **Language/Framework:** Go 1.22, chi router, logpipe logger, cachex cache, envelope HTTP responses
- **Project layout:** `cmd/server/`, `config/`, `internal/domain/`, `internal/handler/`, `internal/store/`
- **Current state:** Handlers use a concrete `*store.Store` (in-memory). No interfaces exist for billing operations. Plan prices and usage costs are hardcoded maps in `internal/handler/invoice.go`.

## User Stories

### 1. As a developer, I want a `PaymentProvider` interface so I can integrate any payment gateway without changing handler code.

**Acceptance Criteria:**
- A Go interface (`PaymentProvider`) exists in `internal/provider/` that defines methods for: creating a charge, refunding a charge, and creating a customer.
- Handlers call the interface, not a concrete implementation.
- At least one concrete implementation exists (a stub/no-op provider for development and testing).

### 2. As a developer, I want provider selection driven by configuration so I can switch providers without code changes.

**Acceptance Criteria:**
- `config.Config` includes a `PaymentProvider` field (e.g., `"stub"`, `"stripe"`).
- A factory function in `internal/provider/` returns the correct provider implementation based on the config value.
- An unknown provider name returns a clear error at startup.

### 3. As a developer, I want the invoice generation flow to use the payment provider to create real charges.

**Acceptance Criteria:**
- When an invoice is generated, the handler calls `PaymentProvider.CreateCharge` with the computed amount and currency.
- The provider's external charge ID is stored on the `Invoice` domain model.
- The stub provider returns a deterministic fake charge ID so existing tests keep passing.

### 4. As a developer, I want the subscription creation flow to optionally create a customer on the payment provider.

**Acceptance Criteria:**
- When a subscription is created, the handler calls `PaymentProvider.CreateCustomer` with the user ID.
- The provider's external customer ID is stored on the `Subscription` domain model.
- Creating a customer is a best-effort operation — a provider error is logged but does not block subscription creation.

## Out of Scope

- Implementing a real Stripe or Braintree provider (only the stub is required now).
- Webhook handling for provider callbacks.
- Recurring billing automation.
- Migrating the in-memory store to a database.