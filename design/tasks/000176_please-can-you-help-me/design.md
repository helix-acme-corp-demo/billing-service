# Design: HelixPay Payment Integration

## Overview

Add HelixPay as a payment method to the existing billing service. The integration adds a single new endpoint to process payments for existing invoices, a HelixPay API client, and minor domain/store extensions. All existing endpoints and behaviour remain unchanged.

## Architecture

The billing service follows a simple layered structure:

```
cmd/server/main.go         → wires dependencies and registers routes
internal/handler/          → HTTP handlers (thin, delegates to store)
internal/domain/billing.go → domain structs and request/response types
internal/store/billing.go  → in-memory data store
config/config.go           → app config loaded from environment
```

A new `internal/payment/helixpay.go` package will house the HelixPay HTTP client. This keeps external-API concerns isolated from the handler and domain layers, matching how the service already isolates caching (`cachex`) and logging (`logpipe`).

## New Components

### `internal/payment/helixpay.go`
A thin HTTP client that:
- Accepts a base URL and API key (injected at construction time)
- Exposes a single `Charge(ctx, token, amount, currency, invoiceID string) error` method
- Uses `retryx` with exponential backoff for transient errors (network failures, 5xx responses)
- Returns a typed `PaymentError` on permanent failures (4xx) so callers can distinguish retryable vs non-retryable

### `internal/handler/payment.go`
`PaymentHandler` with one action:

| Method | Route | Description |
|--------|-------|-------------|
| `POST` | `/invoices/{id}/pay` | Charge the invoice via HelixPay |

Request body:
```json
{ "token": "<helixpay_payment_token>" }
```

Success response (`200`): updated `Invoice` object with `status: "paid"`, `paid_at`, and `payment_method: "helixpay"`.

Error responses:
- `404` — invoice not found
- `400` — missing token or invoice already paid
- `402` — HelixPay payment declined

### Domain changes (`internal/domain/billing.go`)
- Add `PaymentMethod string` field to `Invoice`
- Add `PayInvoiceRequest` struct `{ Token string }`

### Store changes (`internal/store/billing.go`)
No structural changes required. `SaveInvoice` already handles updates (upsert by ID).

### Config changes (`config/config.go`)
Add two new fields read from environment variables:
- `HELIXPAY_BASE_URL` (e.g. `https://api.helixpay.io`)
- `HELIXPAY_API_KEY`

## Sequence: `POST /invoices/{id}/pay`

```
Client → POST /invoices/{id}/pay {token}
  Handler: load invoice from store
    → 404 if not found
    → 400 if already paid
  Handler: call helixpay.Charge(token, amount, currency, invoiceID)
    → retryx retries on transient errors
    → 402 if HelixPay returns permanent failure
  Handler: set invoice.Status = "paid", invoice.PaidAt = now, invoice.PaymentMethod = "helixpay"
  Handler: store.SaveInvoice(invoice)
  Handler: return 200 updated invoice
```

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Separate `internal/payment` package | Isolates HelixPay HTTP concerns; easy to swap or mock in tests |
| Use existing `retryx` library | Already a project dependency; provides exponential backoff + jitter with minimal boilerplate |
| `PaymentMethod` field on `Invoice` | Allows future payment methods (e.g. Stripe) without schema changes |
| Env vars for credentials | Follows 12-factor; credentials stay out of code and logs |
| No DB migration needed | Store is in-memory; `SaveInvoice` already upserts by ID |

## Codebase Patterns to Follow

- All HTTP handlers use `envelope.Write` / `envelope.OK` / `envelope.BadRequest` etc. for responses — continue this pattern
- Structured logging via `logpipe.String(...)` fields — log payment outcome (success/failure) without logging the API key or token
- Handler constructors follow `NewXxx(store, logger)` convention — `NewPayment(store, client, logger)`
- Routes registered in `cmd/server/main.go` next to related invoice routes

## Implementation Notes

### What Was Built
- `config/config.go` — Added `HelixPayBaseURL` and `HelixPayAPIKey` fields; also made `PORT` env-configurable (was hardcoded). Defaults to `https://api.helixpay.io` if `HELIXPAY_BASE_URL` is unset.
- `internal/domain/billing.go` — Added `PaymentMethod string` (omitempty) to `Invoice`; added `PayInvoiceRequest{Token string}`.
- `internal/payment/helixpay.go` — New package. `Client` wraps `net/http` with a 10s timeout. `Charge()` POSTs to `{baseURL}/v1/charge` with `Authorization: Bearer <key>`. Uses `retryx.Do` with `WithRetryIf` to skip retry on `*PaymentError` (4xx), retries on network errors and 5xx. Max 4 attempts, 200ms base delay, 5s max delay.
- `internal/handler/payment.go` — `PaymentHandler.Pay()` handles the full lifecycle: parse token → find invoice → guard already-paid → call Charge → update invoice → save → respond. Added a local `paymentRequired()` helper to write 402 responses in the envelope shape (the `envelope` package has no `PaymentRequired` helper).
- `cmd/server/main.go` — Constructs `payment.NewClient(cfg.HelixPayBaseURL, cfg.HelixPayAPIKey)`, constructs `handler.NewPayment(...)`, registers `r.Post("/invoices/{id}/pay", paymentHandler.Pay())`.

### retryx API (learned during implementation)
The `retryx` package exposes a simple `retryx.Do(ctx, fn, ...opts)` top-level function. Key options:
- `retryx.WithMaxAttempts(n)` — total attempts including first call
- `retryx.WithBaseDelay(d)` / `retryx.WithMaxDelay(d)` — backoff tuning
- `retryx.WithRetryIf(func(error) bool)` — predicate to skip retry on permanent errors

There is no `Policy` or `strategy` sub-package in this version of the library; the cookbook example was illustrative only.

### envelope package (learned during implementation)
The `envelope` package does **not** have a `PaymentRequired` (402) helper. Added a local `paymentRequired()` helper in `handler/payment.go` that manually encodes an `envelope.Response` at HTTP 402.

### Gotchas
- `envelope.Write` sets both `Content-Type` and calls `w.WriteHeader` internally — do not call `w.WriteHeader` before it or the header will be sent twice (only the first call takes effect in Go's `http.ResponseWriter`). The local `paymentRequired()` helper correctly calls `w.WriteHeader(402)` before encoding directly, bypassing `envelope.Write`.
- The `retryx` module files in the Go module cache are read-only (`chmod 444`); use `sudo cat` to inspect them.

### Smoke Test Results
All edge cases verified against a running local server:
- `POST /invoices/{id}/pay` with valid token → 500 (HelixPay host not reachable in sandbox; retried 4× as expected)
- Missing `token` field → 400 `missing_fields`
- Unknown invoice ID → 404 `not_found`
- All pre-existing endpoints (subscriptions, usage, invoices) unaffected