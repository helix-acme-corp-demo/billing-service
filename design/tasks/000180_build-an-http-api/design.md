# Design: JWT Authentication Middleware

## Architecture Overview

A new `internal/middleware` package provides two chi-compatible middleware functions. A new `internal/handler/auth.go` exposes token refresh and revocation endpoints. The revocation list is stored inside the existing `store.Store`.

```
billing-service/
├── cmd/server/main.go          ← wire middleware + auth routes
├── config/config.go            ← add JWTSecret, AccessTokenTTL, RefreshTokenTTL
├── internal/
│   ├── domain/billing.go       ← add TokenClaims, RefreshRequest, RevokeRequest, TokenPair
│   ├── middleware/
│   │   └── auth.go             ← Authenticate + RequireScope
│   ├── handler/
│   │   └── auth.go             ← /auth/refresh and /auth/revoke handlers
│   └── store/billing.go        ← extend Store with revokedJTIs map + methods
```

## Key Design Decisions

### JWT Library
Use `github.com/golang-jwt/jwt/v5` — the canonical Go JWT library. Supports HS256 out of the box with no transitive dependencies. Add it to `go.mod`.

### Claims Shape
Custom `TokenClaims` struct embeds `jwt.RegisteredClaims` (which provides `exp`, `iat`, `jti`, `sub`) and adds two fields:

```billing-service/internal/domain/billing.go
type TokenClaims struct {
    jwt.RegisteredClaims
    Scopes []string `json:"scopes"`
    Type   string   `json:"type"` // "access" or "refresh"
}
```

### Middleware Functions

**`Authenticate(store, jwtSecret, logger)`** — extracts `Authorization: Bearer <token>`, parses and verifies the HS256 signature, rejects expired tokens, checks the revocation list by `jti`, then stores `*domain.TokenClaims` in the request context via an unexported key:

```billing-service/internal/middleware/auth.go
type contextKey struct{}
var claimsKey = contextKey{}

func ClaimsFromContext(ctx context.Context) (*domain.TokenClaims, bool) { ... }
```

**`RequireScope(scope string)`** — reads claims from context (set by `Authenticate`), checks whether the required scope exists in `claims.Scopes`, and returns `403` if absent. Applied per-route with `r.With(...)`.

### Revocation List

`store.Store` gains a `revokedJTIs map[string]time.Time` field (jti → token expiry). Two new methods:

- `RevokeToken(jti string, expiry time.Time)` — write-locks and stores the entry.
- `IsRevoked(jti string) bool` — read-locks, checks presence, and lazily prunes entries whose stored expiry is in the past before returning.

Lazy pruning avoids the need for a background goroutine and keeps memory bounded naturally.

### Auth Handler

`AuthHandler` in `internal/handler/auth.go` holds `store`, `jwtSecret`, `accessTTL`, `refreshTTL`, and `logger`. Two handlers:

- **`Refresh()`** — parses the refresh token, validates type (`"refresh"`), expiry, and revocation; revokes the old `jti`; signs and returns a new `TokenPair`.
- **`Revoke()`** — validates the access token from the `Authorization` header, adds its `jti` to the revocation list.

Both use private helpers `signToken` and `parseToken` defined in the same file.

### Token Refresh Flow

```
Client                     POST /auth/refresh                 Store
  |                               |                              |
  |-- {"refresh_token":"..."}  -> |                              |
  |                               |-- parseToken(refresh)        |
  |                               |-- IsRevoked(jti) ----------> |
  |                               |<- false -------------------- |
  |                               |-- RevokeToken(old jti) ----> |
  |                               |-- signToken(new access)      |
  |                               |-- signToken(new refresh)     |
  |<-- {"access_token","refresh_token"} ------------------------- |
```

### Route Wiring

```billing-service/cmd/server/main.go
// Public
r.Get("/health", handler.Health())
r.Post("/auth/refresh", authHandler.Refresh())
r.Post("/auth/revoke",  authHandler.Revoke())

// Protected billing routes
r.Group(func(r chi.Router) {
    r.Use(middleware.Authenticate(billingStore, cfg.JWTSecret, logger))

    r.With(middleware.RequireScope("billing:subscriptions:write")).Post("/subscriptions", ...)
    r.With(middleware.RequireScope("billing:subscriptions:read")).Get("/subscriptions", ...)
    r.With(middleware.RequireScope("billing:subscriptions:read")).Get("/subscriptions/{id}", ...)
    r.With(middleware.RequireScope("billing:subscriptions:write")).Post("/subscriptions/{id}/cancel", ...)

    r.With(middleware.RequireScope("billing:usage:write")).Post("/usage", ...)
    r.With(middleware.RequireScope("billing:usage:read")).Get("/usage", ...)

    r.With(middleware.RequireScope("billing:invoices:write")).Post("/invoices/generate", ...)
    r.With(middleware.RequireScope("billing:invoices:read")).Get("/invoices/{id}", ...)
    r.With(middleware.RequireScope("billing:invoices:read")).Get("/invoices", ...)
})
```

### Config

Three new fields in `config.Config`:

| Field | Env Var | Default |
|-------|---------|---------|
| `JWTSecret string` | `JWT_SECRET` | — (fatal if empty) |
| `AccessTokenTTL time.Duration` | `ACCESS_TOKEN_TTL` | 15 minutes |
| `RefreshTokenTTL time.Duration` | `REFRESH_TOKEN_TTL` | 7 days |

### Error Responses

All errors go through `envelope.Write`. The `envelope` package already ships `Unauthorized(message string)` and `Forbidden(message string)` — but they hard-code the `Error.Code` field to `"unauthorized"` and `"forbidden"` respectively, with no way to pass a custom code. Because we need distinct codes (`token_expired`, `token_revoked`, `invalid_token`, `insufficient_scope`), the middleware builds `envelope.Response` values directly instead of calling those helpers:

```billing-service/internal/middleware/auth.go
func tokenError(status int, code, message string) envelope.Response {
    return envelope.Response{
        Status: status,
        OK:     false,
        Error:  &envelope.ErrorDetail{Code: code, Message: message},
    }
}
```

No changes to the `envelope` package are needed. The Envelope Helpers tasks in tasks.md are removed.

| Condition | Status | Code |
|-----------|--------|------|
| Condition | Status | Code |
|-----------|--------|------|
| Missing / malformed token | 401 | `invalid_token` |
| Expired token | 401 | `token_expired` |
| Revoked token | 401 | `token_revoked` |
| Missing scope | 403 | `insufficient_scope` |

## Codebase Patterns Observed

- **Handler shape**: each handler struct takes `store`, optionally `cache`, and `logger`; methods return `http.HandlerFunc`. `AuthHandler` follows this exact pattern.
- **Envelope responses**: every response (success and error) goes through `envelope.Write(w, ...)`. The middleware must follow suit.
- **Structured logging**: use `logpipe.String(key, value)` on the injected logger for all log lines.
- **In-memory store**: `store.Store` uses `sync.RWMutex` protecting all maps. The new `revokedJTIs` map must follow the same locking discipline as the existing maps.
- **Config**: flat struct loaded once at startup from environment; no configuration library needed.
- **No database**: the service is entirely in-memory, consistent with the existing store.

## Implementation Notes

- **Branch**: `feature/000180-build-an-http-api` off `main`
- **New files created**:
  - `internal/middleware/auth.go` — `Authenticate`, `RequireScope`, `SignToken`, `ParseToken`, `ClaimsFromContext`
  - `internal/handler/auth.go` — `AuthHandler` with `Refresh()` and `Revoke()` handlers
  - `internal/middleware/auth_test.go` — 9 tests covering all middleware cases
  - `internal/handler/auth_test.go` — 11 tests covering refresh and revoke flows
  - `internal/store/revocation_test.go` — 5 tests covering revocation list and lazy pruning
- **Modified files**:
  - `config/config.go` — added `JWTSecret`, `AccessTokenTTL`, `RefreshTokenTTL`; `log.Fatal` on missing `JWT_SECRET`
  - `internal/domain/billing.go` — added `TokenClaims`, `RefreshRequest`, `RevokeRequest`, `TokenPair`
  - `internal/store/billing.go` — added `revokedJTIs` map, `RevokeToken`, `IsRevoked` methods
  - `cmd/server/main.go` — aliased `chi/v5/middleware` as `chimiddleware` to avoid collision; wired auth handler and protected route group
  - `go.mod` / `go.sum` — added `github.com/golang-jwt/jwt/v5 v5.3.1`

- **Envelope discovery**: `envelope.Unauthorized` and `envelope.Forbidden` already exist but only accept a `message string` — no `code` parameter. Rather than modifying the external package, the middleware and auth handler construct `envelope.Response` values directly using the exported `envelope.Response` and `envelope.ErrorDetail` structs. No changes to the envelope package were needed.

- **SignToken/ParseToken exported**: Originally designed as private helpers, they were promoted to exported (`SignToken`, `ParseToken`) so that `internal/handler/auth.go` could reuse them without duplicating logic. The `Authenticate` middleware now calls `ParseToken` internally.

- **`revocationStore` interface**: The middleware uses a narrow `revocationStore` interface (`IsRevoked(string) bool`) rather than importing `*store.Store` directly. This keeps the middleware package free of a circular import and makes it easy to test with a `fakeStore`.

- **`authStore` interface**: Similarly, `AuthHandler` uses an `authStore` interface (`RevokeToken`, `IsRevoked`) rather than depending on `*store.Store` directly, enabling independent unit testing with `fakeAuthStore`.

- **`IsRevoked` uses write lock for pruning**: The lazy-prune path inside `IsRevoked` calls `delete(s.revokedJTIs, jti)`, which requires a write lock. The method therefore acquires `s.mu.Lock()` (not `RLock()`) to handle both the read and the conditional delete safely.

- **All 23 tests pass**, `go vet` reports no issues.