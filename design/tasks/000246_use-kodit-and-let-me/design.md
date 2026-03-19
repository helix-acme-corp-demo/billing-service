# Design: Kodit Repo Discovery & Duplication Analysis

## Repositories Overview

Kodit tracks 5 repositories across the `helix-acme-corp-demo` organisation:

| Repo | Language | Purpose |
|---|---|---|
| `code-notification-service` | Go | HTTP service for creating & delivering notifications |
| `code-billing-service` | Go | HTTP service for subscriptions, usage tracking & invoicing |
| `code-helix-pay-go` | Go | SDK/client library for the Helix Pay payment gateway |
| `code-authtokens` | Go | Shared auth token validation library |
| `ratelimit` | Go | Shared rate-limiting library (token bucket + sliding window) |

Two repos (`code-billing-service`, `code-authtokens`) have no commits indexed by kodit yet. The billing-service source is available locally.

All active services use the same shared ecosystem:
- **`logpipe`** — structured logging + ID generation
- **`envelope`** — consistent HTTP response wrapper
- **`retryx`** — retry logic with backoff
- **`cachex`** — in-memory cache
- **`authtokens`** — JWT middleware
- **`chi`** — HTTP router

---

## Duplication Patterns Found

### Pattern A: Private `bytesReader` (copy-paste of stdlib)

Both `charges/service.go` and `customers/service.go` in `helix-pay-go` define their own private struct to wrap `[]byte` as an `io.Reader`:

```helix-pay-go/charges/service.go
type bytesReader struct {
    data []byte
    pos  int
}
func (r *bytesReader) Read(p []byte) (int, error) { ... }
```

```helix-pay-go/customers/service.go
type bytesReader struct {
    data []byte
    pos  int
}
func (r *bytesReader) Read(p []byte) (int, error) { ... }
```

The standard library already provides `bytes.NewReader()` which is identical in behaviour. Both files should be simplified to use it directly.

---

### Pattern B: `decodeError` — duplicated with structural drift

Both sub-packages implement `decodeError(resp *http.Response) error` independently. The implementations diverge subtly:

- `charges/service.go` returns a private `*apiError` struct with an `Unwrap()` method
- `customers/service.go` returns a plain `fmt.Errorf` string — no type, no unwrapping

Meanwhile, the top-level `errors.go` defines the public `APIError` struct that neither sub-package uses. This means callers of the charges package *can* do `errors.As(err, &apiError)` but callers of the customers package cannot — a silent API inconsistency.

**Root cause:** packages were written independently without a shared internal HTTP transport helper.

**Fix direction:** Introduce `internal/transport` with a single `decodeError` returning `*APIError`.

---

### Pattern C: `Health()` handler — identical across services

`billing-service` and `notification-service` both contain:

```billing-service/internal/handler/health.go
func Health() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        envelope.Write(w, envelope.OK(map[string]string{"status": "healthy"}))
    }
}
```

The files are byte-for-byte identical. Since both services already depend on `envelope`, the simplest fix is adding `envelope.Health()` to that package, or creating a small `httputil` shared library.

---

### Pattern D: In-memory store — same mechanical pattern

Both services implement a `sync.RWMutex`-protected `map[string]*Entity` store. The shape is identical:

```
New() *Store          → allocate maps
Save(entity)          → Lock, write
Find(id) (*T, bool)   → RLock, lookup
All() []*T            → RLock, copy to slice
```

The billing store has three entity types (subscriptions, usage, invoices) but each follows the same pattern. Go 1.18+ generics would allow a shared `MapStore[T]` to replace all of this. Since the `envelope` and `logpipe` libs are already shared, a `storex` or `memstore` package would fit this ecosystem.

---

### Pattern E: `generateUUID()` vs `logpipe.GenerateID()`

`billing-service/internal/handler/subscription.go` defines its own `generateUUID()` using `crypto/rand`. The `logpipe` package (already imported by billing-service) exports `GenerateID()` for the same purpose, which is what the notification-service uses.

No new code needed — just replace calls to `generateUUID()` with `logpipe.GenerateID()` and delete the private function.

---

### Pattern F: Chi bootstrap boilerplate

Both `main.go` files wire chi with the same three middleware lines in the same order. Low impact — acceptable to leave as-is, but worth noting if a third service is added.

---

## Key Design Observations

- **Shared library ecosystem is healthy.** The organisation has extracted logging, retries, caching, auth, and response formatting into reusable packages. New duplication is appearing at the *application layer*, not the library layer.
- **The `helix-pay-go` SDK has internal cohesion issues.** Sub-packages (`charges`, `customers`) were written as siblings without a shared internal HTTP layer, causing both the `bytesReader` and `decodeError` drift.
- **Error handling is the highest-risk duplication.** The `decodeError` inconsistency between `charges` and `customers` means the public API behaves differently depending on which sub-package you call — a real correctness issue, not just aesthetic duplication.
- **The in-memory store pattern is consistent but unscalable.** As more services are added, each will likely copy this pattern. A generic `MapStore[T]` in a shared package would prevent further drift.

---

## Recommended Fix Priority

| # | Finding | Risk | Effort |
|---|---|---|---|
| 1 | `decodeError` inconsistency in `helix-pay-go` | High (API behaviour differs) | Low |
| 2 | `bytesReader` — replace with `bytes.NewReader` | Low (correctness fine, just noise) | Trivial |
| 3 | `generateUUID` — use `logpipe.GenerateID()` | Low | Trivial |
| 4 | `Health()` — move to `envelope` or `httputil` | Low | Low |
| 5 | Generic `MapStore[T]` shared package | Medium (prevents future drift) | Medium |
| 6 | Chi bootstrap boilerplate | Cosmetic | Low |