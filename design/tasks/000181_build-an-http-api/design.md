# Design: JWT Authentication Middleware for Billing Service

## Overview

Wire the existing `authtokens` package into `billing-service` as chi middleware. Add scope-based route groups, an in-memory revocation list, and a token refresh endpoint. No new libraries are needed — everything required already exists in the monorepo.

## Architecture

```
cmd/server/main.go
  └── builds: secret, issuer, validator, revocationList
      ├── authtokens.Middleware(validator)        ← applied to all protected routes
      ├── scopedValidator(validator, "billing:read")   ← GET route group
      ├── scopedValidator(validator, "billing:write")  ← POST/mutating route group
      └── handler.NewAuth(issuer, validator)      ← POST /auth/refresh
```

### Route Groups

| Group | Routes | Required Scope |
|---|---|---|
| Public | `GET /health` | none |
| Read | `GET /subscriptions`, `GET /subscriptions/{id}`, `GET /usage`, `GET /invoices`, `GET /invoices/{id}` | `billing:read` |
| Write | `POST /subscriptions`, `POST /subscriptions/{id}/cancel`, `POST /usage`, `POST /invoices/generate` | `billing:write` |
| Auth | `POST /auth/refresh` | signature + expiry + revocation only (no scope) |

Each group is implemented as a chi sub-router with its own `authtokens.Middleware(scopedValidator)` applied via `r.Group(...)`.

## Key Components

### `internal/auth/revocation.go` — In-Memory Revocation List

Implements `authtokens.RevocationChecker`. Stores revoked JTI strings in a `map[string]struct{}`. Populated at startup from `cfg.RevokedTokenIDs` (a string slice from env). Safe for concurrent reads via `sync.RWMutex`.

```go
type RevocationList struct {
    mu      sync.RWMutex
    revoked map[string]struct{}
}

func (r *RevocationList) IsRevoked(id string) bool { ... }
func (r *RevocationList) Revoke(id string)         { ... }
```

### `internal/handler/auth.go` — Refresh Handler

`AuthHandler.Refresh()` reads the Bearer token from the request (already validated by middleware), calls `issuer.Refresh(raw, validator)`, and returns `{"token": "<raw>"}`.

```go
type AuthHandler struct {
    issuer    authtokens.Issuer
    validator authtokens.Validator
}
```

### `config/config.go` — Extended Config

Add two fields:
- `JWTSecret string` — read from `JWT_SECRET` env var (required)
- `JWTDefaultTTL time.Duration` — read from `JWT_DEFAULT_TTL` env var (default: `1h`)
- `RevokedTokenIDs []string` — read from `REVOKED_TOKEN_IDS` env var (comma-separated, optional)

### Validator Construction in `main.go`

Two validators are built from the same secret:

1. **Base validator** — used for `/auth/refresh` and as the foundation for scoped validators.
   ```go
   baseValidator := authtokens.NewValidator(
       authtokens.WithSecret(secret),
       authtokens.WithAudience("billing-service"),
       authtokens.WithRevocationCheck(revocationList),
   )
   ```

2. **Scoped validators** — composed per route group by adding `WithRequiredScopes`:
   ```go
   readValidator := authtokens.NewValidator(
       authtokens.WithSecret(secret),
       authtokens.WithAudience("billing-service"),
       authtokens.WithRevocationCheck(revocationList),
       authtokens.WithRequiredScopes("billing:read"),
   )
   ```

This avoids a global middleware that would block the refresh endpoint from tokens without billing scopes.

## Error Responses

The `authtokens.Middleware` already writes a JSON `{"error": "<message>"}` with `401` on any validation failure — no changes needed to error handling.

## Patterns Observed in Codebase

- Handlers follow the `handler.New*(deps...) *Handler` constructor pattern; `AuthHandler` should match this.
- The chi router uses `r.Group(func(r chi.Router) { r.Use(...); r.Get/Post(...) })` for middleware scoping — use this to attach per-group validators.
- `logpipe.Logger` is threaded through all handlers; pass it to `AuthHandler` too.
- `envelope.Write` / `envelope.OK` / `envelope.Created` are used for all non-auth responses; the refresh endpoint can return a plain `json.NewEncoder(w).Encode(...)` since it doesn't fit the standard resource envelope, or use `envelope.OK`.

## What Is NOT Changing

- The `authtokens` package itself — it is complete and well-tested.
- Existing handler logic in `subscription.go`, `usage.go`, `invoice.go`.
- The `store` package.
- `GET /health` remains public.