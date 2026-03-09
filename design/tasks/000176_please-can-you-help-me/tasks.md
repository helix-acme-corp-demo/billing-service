# Implementation Tasks

- [ ] Add `HELIXPAY_BASE_URL` and `HELIXPAY_API_KEY` fields to `config/config.go`, loaded from environment variables
- [ ] Add `PaymentMethod string` field to the `Invoice` struct in `internal/domain/billing.go`
- [ ] Add `PayInvoiceRequest` struct (with `Token string` field) to `internal/domain/billing.go`
- [ ] Create `internal/payment/helixpay.go` with a `Client` struct and a `Charge(ctx, token, amount, currency, invoiceID string) error` method that calls the HelixPay API
- [ ] Use `retryx` with exponential backoff in `helixpay.Client.Charge` for transient errors (network failures, 5xx); return a permanent error for 4xx responses without retrying
- [ ] Create `internal/handler/payment.go` with a `PaymentHandler` and a `Pay()` handler for `POST /invoices/{id}/pay`
- [ ] In `Pay()`: load invoice by ID (404 if not found), reject if already paid (400), call `helixpay.Client.Charge`, update invoice status to `paid` with `paid_at` and `payment_method = "helixpay"`, save and return updated invoice
- [ ] Return `402 Payment Required` when HelixPay returns a permanent payment failure
- [ ] Log payment outcome (success/failure) using `logpipe` — do not log the API key or payment token
- [ ] Wire `PaymentHandler` in `cmd/server/main.go`: construct `helixpay.Client` with config values, register `POST /invoices/{id}/pay` route
- [ ] Verify all existing endpoints still work correctly after the changes