# Design: JWT Authentication Middleware

## Architecture Overview

The middleware is implemented as a set of chi-compatible `func(http.Handler) http.Handler` functions living in a new package `internal/middleware`. Two middleware functions are needed:

1. **`Authenticate`** — extracts and validates the JWT from the `Authorization` header, checks revocation, and stores claims in the request context.
2. **`RequireScope(scope string)`** — reads claims from context and enforces a specific scope.

A new handler package `internal/handler/auth.go` provides the `/auth/refresh` and `/auth/revoke` HTTP endpoints. The revocation list lives in the existing `store.Store`, extended with a `revokedTokens` map.

```
billing-service/
├── cmd/server/main.go          ← wire middleware + auth routes
├── config/config.go            ← add JWTSecret, AccessTTL, RefreshTTL
├── internal/
│   ├── domain/billing.go       ← add TokenClaims, RefreshRequest, RevokeRequest types
│   ├── middleware/
│   │   └── auth.go             ← Authenticate + RequireScope middleware
│   ├── handler/
│   │   └── auth.go             ← /auth/refresh and /auth/revoke handlers
│   └── store/billing.go        ← extend Store with revocation list methods
```

## Key Design Decisions

### JWT Library
Use `github.com/golang-jwt/jwt/v5` — the standard Go JWT library with active maintenance, no transitive deps, and HS256 support out of the box. Add it to `go.mod`.

### Claims Shape
```
{
  "sub":    "user-uuid",
  "jti":    "unique-token-id",
  "scopes": ["billing:subscriptions:read", "billing:usage:write"],
  "type":   "access" | "refresh",
  "exp":    <unix timestamp>,
  "iat":    <unix timestamp>
}
```
Custom `TokenClaims` struct embeds `jwt.RegisteredClaims` and adds `Scopes []string` and `Type string`.

### Context Key
Claims are stored in `context.Value` using an unexported package-level key type to avoid collisions:
```go
type contextKey struct{}
var claimsKey = contextKey{}
```
A helper `ClaimsFromContext(ctx) (*TokenClaims, bool)` is exported for use in handlers.

### Revocation List
Stored in `store.Store` as `revokedJTIs map[string]time.Time` (jti → expiry time). On every authenticated request the middleware calls `store.IsRevoked(jti)`. Entries whose stored expiry has passed are lazily pruned during the `IsRevoked` check to keep memory bounded without a background goroutine.

### Token Refresh Flow
1. Client POSTs `{"refresh_token": "..."}` to `/auth/refresh`.
2. Handler parses and validates the token (type must be `"refresh"`, not revoked, not expired).
3. Old refresh token's `jti` is added to the revocation list.
4. New access token and new refresh token are signed and returned.
5. `/auth/refresh` is exempt from `Authenticate` middleware (it does its own token parsing internally).

### Route Wiring in main.go
```go
// Public
r.Get("/health", handler.Health())
r.Post("/auth/refresh", authHandler.Refresh())
r.Post("/auth/revoke", authHandler.Revoke())  // requires valid access token internally

// Protected — apply Authenticate to all billing routes
r.Group(func(r chi.Router) {
    r.Use(middleware.Authenticate(store, cfg.JWTSecret, logger))

    r.With(middleware.RequireScope("billing:subscriptions:write")).Post("/subscriptions", ...)
    r.With(middleware.RequireScope("billing:subscriptions:read")).Get("/subscriptions", ...)
    // ... etc
})
```

### Config
Two new fields added to `config.Config`:
- `JWTSecret string` — loaded from `JWT_SECRET` env var; server fails to start if empty.
- `AccessTokenTTL time.Duration` — default 15 minutes.
- `RefreshTokenTTL time.Duration` — default 7 days.

### Error Responses
All errors use the existing `envelope` package. Since `envelope.Unauthorized` and `envelope.Forbidden` may not exist yet, add them following the same pattern as `envelope.BadRequest` / `envelope.NotFound`.

## Sequence Diagram

```
Client                  Middleware (Authenticate)         Store            Handler
  |                              |                           |                 |
  |-- GET /subscriptions ------> |                           |                 |
  |   Authorization: Bearer xxx  |                           |                 |
  |                              |-- parse + verify JWT      |                 |
  |                              |-- store.IsRevoked(jti) -> |                 |
  |                              |<- false ------------------|                 |
  |                              |-- inject claims in ctx                      |
  |                              |--------------- next(w, r) ---------------> |
  |                              |                           |  RequireScope   |
  |                              |                           |  checks context |
  |<------------------------------------------------------------- 200 OK ------|
```

## Codebase Patterns Observed

- **Handler pattern**: each handler struct is initialized with `store`, `cache` (optional), and `logger`; methods return `http.HandlerFunc`. The new `AuthHandler` follows this exact shape.
- **Envelope responses**: all responses go through `envelope.Write(w, envelope.OK(...))` — errors must do the same.
- **Logging**: structured logging via `logpipe.String(key, value)` calls on the logger.
- **In-memory store**: `store.Store` uses `sync.RWMutex` for all map access; the revocation map must follow the same locking discipline.
- **Config**: flat struct loaded once at startup; new fields follow the same pattern.
- **No database**: everything is in-memory, consistent with the rest of the service.