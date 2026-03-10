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

All errors go through `envelope.Write`. Add `envelope.Unauthorized(code, message string)` and `envelope.Forbidden(code, message string)` following the same pattern as the existing `envelope.BadRequest` and `envelope.NotFound`.

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