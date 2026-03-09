# Requirements: HelixPay Payment Integration

## Overview

Integrate the `helix-pay-go` SDK into the `billing-service` so that users can pay invoices via HelixPay. When an invoice is paid, the service initiates a charge through HelixPay, tracks the charge status, and marks the invoice as paid once the charge settles.

---

## User Stories

### 1. Pay an Invoice via HelixPay

**As a** user with an outstanding invoice,  
**I want to** submit a HelixPay payment for that invoice,  
**So that** the invoice is marked as paid and my subscription remains active.

**Acceptance Criteria:**
- `POST /invoices/{id}/pay` accepts a `customer_id` (HelixPay customer ID) and initiates a charge for the invoice amount and currency.
- The invoice status transitions from `draft` → `pending_payment` immediately.
- The HelixPay charge ID is stored on the invoice record.
- Returns `202 Accepted` with the updated invoice (status `pending_payment`, charge ID included).
- Returns `404` if the invoice does not exist.
- Returns `400` if the invoice is already paid, voided, or in a non-payable state.

---

### 2. Register a HelixPay Customer

**As a** user,  
**I want to** be registered as a HelixPay customer when I first pay,  
**So that** my payment identity is tracked in HelixPay.

**Acceptance Criteria:**
- If a `customer_id` is not provided, the caller may optionally provide `email` and `name` to auto-register a new HelixPay customer.
- `Customers.Register` is idempotent by email — re-registering the same email returns the existing customer.
- The HelixPay `customer_id` is attached as metadata on the invoice.

---

### 3. Handle Charge Settlement via Webhook

**As a** billing operator,  
**I want** the service to automatically mark invoices as paid when HelixPay settles a charge,  
**So that** invoice state stays in sync without manual intervention.

**Acceptance Criteria:**
- `POST /webhooks/helixpay` receives and verifies inbound webhook events using HMAC-SHA256 (`X-HelixPay-Signature` header).
- On `charge.settled`: the matching invoice status changes to `paid` and `paid_at` is set.
- On `charge.declined`: the invoice status changes back to `draft` (allowing retry).
- On `charge.voided`: the invoice status changes to `draft`.
- Unrecognised or unhandled event types respond with `200 OK` and are ignored.
- Requests with invalid/missing signatures respond with `401 Unauthorized`.

---

### 4. Configuration

**As a** platform operator,  
**I want** HelixPay credentials to be supplied via environment variables,  
**So that** secrets are not hardcoded in the codebase.

**Acceptance Criteria:**
- The following environment variables are read at startup:
  - `HELIXPAY_API_KEY` — merchant API key (required, prefix `hpk_`)
  - `HELIXPAY_MERCHANT_ID` — merchant ID (required)
  - `HELIXPAY_WEBHOOK_SECRET` — webhook signing secret (required)
  - `HELIXPAY_ENV` — `sandbox` or `production` (defaults to `sandbox`)
- Service fails to start if required variables are missing.