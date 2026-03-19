# Implementation Tasks

## Fix `decodeError` inconsistency in `helix-pay-go` (High Priority)

- [ ] Create `internal/transport/error.go` in `helix-pay-go` with a shared `DecodeError(resp *http.Response) error` function returning `*APIError` from the top-level `errors.go`
- [ ] Update `charges/service.go` to delete the private `apiError` struct and `decodeError` function, and import the shared one from `internal/transport`
- [ ] Update `customers/service.go` to delete its private `decodeError` function and import the shared one from `internal/transport`
- [ ] Verify callers of both packages can now use `errors.As(err, &helixpay.APIError{})` consistently

## Remove `bytesReader` — use `bytes.NewReader` (Trivial)

- [ ] In `charges/service.go`, delete the `bytesReader` struct, `Read` method, and `jsonReader()` helper; replace all call sites with `bytes.NewReader(payload)`
- [ ] In `customers/service.go`, delete the `bytesReader` struct, `Read` method, and `newReader()` helper; replace all call sites with `bytes.NewReader(payload)`

## Replace `generateUUID()` with `logpipe.GenerateID()` in `billing-service` (Trivial)

- [ ] Delete the private `generateUUID()` function from `billing-service/internal/handler/subscription.go`
- [ ] Replace all calls to `generateUUID()` across billing-service handlers with `logpipe.GenerateID()`

## Deduplicate `Health()` handler (Low Priority)

- [ ] Add a `Health() http.HandlerFunc` helper to the `envelope` package (or a new `httputil` shared package) that writes `{"status": "healthy"}`
- [ ] Remove `internal/handler/health.go` from `billing-service` and update its import to use the shared helper
- [ ] Remove `internal/handler/health.go` from `notification-service` and update its import to use the shared helper

## Generic `MapStore[T]` shared package (Medium Priority — prevents future drift)

- [ ] Create a new shared Go module `github.com/helix-acme-corp-demo/memstore` (or add to an existing shared lib) with a generic `MapStore[T any]` backed by `sync.RWMutex` and `map[string]T`, exposing `Save(id string, v T)`, `Find(id string) (T, bool)`, and `All() []T`
- [ ] Refactor `notification-service/internal/store/notification.go` to use `MapStore[*domain.Notification]`
- [ ] Refactor `billing-service/internal/store/billing.go` to use `MapStore` for each of its three entity types (subscriptions, usage, invoices)

## Ensure `code-billing-service` and `code-authtokens` are indexed by kodit (Ops)

- [ ] Push at least one commit to `code-billing-service` and `code-authtokens` repos (or trigger a kodit re-index) so they appear with a `latest` commit SHA in `kodit_repositories()`
