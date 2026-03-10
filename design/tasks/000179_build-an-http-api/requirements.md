# Requirements: JWT Authentication Middleware

## Overview

Add HTTP middleware to the billing-service that validates JWT tokens, enforces scope-based access control, supports token refresh, and checks a revocation list. The middleware must integrate with the existing chi router in `cmd/server/main.go` and protect all billing endpoints.

## User Stories

### 1. Authenticated Access
**As a** client of the billing API,  
**I want** my requests to be authenticated via JWT tokens in the `Authorization: Bearer <token>` header,  
**so that** only valid, non-expired tokens can access billing resources.

**Acceptance Criteria:**
- Requests missing the `Authorization` header receive `401 Unauthorized`
- Requests with malformed or invalid JWT signatures receive `401 Unauthorized`
- Requests with expired tokens receive `401 Unauthorized` (with a specific `token_expired` error code)
- Valid tokens allow the request to proceed and inject claims into the request context
- The `/health` endpoint remains unauthenticated

### 2. Scope-Based Access Control
**As a** billing API operator,  
**I want** each endpoint to require a specific JWT scope claim,  
**so that** clients can only access the resources their token permits.

**Acceptance Criteria:**
- Tokens missing required scopes receive `403 Forbidden`
- Scope enforcement is configurable per route group or individual route
- Scope values follow the format `billing:<resource>:<action>` (e.g., `billing:subscriptions:read`)
- Required scopes per endpoint:

| Method | Path | Required Scope |
|--------|------|----------------|
| POST | `/subscriptions` | `billing:subscriptions:write` |
| GET | `/subscriptions` | `billing:subscriptions:read` |
| GET | `/subscriptions/{id}` | `billing:subscriptions:read` |
| POST | `/subscriptions/{id}/cancel` | `billing:subscriptions:write` |
| POST | `/usage` | `billing:usage:write` |
| GET | `/usage` | `billing:usage:read` |
| POST | `/invoices/generate` | `billing:invoices:write` |
| GET | `/invoices/{id}` | `billing:invoices:read` |
| GET | `/invoices` | `billing:invoices:read` |

### 3. Token Refresh
**As a** client,  
**I want** to exchange a valid (non-revoked) refresh token for a new access token,  
**so that** my session continues without re-authentication.

**Acceptance Criteria:**
- `POST /auth/refresh` accepts `{"refresh_token": "<token>"}` in the request body
- Returns a new access token and a new refresh token on success
- Returns `401` if the refresh token is expired, invalid, or revoked
- The old refresh token is invalidated after a successful refresh (rotation)
- New access tokens have a configurable TTL (default: 15 minutes)
- New refresh tokens have a configurable TTL (default: 7 days)

### 4. Token Revocation
**As an** operator,  
**I want** issued tokens to be checkable against a revocation list,  
**so that** logged-out or compromised tokens are immediately rejected.

**Acceptance Criteria:**
- Every incoming access token is checked against an in-memory revocation list (by `jti` claim)
- Revoked tokens receive `401 Unauthorized` with error code `token_revoked`
- `POST /auth/revoke` accepts `{"token": "<token>"}` and adds its `jti` to the revocation list
- The revocation endpoint requires a valid (but possibly soon-to-expire) access token
- Revoked entries are stored in the existing in-memory `Store` pattern (thread-safe, `sync.RWMutex`)

## Non-Functional Requirements

- Middleware must be compatible with the `go-chi/chi/v5` router already used in the project
- JWT parsing uses HS256 signing with a secret loaded from config/environment
- All error responses follow the existing `envelope` package conventions (e.g., `envelope.Unauthorized`, `envelope.Forbidden`)
- Logging uses the existing `logpipe` logger
- No new external dependencies beyond a JWT parsing library (e.g., `golang-jwt/jwt/v5`)