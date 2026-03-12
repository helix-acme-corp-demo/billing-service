# Implementation Tasks

- [~] Define `PaymentProvider` interface in `internal/provider/provider.go` with methods: `CreateCustomer`, `Charge`, `Refund`, `CreateSubscription`, `CancelSubscription`, and `GetPaymentStatus`
- [~] Define provider-related domain types in `internal/domain/payment.go` (`PaymentRequest`, `PaymentResult`, `CustomerInfo`, `ProviderSubscription`, etc.)
- [ ] Implement a `StubProvider` in `internal/provider/stub.go` that satisfies the interface with in-memory/no-op behavior (useful for tests and local dev)
- [ ] Implement a `StripeProvider` in `internal/provider/stripe.go` as the first real provider example (can be a skeleton with TODOs for actual Stripe SDK calls)
- [ ] Create a provider registry/factory in `internal/provider/registry.go` that maps a provider name (from config) to its constructor
- [ ] Add `PaymentProvider` config field to `config/config.go` (e.g., `Provider string` defaulting to `"stub"`)
- [ ] Refactor `InvoiceHandler` to accept a `PaymentProvider` interface and call `Charge` when generating/finalizing an invoice instead of only saving locally
- [ ] Refactor `SubscriptionHandler` to accept a `PaymentProvider` interface and call `CreateSubscription`/`CancelSubscription` on create/cancel flows
- [ ] Wire the provider into `cmd/server/main.go` — use the registry to resolve the configured provider and inject it into handlers
- [ ] Add unit tests for `StubProvider` to verify it satisfies the `PaymentProvider` interface and returns expected results
- [ ] Add unit tests for the registry to verify correct provider resolution and error on unknown provider name
- [ ] Update `README.md` with instructions on selecting and configuring a payment provider