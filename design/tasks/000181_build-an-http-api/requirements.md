# Requirements: JWT Authentication Middleware for Billing Service

## Background

The billing-service exposes HTTP endpoints (subscriptions, usage, invoices) via a chi router with no authentication today. The `authtokens` package already exists (`github.com/helix-acme-corp-demo/authtokens`) and provides a complete HS256 JWT issuer, validator, HTTP middleware, and revocation checker interface. The task is to wire this middleware into the billing-service router with per-route scope enforcement and a token refresh endpoint.

## User Stories

### 1. Token Validation
**As an** API consumer,  
**I want** my JWT Bearer token validated on every protected request,  
**So that** unauthenticated callers are rejected before reaching business logic.

**Acceptance Criteria:**
- Requests without an `Authorization: Bearer <token>` header receive `401 Unauthorized`
- Requests with a malformed or tampered token receive `401 Unauthorized`
- Requests with an expired token receive `401 Unauthorized`
- Valid tokens allow the request to proceed; `Claims` are available in the request context

### 2. Scope-Based Access Control
**As an** API operator,  
**I want** each route group to require specific JWT scopes,  
**So that** a token with `read` scope cannot mutate data, and vice versa.

**Acceptance Criteria:**
- `GET` endpoints (list/get subscriptions, usage, invoices) require the `billing:read` scope
- `POST` endpoints (create subscription, record usage, generate invoice) require the `billing:write` scope
- `POST /subscriptions/{id}/cancel` requires the `billing:write` scope
- A token missing a required scope receives `401 Unauthorized` (propagates `ErrInsufficientScopes`)
- A token with a superset of scopes (e.g. `billing:read billing:write`) passes all checks

### 3. Revocation List Check
**As an** API operator,  
**I want** revoked tokens to be rejected even if they are otherwise valid,  
**So that** compromised or logged-out tokens cannot be reused.

**Acceptance Criteria:**
- An in-memory `RevocationChecker` implementation is provided (satisfies the `authtokens.RevocationChecker` interface)
- Revoked token IDs (`jti` claim) are rejected with `401 Unauthorized`
- Non-revoked tokens are unaffected
- The revocation list can be populated at startup (e.g. from a config slice or environment)

### 4. Token Refresh Endpoint
**As an** API consumer,  
**I want** to exchange a valid (non-expired) token for a fresh one,  
**So that** long-lived sessions do not require re-login.

**Acceptance Criteria:**
- `POST /auth/refresh` accepts a Bearer token in the `Authorization` header
- A valid, non-expired token is re-issued with a new `iat` / `exp` and the same claims (subject, audience, extra)
- An expired or revoked token returns `401 Unauthorized`
- The refreshed token is returned as `{"token": "<raw>"}` with status `200 OK`
- The `/auth/refresh` route is protected by the base validator (signature + expiry + revocation) but does NOT require billing scopes

### 5. Public Health Endpoint
**As an** operator,  
**I want** `GET /health` to remain unauthenticated,  
**So that** load balancers and uptime monitors work without credentials.

**Acceptance Criteria:**
- `GET /health` continues to respond `200 OK` with no `Authorization` header required