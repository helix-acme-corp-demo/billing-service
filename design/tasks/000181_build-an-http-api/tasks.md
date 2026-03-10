# Implementation Tasks

## Config
- [~] Add `JWTSecret`, `JWTDefaultTTL`, and `RevokedTokenIDs` fields to `config/config.go`, reading from `JWT_SECRET`, `JWT_DEFAULT_TTL`, and `REVOKED_TOKEN_IDS` env vars

## Revocation List
- [~] Create `internal/auth/revocation.go` with an in-memory `RevocationList` struct implementing `authtokens.RevocationChecker`
- [~] Add `IsRevoked(id string) bool` and `Revoke(id string)` methods protected by `sync.RWMutex`
- [~] Add `NewRevocationList(ids []string) *RevocationList` constructor that pre-populates from a string slice

## Auth Handler
- [ ] Create `internal/handler/auth.go` with `AuthHandler` struct holding `issuer authtokens.Issuer`, `validator authtokens.Validator`, and `logger logpipe.Logger`
- [ ] Implement `NewAuth(issuer, validator, logger)` constructor
- [ ] Implement `Refresh() http.HandlerFunc` that reads the raw Bearer token, calls `issuer.Refresh(raw, validator)`, and returns `{"token": "<raw>"}` with `200 OK`
- [ ] Return `401` with a JSON error body if `Refresh` returns an error

## Dependency Wiring in `main.go`
- [ ] Add `authtokens` import to `cmd/server/main.go`
- [ ] Build `revocationList` from `cfg.RevokedTokenIDs` using `NewRevocationList`
- [ ] Build `issuer` with `authtokens.NewIssuer(WithSecret, WithDefaultTTL, WithAudience("billing-service"))`
- [ ] Build `baseValidator` with secret + audience + revocation check (no scopes) for `/auth/refresh`
- [ ] Build `readValidator` adding `WithRequiredScopes("billing:read")` on top of base options
- [ ] Build `writeValidator` adding `WithRequiredScopes("billing:write")` on top of base options
- [ ] Create `authHandler` via `handler.NewAuth(issuer, baseValidator, logger)`

## Router Updates in `main.go`
- [ ] Keep `GET /health` outside all auth middleware (public)
- [ ] Add `r.Group` for the auth route: apply `authtokens.Middleware(baseValidator)`, register `POST /auth/refresh`
- [ ] Add `r.Group` for read routes: apply `authtokens.Middleware(readValidator)`, register all `GET` subscription/usage/invoice routes
- [ ] Add `r.Group` for write routes: apply `authtokens.Middleware(writeValidator)`, register all `POST` subscription/usage/invoice routes

## Tests
- [ ] Write `internal/auth/revocation_test.go` covering: pre-populated revoked IDs, `Revoke` adds an ID, non-revoked IDs return false
- [ ] Write `internal/handler/auth_test.go` covering: valid token returns new token, expired token returns 401, missing header returns 401
- [ ] Write an integration smoke test in `cmd/server/main_test.go` (or similar) verifying that a `GET /subscriptions` request with a `billing:read` token succeeds and one with a `billing:write`-only token returns 401