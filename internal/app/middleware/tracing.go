package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

// Tracing middleware for distributed tracing (optional for this project)
// Can be implemented later if needed with OpenTelemetry or similar

// RequestID adds a unique request ID to each request
func RequestID(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For now, just pass through
			// Can add request ID generation here if needed
			next.ServeHTTP(w, r)
		})
	}
}
