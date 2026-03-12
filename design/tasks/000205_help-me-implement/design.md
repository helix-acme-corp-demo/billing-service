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

## Implementation Notes

### Files modified (4 total)
1. `go.mod` / `go.sum` — added `github.com/helix-acme-corp-demo/authtokens` dependency via `go get`
2. `config/config.go` — added `AuthSecret` and `AuthAudience` fields, read via `os.Getenv`
3. `cmd/server/main.go` — added fail-fast check, created validator, wrapped routes in `r.Group()`

### Order of changes that worked
1. `go get` the dependency first so the import resolves
2. Update config — no dependencies on other changes
3. Update main.go last — depends on both config fields and the authtokens import
4. Run `go mod tidy` after code changes to clean up

### Learnings for future agents
- **chi `r.Use()` is global** — it applies to ALL routes on that Mux regardless of registration order. To exempt `/health`, you must use `r.Group()` for the protected routes. Do NOT try to register `/health` before `r.Use(authtokens.Middleware(...))` thinking it will be excluded — it won't.
- **`authtokens` option pattern** — build a `[]authtokens.Option` slice and conditionally append `WithAudience()` only if the env var is set. Pass the slice with `validatorOpts...`.
- **`authtokens` 401 response format** — returns `{"error": "..."}` which differs from the billing service's `envelope` format (`{"ok": false, "error": {...}}`). This is acceptable for auth errors and avoids writing a wrapper.
- **No handler changes needed** — the middleware injects claims into context via `authtokens.ClaimsFromContext()`. Handlers work unchanged behind auth.
- **`authtokens` is available on the default Go module proxy** — no GOPRIVATE or special proxy config needed for `helix-acme-corp-demo` modules.

### Verification results
- `GET /health` → 200 (no token required) ✓
- `GET /subscriptions` (no token) → 401 `{"error":"authorization token not provided"}` ✓
- `GET /subscriptions` (malformed token) → 401 `{"error":"token is malformed"}` ✓
- `GET /subscriptions` (wrong signature) → 401 `{"error":"signature verification failed"}` ✓
- `GET /subscriptions` (valid token) → 200 with data ✓
- `POST /subscriptions` (valid token) → 201 ✓
- `POST /usage` (valid token) → 201 ✓