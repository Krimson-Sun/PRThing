package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

// Recovery is a middleware that recovers from panics and returns 500 Internal Server Error
func Recovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered",
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Any("panic", err),
						zap.String("stack", string(debug.Stack())),
					)

					// Return 500 Internal Server Error
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// handlePanic logs panic information
func handlePanic(logger *zap.Logger, r *http.Request) {
	if err := recover(); err != nil {
		logger.Error("Panic recovered in handler",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("error", fmt.Sprintf("%v", err)),
			zap.String("stack", string(debug.Stack())),
		)
	}
}
