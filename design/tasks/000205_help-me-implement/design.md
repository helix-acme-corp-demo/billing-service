# Design: Authentication for Billing Service

## Overview

Add JWT Bearer token authentication to the billing service by integrating the existing `authtokens` library. This is a straightforward middleware wiring task — no new authentication logic needs to be written.

## Architecture Decision

**Use `authtokens.Middleware` as a chi middleware on a route group.**

The `authtokens` library already provides:
- `Middleware(v Validator) func(http.Handler) http.Handler` — extracts Bearer token, validates, injects claims into context
- `NewValidator(opts ...Option) Validator` — creates an HS256 validator
- `ClaimsFromContext(ctx context.Context) (Claims, bool)` — retrieves claims in handlers

The billing service already uses `go-chi/chi` which supports `r.Group()` with per-group middleware. This is the natural integration point.

**Why not a custom middleware?** The `authtokens.Middleware` already handles header extraction, validation, error responses (401 JSON), and context injection. Writing a custom one would duplicate this.

## Key Patterns Discovered

- **Router:** `go-chi/chi/v5` with `r.Use()` for global middleware and inline route registration in `cmd/server/main.go`.
- **Config:** `config/config.go` uses a simple struct with a `Load()` function. Currently hardcoded — needs `os.Getenv` for auth settings.
- **Response format:** All handlers use `envelope.Write()` for consistent JSON responses. The `authtokens.Middleware` writes its own 401 JSON response (`{"error": "..."}`) which is slightly different but acceptable for auth errors.
- **Dependencies:** The `authtokens` module (`github.com/helix-acme-corp-demo/authtokens`) has zero external dependencies and uses Go 1.22, matching the billing service.

## Changes Required

### 1. `config/config.go` — Add auth configuration fields

Add `AuthSecret` (required) and `AuthAudience` (optional) to the `Config` struct. Read from `AUTH_SECRET` and `AUTH_AUDIENCE` environment variables in `Load()`.

```
type Config struct {
    Port         string
    AuthSecret   string
    AuthAudience string
}
```

### 2. `cmd/server/main.go` — Wire the middleware

- Import `authtokens`.
- Validate that `cfg.AuthSecret` is non-empty; `log.Fatal` if missing.
- Create a `Validator` with `authtokens.NewValidator(authtokens.WithSecret(...), authtokens.WithAudience(...))`.
- Use `r.Group()` to apply `authtokens.Middleware(validator)` to all business routes. `/health` stays on the top-level router, unauthenticated. Move the existing route registrations (unchanged) into the group.

```
r.Get("/health", handler.Health())          // unauthenticated

r.Group(func(r chi.Router) {
    r.Use(authtokens.Middleware(validator))
    // all existing /subscriptions, /usage, /invoices routes go here unchanged
})
```

### 3. `go.mod` — Add authtokens dependency

Run `go get github.com/helix-acme-corp-demo/authtokens` to add the dependency.

### 4. `Dockerfile` — No changes needed

The existing multi-stage Dockerfile already copies `go.mod`/`go.sum` and runs `go mod download`, so the new dependency is handled automatically.

## Constraints & Gotchas

- **`AUTH_SECRET` must be set at startup.** The service should `log.Fatal` immediately if it's missing, not silently run without auth.
- **`authtokens` uses its own error JSON format** (`{"error": "..."}`) rather than the `envelope` package format. This is acceptable for 401 responses and keeps the integration simple. Wrapping it would require a custom middleware defeating the purpose.
- **No handler code changes.** The middleware injects claims into context automatically. Handlers don't need modification in this iteration — they continue to work as before, just behind auth.
- **`/health` must remain unauthenticated** for load balancer and orchestration health checks.