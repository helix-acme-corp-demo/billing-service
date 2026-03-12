# Requirements: Authentication for Billing Service

## Context

The billing service (`billing-service`) is a Go microservice using `go-chi/chi` that exposes REST endpoints for subscriptions, usage metering, and invoices. Currently, **all endpoints are unprotected** — no authentication or authorization is performed.

The `authtokens` library (`github.com/helix-acme-corp-demo/authtokens`) is an internal Go package that provides HS256 JWT issuing, validation, and ready-made HTTP middleware. It has zero external dependencies and is designed for exactly this use case.

## User Stories

### US-1: Protect billing endpoints with Bearer token authentication
**As** an API consumer,
**I want** the billing service to require a valid JWT Bearer token on all business endpoints,
**So that** only authenticated clients can access billing data.

**Acceptance Criteria:**
- All `/subscriptions`, `/usage`, and `/invoices` endpoints require a valid `Authorization: Bearer <token>` header.
- `GET /health` remains unauthenticated.
- Requests without a token receive a `401` JSON response with `{"error": "authorization token not provided"}`.
- Requests with an expired or malformed token receive a `401` JSON response with an appropriate error message.

### US-2: Configure authentication via environment variables
**As** a service operator,
**I want** the JWT signing secret and audience to be configured through environment variables,
**So that** I can deploy the service in different environments without code changes.

**Acceptance Criteria:**
- `AUTH_SECRET` environment variable provides the HMAC signing secret (required).
- `AUTH_AUDIENCE` environment variable sets the expected audience claim (optional).
- The service fails fast at startup if `AUTH_SECRET` is not set.

### US-3: Access authenticated user identity in handlers
**As** a developer working on the billing service,
**I want** to access the authenticated user's claims inside request handlers,
**So that** I can implement user-scoped logic in the future (e.g., "list only my subscriptions").

**Acceptance Criteria:**
- `authtokens.ClaimsFromContext(r.Context())` returns the validated claims inside any protected handler.
- No handler changes are required in this iteration — this is a wiring concern only.

## Out of Scope
- Role-based or scope-based authorization (can be added later via `WithRequiredScopes`).
- Token issuance — the billing service only validates tokens; a separate auth service issues them.
- User-scoped data filtering in handlers (future work).
- Token revocation checking (future work).