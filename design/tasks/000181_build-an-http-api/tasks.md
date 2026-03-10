# Implementation Tasks

## Config
- [x] Add `JWTSecret`, `JWTDefaultTTL`, and `RevokedTokenIDs` fields to `config/config.go`, reading from `JWT_SECRET`, `JWT_DEFAULT_TTL`, and `REVOKED_TOKEN_IDS` env vars

## Revocation List
- [x] Create `internal/auth/revocation.go` with an in-memory `RevocationList` struct implementing `authtokens.RevocationChecker`
- [x] Add `IsRevoked(id string) bool` and `Revoke(id string)` methods protected by `sync.RWMutex`
- [x] Add `NewRevocationList(ids []string) *RevocationList` constructor that pre-populates from a string slice

## Auth Handler
- [x] Create `internal/handler/auth.go` with `AuthHandler` struct holding `issuer authtokens.Issuer`, `validator authtokens.Validator`, and `logger logpipe.Logger`
- [x] Implement `NewAuth(issuer, validator, logger)` constructor
- [x] Implement `Refresh() http.HandlerFunc` that reads the raw Bearer token, calls `issuer.Refresh(raw, validator)`, and returns `{"token": "<raw>"}` with `200 OK`
- [x] Return `401` with a JSON error body if `Refresh` returns an error

## Dependency Wiring in `main.go`
- [x] Add `authtokens` import to `cmd/server/main.go`
- [x] Build `revocationList` from `cfg.RevokedTokenIDs` using `NewRevocationList`
- [x] Build `issuer` with `authtokens.NewIssuer(WithSecret, WithDefaultTTL, WithAudience("billing-service"))`
- [x] Build `baseValidator` with secret + audience + revocation check (no scopes) for `/auth/refresh`
- [x] Build `readValidator` adding `WithRequiredScopes("billing:read")` on top of base options
- [x] Build `writeValidator` adding `WithRequiredScopes("billing:write")` on top of base options
- [x] Create `authHandler` via `handler.NewAuth(issuer, baseValidator, logger)`

## Router Updates in `main.go`
- [x] Keep `GET /health` outside all auth middleware (public)
- [x] Add `r.Group` for the auth route: apply `authtokens.Middleware(baseValidator)`, register `POST /auth/refresh`
- [x] Add `r.Group` for read routes: apply `authtokens.Middleware(readValidator)`, register all `GET` subscription/usage/invoice routes
- [x] Add `r.Group` for write routes: apply `authtokens.Middleware(writeValidator)`, register all `POST` subscription/usage/invoice routes

## Tests
- [x] Write `internal/auth/revocation_test.go` covering: pre-populated revoked IDs, `Revoke` adds an ID, non-revoked IDs return false
- [x] Write `internal/handler/auth_test.go` covering: valid token returns new token, expired token returns 401, missing header returns 401
- [x] Write an integration smoke test in `cmd/server/main_test.go` (or similar) verifying that a `GET /subscriptions` request with a `billing:read` token succeeds and one with a `billing:write`-only token returns 401