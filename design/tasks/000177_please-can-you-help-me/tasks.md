# Implementation Tasks

## Configuration

- [x] Extend `config/config.go` to read `HELIXPAY_API_KEY`, `HELIXPAY_MERCHANT_ID`, `HELIXPAY_WEBHOOK_SECRET`, and `HELIXPAY_ENV` from environment variables
- [x] Fail fast at startup (log fatal) if `HELIXPAY_API_KEY` or `HELIXPAY_MERCHANT_ID` are empty

## Domain & Store

- [x] Add `HelixPayChargeID string` and `PaymentMethod string` fields to `domain.Invoice` in `internal/domain/billing.go`
- [x] Add `FindInvoiceByChargeID(chargeID string) (*domain.Invoice, bool)` method to `internal/store/billing.go`

## HelixPay Client

- [x] Add `github.com/helix-acme-corp-demo/helix-pay-go` to `go.mod` / `go.sum`
- [x] Initialise `helixpay.Client` in `cmd/server/main.go` using `helixpay.Dial` with `WithEnvironment`, `WithMerchantID`, and `WithWebhookSecret` options drawn from config

## Payment Handler (`internal/handler/payment.go`)

- [x] Create `PaymentHandler` struct with `store`, `helixClient`, and `logger` fields
- [x] Implement `POST /invoices/{id}/pay` — decode request body accepting either `customer_id` or `email`+`name`
- [x] If `email`+`name` provided, call `helixClient.Customers.Register` to obtain a HelixPay customer ID
- [x] Return `400` if neither `customer_id` nor `email` is supplied
- [x] Return `404` if invoice does not exist
- [x] Return `400` with code `invoice_not_payable` if invoice status is not `draft`
- [x] Build charge request using `charges.NewBuilder` with a deterministic idempotency key via `idempotency.FromComponents("invoice", invoiceID)`
- [x] Add invoice ID as charge metadata (`"invoice_id"`)
- [x] Call `helixClient.Charges.Initiate` and handle API errors (return `500` on failure)
- [x] Set invoice `Status` to `pending_payment`, `HelixPayChargeID` to the returned charge ID, and `PaymentMethod` to `"helixpay"`; persist via `store.SaveInvoice`
- [x] Return `202 Accepted` with updated invoice via `envelope.Write`

## Webhook Handler (`internal/handler/webhook.go`)

- [x] Create `NewHelixPayWebhookHandler(store, helixClient, logger)` that returns an `http.Handler` using `webhooks.NewListener`
- [x] Register handler for `webhooks.ChargeSettled`: look up invoice by charge ID, set status to `paid` and `paid_at` to now, persist
- [x] Register handler for `webhooks.ChargeDeclined`: look up invoice by charge ID, set status back to `draft`, persist
- [x] Register handler for `webhooks.ChargeVoided`: look up invoice by charge ID, set status back to `draft`, persist
- [x] Log each status transition with invoice ID and charge ID
- [x] Unhandled event types are left to the SDK default (respond `200 OK`, no action)

## Routing

- [x] Register `POST /invoices/{id}/pay` in `cmd/server/main.go` using the new `PaymentHandler`
- [x] Mount webhook handler at `POST /webhooks/helixpay` in `cmd/server/main.go`

## Implementation Notes

- `envelope.Error(status, code, msg)` does not exist in this codebase — used `envelope.InternalError(msg)` (500) for HelixPay gateway failures instead of a custom 502
- The `envelope.Write` function sets its own status code, so for the 202 Accepted response on `/invoices/{id}/pay` we write the header and JSON manually to avoid a double-write conflict
- `HELIXPAY_ENV` defaults to `"sandbox"`; set to `"production"` for live traffic
- `idempotency.FromComponents("invoice", invoiceID)` generates a stable key — retrying the same invoice pay request within 24 hours will not create a duplicate charge in HelixPay
- Webhook signature verification is handled entirely by the `webhooks.NewListener` / `Verifier.Authenticate` in the SDK (HMAC-SHA256, 5-minute replay tolerance)
- `FindInvoiceByChargeID` does a linear scan of the in-memory map — acceptable for this in-memory store; a real DB would add an index on `helix_pay_charge_id`
