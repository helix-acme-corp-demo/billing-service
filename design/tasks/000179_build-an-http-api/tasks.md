# Implementation Tasks

## Config
- [ ] Add `JWTSecret string`, `AccessTokenTTL time.Duration`, and `RefreshTokenTTL time.Duration` fields to `config.Config`
- [ ] Load `JWTSecret` from `JWT_SECRET` env var; fail fast at startup if empty

## Dependencies
- [ ] Add `github.com/golang-jwt/jwt/v5` to `go.mod` and `go.sum`

## Domain Types
- [ ] Add `TokenClaims` struct to `internal/domain/billing.go` (embeds `jwt.RegisteredClaims`, adds `Scopes []string` and `Type string`)
- [ ] Add `RefreshRequest` struct (`RefreshToken string`)
- [ ] Add `RevokeRequest` struct (`Token string`)
- [ ] Add `TokenPair` struct (`AccessToken string`, `RefreshToken string`) for refresh responses

## Store — Revocation List
- [ ] Add `revokedJTIs map[string]time.Time` field to `store.Store`
- [ ] Initialize the map in `store.New()`
- [ ] Add `RevokeToken(jti string, expiry time.Time)` method (write lock)
- [ ] Add `IsRevoked(jti string) bool` method (read lock + lazy prune of expired entries)

## Middleware Package
- [ ] Create `internal/middleware/auth.go`
- [ ] Implement `Authenticate(store, jwtSecret, logger)` middleware: extract Bearer token, parse/verify HS256 signature, check expiry, check revocation, inject `*domain.TokenClaims` into context
- [ ] Define unexported `contextKey` type and package-level `claimsKey` var
- [ ] Export `ClaimsFromContext(ctx context.Context) (*domain.TokenClaims, bool)` helper
- [ ] Implement `RequireScope(scope string)` middleware: read claims from context, check `Scopes` slice, return 403 if missing
- [ ] Return `401` with `envelope.Unauthorized` (code `token_expired`) for expired tokens
- [ ] Return `401` with `envelope.Unauthorized` (code `token_revoked`) for revoked tokens
- [ ] Return `401` with `envelope.Unauthorized` (code `invalid_token`) for malformed/bad-signature tokens
- [ ] Return `403` with `envelope.Forbidden` for insufficient scopes

## Envelope Helpers
- [ ] Add `envelope.Unauthorized(code, message string)` function if not already present
- [ ] Add `envelope.Forbidden(code, message string)` function if not already present

## Auth Handler
- [ ] Create `internal/handler/auth.go` with `AuthHandler` struct (holds `store`, `jwtSecret`, `accessTTL`, `refreshTTL`, `logger`)
- [ ] Implement `NewAuth(...)` constructor
- [ ] Implement `Refresh() http.HandlerFunc`: parse refresh token, validate type/expiry/revocation, revoke old jti, issue new token pair, return `TokenPair`
- [ ] Implement `Revoke() http.HandlerFunc`: validate access token from header, add its jti to revocation list, return 200

## JWT Signing Helper
- [ ] Create private helper `signToken(claims *domain.TokenClaims, secret string) (string, error)` (HS256)
- [ ] Create private helper `parseToken(tokenStr, secret string) (*domain.TokenClaims, error)`

## Route Wiring (main.go)
- [ ] Instantiate `AuthHandler` in `main.go`
- [ ] Register public routes: `POST /auth/refresh` and `POST /auth/revoke`
- [ ] Wrap all billing routes in a `r.Group` with `middleware.Authenticate` applied
- [ ] Apply `r.With(middleware.RequireScope(...))` to each billing route per the scope table in requirements.md

## Tests
- [ ] Unit test `Authenticate` middleware: missing header, invalid token, expired token, revoked token, valid token
- [ ] Unit test `RequireScope` middleware: missing scope, matching scope
- [ ] Unit test `AuthHandler.Refresh`: valid refresh, expired refresh, revoked refresh, wrong token type
- [ ] Unit test `AuthHandler.Revoke`: valid revocation, invalid token
- [ ] Unit test `store.IsRevoked` with lazy pruning of expired entries