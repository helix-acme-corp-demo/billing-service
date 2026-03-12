# Implementation Tasks

- [x] Add `github.com/helix-acme-corp-demo/authtokens` dependency to `billing-service/go.mod` (`go get github.com/helix-acme-corp-demo/authtokens`)
- [~] Update `billing-service/config/config.go`: add `AuthSecret` and `AuthAudience` fields to `Config` struct, read from `AUTH_SECRET` and `AUTH_AUDIENCE` environment variables in `Load()`
- [~] Update `billing-service/cmd/server/main.go`: fail fast with `log.Fatal` if `cfg.AuthSecret` is empty
- [~] Update `billing-service/cmd/server/main.go`: create an `authtokens.Validator` using `authtokens.NewValidator(authtokens.WithSecret(...))` and optionally `authtokens.WithAudience(...)` if configured
- [~] Update `billing-service/cmd/server/main.go`: wrap all business routes (`/subscriptions`, `/usage`, `/invoices`) in a `r.Group()` with `authtokens.Middleware(validator)`, keeping `GET /health` outside the group
- [ ] Verify `GET /health` returns 200 without a token
- [ ] Verify protected endpoints return 401 JSON when no token is provided
- [ ] Verify protected endpoints return 401 JSON when an expired or malformed token is provided
- [ ] Verify protected endpoints return 200 when a valid Bearer token is provided