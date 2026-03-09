# Requirements: HelixPay Payment Integration

## Overview

Integrate HelixPay as a payment method in the billing service, allowing users to pay invoices using HelixPay.

## User Stories

### US-1: Pay an Invoice via HelixPay
As a user, I want to pay an outstanding invoice using HelixPay so that I have a new payment option beyond existing methods.

**Acceptance Criteria:**
- A user can submit a payment for an invoice by providing a HelixPay payment token/reference
- On successful payment, the invoice status transitions from `draft` to `paid` and `paid_at` is set
- On failed payment, the invoice remains in its current state and an error is returned
- The payment attempt is logged with invoice ID and outcome

### US-2: View Payment Status on Invoice
As a user, I want to see whether my invoice was paid via HelixPay so that I have a clear payment record.

**Acceptance Criteria:**
- The invoice response includes a `payment_method` field (e.g. `"helixpay"`) when paid via HelixPay
- The existing `GET /invoices/{id}` endpoint reflects updated status and `paid_at` timestamp

## Functional Requirements

| ID   | Requirement |
|------|-------------|
| FR-1 | A new endpoint `POST /invoices/{id}/pay` accepts a HelixPay payment token and processes payment |
| FR-2 | The service calls the HelixPay API to charge the given token for the invoice amount |
| FR-3 | On HelixPay success, the invoice is marked `paid` with `paid_at` timestamp and `payment_method = "helixpay"` |
| FR-4 | On HelixPay failure, return a `402 Payment Required` error with a descriptive message |
| FR-5 | Payment calls to HelixPay are retried with exponential backoff using the existing `retryx` library |
| FR-6 | The HelixPay base URL and API key are configurable via environment variables |

## Non-Functional Requirements

- Payment calls must be retried on transient errors (network timeouts, 5xx from HelixPay)
- HelixPay credentials must never be logged
- The integration must not break any existing endpoints