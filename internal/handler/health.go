package handler

import (
	"net/http"

	"github.com/helix-acme-corp-demo/envelope"
)

// Health returns an HTTP handler that reports service health.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envelope.Write(w, envelope.OK(map[string]string{"status": "healthy"}))
	}
}
