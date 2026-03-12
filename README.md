# billing-service

Billing and subscription management microservice for Acme Corp. Handles subscriptions, usage metering, and invoice generation.

## Architecture

This service uses several internal Acme libraries:
- **[retryx](https://github.com/helix-acme-corp-demo/retryx)** — Retry with exponential backoff for payment processing
- **[logpipe](https://github.com/helix-acme-corp-demo/logpipe)** — Structured JSON logging with correlation IDs
- **[cachex](https://github.com/helix-acme-corp-demo/cachex)** — In-memory caching for subscription lookups
- **[envelope](https://github.com/helix-acme-corp-demo/envelope)** — Standardized API response formatting

## API Endpoints

### Health Check
```
GET /health
```

### Subscriptions
```bash
# Create subscription
curl -X POST http://localhost:8082/subscriptions \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user-123","plan":"pro"}'

# List subscriptions
curl http://localhost:8082/subscriptions

# Get subscription
curl http://localhost:8082/subscriptions/{id}

# Cancel subscription
curl -X POST http://localhost:8082/subscriptions/{id}/cancel
```

### Usage
```bash
# Record usage
curl -X POST http://localhost:8082/usage \
  -H "Content-Type: application/json" \
  -d '{"subscription_id":"sub-123","metric":"api_calls","quantity":150}'

# List usage for subscription
curl http://localhost:8082/usage?subscription_id=sub-123
```

### Invoices
```bash
# Generate invoice
curl -X POST http://localhost:8082/invoices/generate \
  -H "Content-Type: application/json" \
  -d '{"subscription_id":"sub-123"}'

# Get invoice
curl http://localhost:8082/invoices/{id}

# List invoices
curl http://localhost:8082/invoices?subscription_id=sub-123
```

## Plans

| Plan | Base Price | API Calls | Storage |
|------|-----------|-----------|---------|
| Free | $0/mo | 1,000/mo | 1 GB |
| Pro | $49.99/mo | 50,000/mo | 50 GB |
| Enterprise | $199.99/mo | Unlimited | 500 GB |

## Payment Providers

The billing service uses a pluggable payment provider abstraction. You can switch providers via the `PAYMENT_PROVIDER` environment variable.

### Available Providers

| Provider | Value | Description |
|----------|-------|-------------|
| Stub | `stub` (default) | No-op provider for development and testing. Returns fake IDs and always succeeds. |
| Stripe | `stripe` | Stripe integration (skeleton — requires implementation of SDK calls). |

### Configuration

```bash
# Use the stub provider (default — no config needed)
PAYMENT_PROVIDER=stub go run ./cmd/server

# Use Stripe (requires API key)
PAYMENT_PROVIDER=stripe STRIPE_API_KEY=sk_test_... go run ./cmd/server
```

### Adding a New Provider

1. Create a new file in `internal/provider/` (e.g., `braintree.go`).
2. Implement the `provider.PaymentProvider` interface.
3. Register it in an `init()` function:
   ```go
   func init() {
       Register("braintree", func(cfg map[string]string) (PaymentProvider, error) {
           return NewBraintree(cfg["merchant_id"], cfg["api_key"])
       })
   }
   ```
4. Add any required environment variables to `config/config.go` in the `Load()` function.
5. Set `PAYMENT_PROVIDER=braintree` to activate it.

## Running

```bash
go run ./cmd/server
```

## License

MIT
