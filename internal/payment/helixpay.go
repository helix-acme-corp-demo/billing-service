package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/helix-acme-corp-demo/retryx"
)

// PaymentError is returned when HelixPay declines a payment (4xx). It is not
// retryable.
type PaymentError struct {
	StatusCode int
	Message    string
}

func (e *PaymentError) Error() string {
	return fmt.Sprintf("helixpay payment failed (status %d): %s", e.StatusCode, e.Message)
}

// chargeRequest is the JSON body sent to the HelixPay charge endpoint.
type chargeRequest struct {
	Token     string `json:"token"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Reference string `json:"reference"`
}

// chargeResponse is the JSON body returned by the HelixPay charge endpoint.
type chargeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Client is a HelixPay API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new HelixPay Client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Charge calls the HelixPay API to charge the given token for the specified
// amount. Transient errors (network failures, 5xx responses) are retried with
// exponential backoff. Permanent failures (4xx) are returned immediately as a
// *PaymentError without retrying.
func (c *Client) Charge(ctx context.Context, token string, amount int64, currency, invoiceID string) error {
	body, err := json.Marshal(chargeRequest{
		Token:     token,
		Amount:    amount,
		Currency:  currency,
		Reference: invoiceID,
	})
	if err != nil {
		return fmt.Errorf("helixpay: failed to marshal request: %w", err)
	}

	return retryx.Do(ctx, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/charge", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("helixpay: failed to build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error — retryable.
			return fmt.Errorf("helixpay: request error: %w", err)
		}
		defer resp.Body.Close()

		var result chargeResponse
		_ = json.NewDecoder(resp.Body).Decode(&result)

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// Client error — permanent, do not retry.
			msg := result.Message
			if msg == "" {
				msg = http.StatusText(resp.StatusCode)
			}
			return &PaymentError{StatusCode: resp.StatusCode, Message: msg}
		}

		if resp.StatusCode >= 500 {
			// Server error — transient, retryable.
			return fmt.Errorf("helixpay: server error (status %d)", resp.StatusCode)
		}

		return nil
	},
		retryx.WithMaxAttempts(4),
		retryx.WithBaseDelay(200*time.Millisecond),
		retryx.WithMaxDelay(5*time.Second),
		retryx.WithRetryIf(func(err error) bool {
			// Do not retry permanent payment errors.
			_, isPermanent := err.(*PaymentError)
			return !isPermanent
		}),
	)
}
