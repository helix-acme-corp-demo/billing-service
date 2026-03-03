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

## Running

```bash
go run ./cmd/server
```

## License

MIT
