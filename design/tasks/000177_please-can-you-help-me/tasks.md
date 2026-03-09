# Implementation Tasks

## Configuration

- [x] Extend `config/config.go` to read `HELIXPAY_API_KEY`, `HELIXPAY_MERCHANT_ID`, `HELIXPAY_WEBHOOK_SECRET`, and `HELIXPAY_ENV` from environment variables
- [x] Fail fast at startup (log fatal) if `HELIXPAY_API_KEY` or `HELIXPAY_MERCHANT_ID` are empty

## Domain & Store

- [x] Add `HelixPayChargeID string` and `PaymentMethod string` fields to `domain.Invoice` in `internal/domain/billing.go`
- [x] Add `FindInvoiceByChargeID(chargeID string) (*domain.Invoice, bool)` method to `internal/store/billing.go`

## HelixPay Client

- [~] Add `github.com/helix-acme-corp-demo/helix-pay-go` to `go.mod` / `go.sum`
- [~] Initialise `helixpay.Client` in `cmd/server/main.go` using `helixpay.Dial` with `WithEnvironment`, `WithMerchantID`, and `WithWebhookSecret` options drawn from config

## Payment Handler (`internal/handler/payment.go`)

- [ ] Create `PaymentHandler` struct with `store`, `helixClient`, and `logger` fields
- [ ] Implement `POST /invoices/{id}/pay` — decode request body accepting either `customer_id` or `email`+`name`
- [ ] If `email`+`name` provided, call `helixClient.Customers.Register` to obtain a HelixPay customer ID
- [ ] Return `400` if neither `customer_id` nor `email` is supplied
- [ ] Return `404` if invoice does not exist
- [ ] Return `400` with code `invoice_not_payable` if invoice status is not `draft`
- [ ] Build charge request using `charges.NewBuilder` with a deterministic idempotency key via `idempotency.FromComponents("invoice", invoiceID)`
- [ ] Add invoice ID as charge metadata (`"invoice_id"`)
- [ ] Call `helixClient.Charges.Initiate` and handle API errors (return `502` on failure)
- [ ] Set invoice `Status` to `pending_payment`, `HelixPayChargeID` to the returned charge ID, and `PaymentMethod` to `"helixpay"`; persist via `store.SaveInvoice`
- [ ] Return `202 Accepted` with updated invoice via `envelope.Write`

## Webhook Handler (`internal/handler/webhook.go`)

- [ ] Create `NewHelixPayWebhookHandler(store, helixClient, logger)` that returns an `http.Handler` using `webhooks.NewListener`
- [ ] Register handler for `webhooks.ChargeSettled`: look up invoice by charge ID, set status to `paid` and `paid_at` to now, persist
- [ ] Register handler for `webhooks.ChargeDeclined`: look up invoice by charge ID, set status back to `draft`, persist
- [ ] Register handler for `webhooks.ChargeVoided`: look up invoice by charge ID, set status back to `draft`, persist
- [ ] Log each status transition with invoice ID and charge ID
- [ ] Unhandled event types are left to the SDK default (respond `200 OK`, no action)

## Routing

- [ ] Register `POST /invoices/{id}/pay` in `cmd/server/main.go` using the new `PaymentHandler`
- [ ] Mount webhook handler at `POST /webhooks/helixpay` in `cmd/server/main.go`
