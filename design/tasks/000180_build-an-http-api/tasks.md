# Implementation Tasks

## Dependencies
- [ ] Add `github.com/golang-jwt/jwt/v5` to `go.mod` and run `go mod tidy` to update `go.sum`

## Config
- [ ] Add `JWTSecret string`, `AccessTokenTTL time.Duration`, and `RefreshTokenTTL time.Duration` fields to `config.Config`
- [ ] Load `JWTSecret` from `JWT_SECRET` env var; call `log.Fatal` at startup if empty
- [ ] Load `AccessTokenTTL` from `ACCESS_TOKEN_TTL` env var (default: 15 minutes)
- [ ] Load `RefreshTokenTTL` from `REFRESH_TOKEN_TTL` env var (default: 7 days)

## Domain Types
- [ ] Add `TokenClaims` struct to `internal/domain/billing.go` — embeds `jwt.RegisteredClaims`, adds `Scopes []string` and `Type string`
- [ ] Add `RefreshRequest` struct (`RefreshToken string`)
- [ ] Add `RevokeRequest` struct (`Token string`)
- [ ] Add `TokenPair` struct (`AccessToken string`, `RefreshToken string`) for refresh responses

## Store — Revocation List
- [ ] Add `revokedJTIs map[string]time.Time` field to `store.Store`
- [ ] Initialize the map in `store.New()`
- [ ] Add `RevokeToken(jti string, expiry time.Time)` method (write lock)
- [ ] Add `IsRevoked(jti string) bool` method — read lock, check presence, lazily prune entries whose expiry is in the past

## Envelope Helpers
- [ ] Add `envelope.Unauthorized(code, message string)` helper following the same pattern as `envelope.BadRequest`
- [ ] Add `envelope.Forbidden(code, message string)` helper following the same pattern as `envelope.BadRequest`

## Middleware Package
- [ ] Create `internal/middleware/auth.go`
- [ ] Define unexported `contextKey` type and package-level `claimsKey` var
- [ ] Export `ClaimsFromContext(ctx context.Context) (*domain.TokenClaims, bool)` helper
- [ ] Implement `Authenticate(store, jwtSecret string, logger)` middleware:
  - Extract `Authorization: Bearer <token>` header; return `401 invalid_token` if missing or malformed
  - Parse and verify HS256 signature; return `401 invalid_token` on bad signature
  - Return `401 token_expired` for expired tokens
  - Call `store.IsRevoked(jti)`; return `401 token_revoked` if true
  - Store `*domain.TokenClaims` in context and call `next.ServeHTTP`
- [ ] Implement `RequireScope(scope string)` middleware:
  - Read claims from context via `ClaimsFromContext`
  - Return `403 insufficient_scope` if the required scope is absent from `claims.Scopes`
  - Call `next.ServeHTTP` if scope is present

## JWT Helpers
- [ ] Implement private `signToken(claims *domain.TokenClaims, secret string) (string, error)` using HS256
- [ ] Implement private `parseToken(tokenStr, secret string) (*domain.TokenClaims, error)`

## Auth Handler
- [ ] Create `internal/handler/auth.go` with `AuthHandler` struct (fields: `store`, `jwtSecret string`, `accessTTL`, `refreshTTL time.Duration`, `logger`)
- [ ] Implement `NewAuth(...)` constructor
- [ ] Implement `Refresh() http.HandlerFunc`:
  - Decode `RefreshRequest` body; return `400` on bad body
  - Parse and validate refresh token (type must be `"refresh"`, not expired, not revoked)
  - Revoke old token's `jti` via `store.RevokeToken`
  - Sign new access token and new refresh token
  - Return `TokenPair` wrapped in `envelope.OK`
- [ ] Implement `Revoke() http.HandlerFunc`:
  - Decode `RevokeRequest` body; return `400` on bad body
  - Parse and validate the token (signature + expiry)
  - Call `store.RevokeToken(jti, expiry)`
  - Return `200 OK`

## Route Wiring (main.go)
- [ ] Instantiate `AuthHandler` in `main.go` and pass `cfg.JWTSecret`, `cfg.AccessTokenTTL`, `cfg.RefreshTokenTTL`
- [ ] Register public routes: `POST /auth/refresh` and `POST /auth/revoke`
- [ ] Wrap all billing routes in an `r.Group` with `middleware.Authenticate(billingStore, cfg.JWTSecret, logger)` applied
- [ ] Apply `r.With(middleware.RequireScope(...))` to each billing route per the scope table in `requirements.md`

## Tests
- [ ] Unit test `Authenticate` middleware: missing header, invalid signature, expired token, revoked token, valid token
- [ ] Unit test `RequireScope` middleware: missing scope returns 403, matching scope passes through
- [ ] Unit test `AuthHandler.Refresh`: valid refresh, expired refresh, revoked refresh, wrong token type
- [ ] Unit test `AuthHandler.Revoke`: valid token revoked, invalid token rejected
- [ ] Unit test `store.IsRevoked`: not revoked, revoked, lazy pruning of expired entries