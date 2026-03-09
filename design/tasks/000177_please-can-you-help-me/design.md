# Design: HelixPay Payment Integration

## Overview

Add HelixPay as a payment method to the billing service by integrating the `helix-pay-go` SDK. The integration introduces a `PaymentHandler` for initiating charges, a webhook endpoint for async charge lifecycle events, and extends the `Invoice` domain type and in-memory store to track charge state.

---

## Architecture

### New Components

```
billing-service/
├── config/
│   └── config.go              # Extended: HELIXPAY_* env vars
├── internal/
│   ├── domain/
│   │   └── billing.go         # Extended: Invoice gains HelixPayChargeID, updated statuses
│   ├── handler/
│   │   ├── payment.go         # NEW: POST /invoices/{id}/pay
│   │   └── webhook.go         # NEW: POST /webhooks/helixpay
│   └── store/
│       └── billing.go         # Extended: FindInvoiceByChargeID lookup
└── cmd/
    └── server/
        └── main.go            # Extended: wire HelixPay client + new routes
```

### Dependencies

The `helix-pay-go` SDK (already exists as a sibling repo) is added to `go.mod`:

```
github.com/helix-acme-corp-demo/helix-pay-go
```

---

## Key Design Decisions

### 1. Charges are asynchronous — use webhooks for settlement

HelixPay's `Charges.Initiate` returns `202 Accepted` and processes asynchronously. The invoice is immediately set to `pending_payment` with the charge ID stored on it. Final state (`paid` / `draft`) is set only when the webhook fires (`charge.settled`, `charge.declined`, `charge.voided`). This is the correct model for the SDK.

### 2. Invoice status lifecycle

```
draft → pending_payment  (charge initiated)
pending_payment → paid   (charge.settled webhook)
pending_payment → draft  (charge.declined or charge.voided webhook — allows retry)
```

New status values added to `Invoice.Status`: `pending_payment`, `paid` (in addition to existing `draft`).

### 3. Idempotency keys are deterministic

Use `idempotency.FromComponents("invoice", invoiceID)` to generate a stable key per invoice. This means retrying a `POST /invoices/{id}/pay` for the same invoice will not create a duplicate charge in HelixPay (within the 24-hour window).

### 4. Webhook handler uses the SDK's `webhooks.Listener`

The `helix-pay-go` SDK provides `webhooks.NewListener(client.Webhooks)` which handles signature verification and event dispatch. Mount it at `POST /webhooks/helixpay`. Register handlers for `ChargeSettled`, `ChargeDeclined`, and `ChargeVoided`.

### 5. HelixPay customer lookup is caller-supplied or auto-registered

The `POST /invoices/{id}/pay` request body accepts either:
- `customer_id` — an existing HelixPay customer ID (`cust_...`), **or**
- `email` + `name` — used to call `Customers.Register` (idempotent by email) to obtain a customer ID.

Exactly one of these must be present; the handler returns `400` otherwise.

### 6. Configuration via environment variables

`config.Load()` is extended to read HelixPay credentials from the environment. The service panics at startup if `HELIXPAY_API_KEY` or `HELIXPAY_MERCHANT_ID` are empty, preventing silent misconfiguration.

---

## Data Model Changes

### `domain.Invoice` — new fields

| Field              | Type      | Notes                                      |
|--------------------|-----------|--------------------------------------------|
| `HelixPayChargeID` | `string`  | Set when charge is initiated; empty before |
| `PaymentMethod`    | `string`  | Set to `"helixpay"` on pay; empty before   |

### `store.Store` — new method

| Method                              | Purpose                                         |
|-------------------------------------|-------------------------------------------------|
| `FindInvoiceByChargeID(id string)`  | Webhook handler needs to look up invoice by HelixPay charge ID |

---

## API Changes

### `POST /invoices/{id}/pay`

**Request body:**
```json
{
  "customer_id": "cust_abc123"
}
```
or:
```json
{
  "email": "user@example.com",
  "name": "Alice Smith"
}
```

**Response `202 Accepted`:**
```json
{
  "data": {
    "id": "...",
    "subscription_id": "...",
    "amount": 4999,
    "currency": "usd",
    "status": "pending_payment",
    "helix_pay_charge_id": "chg_...",
    "payment_method": "helixpay",
    "issued_at": "..."
  }
}
```

### `POST /webhooks/helixpay`

Mounted as an `http.Handler` from `webhooks.NewListener`. No request/response schema — the SDK handles parsing and verification. Returns `200 OK` on success, `401` on bad signature.

---

## Error Handling

| Scenario                             | HTTP Status | Detail                          |
|--------------------------------------|-------------|---------------------------------|
| Invoice not found                    | 404         | —                               |
| Invoice not in `draft` status        | 400         | `"invoice_not_payable"`         |
| Neither `customer_id` nor email sent | 400         | `"missing_customer_identity"`   |
| HelixPay API error                   | 502         | Logged; generic error returned  |
| Webhook signature invalid            | 401         | Handled by SDK listener         |

---

## Patterns Observed in This Codebase

- Handlers follow the `handler.NewXxx(store, logger)` constructor pattern; new handlers must match this.
- All HTTP responses use the `envelope` package (`envelope.OK`, `envelope.Created`, `envelope.BadRequest`, etc.).
- Logging uses `logpipe.Logger` with structured fields (`logpipe.String(...)`).
- The `cachex` cache is used in the subscription handler for read-through caching; not needed for payment (write path).
- Routes are registered in `cmd/server/main.go` using `chi.Router`.
- The store is in-memory (`sync.RWMutex` over maps); no migration needed, just add new fields and method.