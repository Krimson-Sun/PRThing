package middleware

import (
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
					if _, writeErr := w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`)); writeErr != nil {
						logger.Error("failed to write recovery response", zap.Error(writeErr))
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
