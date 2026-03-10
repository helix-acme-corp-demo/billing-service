# Requirements: JWT Authentication Middleware

## Overview

Add HTTP middleware to the billing-service that validates JWT tokens, enforces scope-based access control, supports token refresh, and maintains an in-memory revocation list. The middleware integrates with the existing `go-chi/chi/v5` router in `cmd/server/main.go` and protects all billing endpoints.

## User Stories

### 1. Authenticated Access
**As a** client of the billing API,  
**I want** my requests authenticated via a JWT in the `Authorization: Bearer <token>` header,  
**so that** only valid, non-expired tokens can reach billing resources.

**Acceptance Criteria:**
- Requests missing `Authorization` header → `401 Unauthorized`
- Malformed header or invalid JWT signature → `401 Unauthorized` (code: `invalid_token`)
- Expired tokens → `401 Unauthorized` (code: `token_expired`)
- Valid tokens allow the request to proceed; parsed claims are injected into `context.Context`
- `GET /health` remains publicly accessible with no token required

### 2. Scope-Based Access Control
**As a** billing API operator,  
**I want** each route to require a specific JWT scope claim,  
**so that** tokens only grant access to resources they were issued for.

**Acceptance Criteria:**
- Token missing the required scope → `403 Forbidden` (code: `insufficient_scope`)
- Scopes follow the format `billing:<resource>:<action>`
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
**I want** to exchange a valid refresh token for a new token pair,  
**so that** my session continues without re-authentication.

**Acceptance Criteria:**
- `POST /auth/refresh` accepts `{"refresh_token": "<token>"}` body
- Returns a new access token and new refresh token (`{"access_token": "...", "refresh_token": "..."}`)
- Returns `401` if the refresh token is expired, invalid, or revoked
- The old refresh token `jti` is immediately revoked upon successful refresh (token rotation)
- Access token TTL default: 15 minutes; refresh token TTL default: 7 days (both configurable)
- `/auth/refresh` is a public route — it does not pass through `Authenticate` middleware

### 4. Token Revocation
**As an** operator,  
**I want** any issued token to be immediately rejectable via a revocation list,  
**so that** compromised or logged-out tokens cannot be used.

**Acceptance Criteria:**
- Every authenticated request checks the token's `jti` claim against an in-memory revocation list
- Revoked token → `401 Unauthorized` (code: `token_revoked`)
- `POST /auth/revoke` accepts `{"token": "<token>"}` and adds its `jti` to the revocation list
- Revocation list is stored in the existing in-memory `store.Store` (thread-safe, `sync.RWMutex`)
- Expired entries are lazily pruned during revocation checks to keep memory bounded

## Non-Functional Requirements

- Compatible with `go-chi/chi/v5` router (middleware as `func(http.Handler) http.Handler`)
- HS256 JWT signing; secret loaded from `JWT_SECRET` environment variable; server fails fast if empty
- All error responses use the existing `envelope` package conventions
- Structured logging via the existing `logpipe` logger
- Only one new external dependency: `github.com/golang-jwt/jwt/v5`
