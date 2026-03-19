# Requirements: Kodit Repo Discovery & Duplication Analysis

## Repositories Visible to Kodit

Kodit can see **5 repositories**:

| Repo URL | Latest Commit |
|---|---|
| `http://api:8080/git/code-notification-service-1772881276` | `ea65e93` |
| `http://api:8080/git/code-billing-service-1773074600` | *(no commits indexed)* |
| `http://api:8080/git/code-helix-pay-go-1773077078` | `faa6671` |
| `http://api:8080/git/code-authtokens-1773137218` | *(no commits indexed)* |
| `https://github.com/helix-acme-corp-demo/ratelimit.git` | `45465db` |

> Note: `code-billing-service` and `code-authtokens` had no commits indexed by kodit, but the billing-service code is available locally at `/home/retro/work/billing-service`.

---

## Duplication Findings

### 1. `bytesReader` — Copy-Pasted in Two Places (High Confidence)

**Exact structural duplicate** between `helix-pay-go` `charges/service.go` and `customers/service.go`.

Both files define an identical private `bytesReader` struct and its `Read` method, used as a drop-in `io.Reader` around a `[]byte` slice. This is internal to each sub-package because `bytes.NewReader` wasn't used.

Affected files:
- `charges/service.go` — `jsonReader()` + `bytesReader`
- `customers/service.go` — `newReader()` + `bytesReader`

**Fix:** Replace both with `bytes.NewReader(payload)` from the standard library. No custom struct needed.

---

### 2. `decodeError` — Near-Duplicate Error Decoder (High Confidence)

Both `charges/service.go` and `customers/service.go` in `helix-pay-go` implement their own private `decodeError(resp *http.Response) error` function with near-identical logic: read body JSON, extract `code`/`message`/`request_id`, return an error.

The only difference is `charges` returns a typed `*apiError` struct (with `Unwrap`) while `customers` returns a plain `fmt.Errorf` string — meaning the customers package silently drops structured error info that charges exposes.

This is also inconsistent with the top-level `errors.go` which defines the canonical `APIError` type that neither sub-package actually uses.

**Fix:** Move `decodeError` into a shared internal package (e.g. `internal/transport`) returning `*APIError` from `errors.go`. Both sub-packages import it.

---

### 3. `Health()` Handler — Copy-Pasted Across Services (Medium Confidence)

`billing-service/internal/handler/health.go` and `notification-service/internal/handler/health.go` are byte-for-byte identical:

```go
func Health() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        envelope.Write(w, envelope.OK(map[string]string{"status": "healthy"}))
    }
}
```

Both services share the same `github.com/helix-acme-corp-demo/envelope` dependency, so this could live in a shared library or in `envelope` itself.

**Fix:** Add a `Health()` helper to the `envelope` package, or extract a `github.com/helix-acme-corp-demo/httputil` shared library.

---

### 4. In-Memory Store Pattern — Structural Duplication (Medium Confidence)

Both `notification-service/internal/store/notification.go` and `billing-service/internal/store/billing.go` implement the same in-memory `sync.RWMutex`-protected map store pattern:

- `New()` → allocates map(s)
- `Save(entity)` → write-locks, writes to map
- `Find(id)` → read-locks, looks up by ID
- `All()` / `AllX()` → read-locks, copies values into slice

The billing store is more complex (3 entity types), but the mechanical pattern is identical. A generic store abstraction (Go 1.18+ generics) could eliminate this.

**Fix:** Create a `store.MapStore[T any]` generic in a shared package, exposing `Save`, `Find`, `All`.

---

### 5. `generateUUID()` — Duplicated in Billing Service (Low Confidence)

`billing-service/internal/handler/subscription.go` defines a private `generateUUID()` using `crypto/rand`. The notification service uses `logpipe.GenerateID()` from the shared `logpipe` library for the same purpose.

The billing service reinvents this instead of using `logpipe.GenerateID()` which is already a transitive dependency.

**Fix:** Replace `generateUUID()` in billing-service handlers with `logpipe.GenerateID()`.

---

### 6. Chi Router + Middleware Setup — Copy-Pasted Bootstrap (Low Confidence)

Both `billing-service/cmd/server/main.go` and `notification-service/cmd/server/main.go` wire up chi with the exact same three middleware lines:

```go
r.Use(middleware.RequestID)
r.Use(logpipe.Middleware(logger))
r.Use(middleware.Recoverer)
```

This is minor but consistent with the pattern that bootstrap boilerplate is copy-pasted rather than shared.

---

## User Stories

- **As a developer**, I want to know which repos kodit can see, so I understand the scope of its code intelligence.
- **As a developer**, I want duplicated code identified, so I can refactor shared logic into libraries and reduce maintenance burden.
- **As a maintainer**, I want inconsistencies between near-duplicate implementations highlighted (e.g. structured vs unstructured errors), so I can fix subtle bugs introduced by drift.

## Acceptance Criteria

- [x] All repos visible to kodit are listed with their status (indexed / no commits).
- [x] Each duplication finding states: what is duplicated, where, confidence level, and a recommended fix.
- [x] Findings are ordered from highest to lowest impact.