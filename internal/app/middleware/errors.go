package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"pr-service/internal/domain"

	"go.uber.org/zap"
)

// ErrorResponse represents the OpenAPI error response format
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents the error details
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorHandler is a middleware that catches panics and errors, converting them to proper HTTP responses
func ErrorHandler(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a custom response writer to capture status code
			crw := &customResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(crw, r)
		})
	}
}

// WriteErrorResponse writes an error response in OpenAPI format
func WriteErrorResponse(w http.ResponseWriter, err error, logger *zap.Logger) {
	statusCode := domain.GetHTTPStatus(err)
	errorCode := domain.GetErrorCode(err)

	// Log internal errors
	if statusCode == http.StatusInternalServerError {
		logger.Error("Internal server error",
			zap.Error(err),
			zap.Int("status", statusCode),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    string(errorCode),
			Message: err.Error(),
		},
	}

	if errorCode == "" {
		// For unknown errors, use generic message
		response.Error.Code = "INTERNAL_ERROR"
		response.Error.Message = "internal server error"
	}

	json.NewEncoder(w).Encode(response)
}

// MapDomainError maps domain errors to HTTP status codes and error codes
func MapDomainError(err error) (int, domain.ErrorCode) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, domain.ErrorCodeNotFound
	case errors.Is(err, domain.ErrTeamExists):
		return http.StatusBadRequest, domain.ErrorCodeTeamExists
	case errors.Is(err, domain.ErrPRExists):
		return http.StatusConflict, domain.ErrorCodePRExists
	case errors.Is(err, domain.ErrPRMerged):
		return http.StatusConflict, domain.ErrorCodePRMerged
	case errors.Is(err, domain.ErrNotAssigned):
		return http.StatusConflict, domain.ErrorCodeNotAssigned
	case errors.Is(err, domain.ErrNoCandidate):
		return http.StatusConflict, domain.ErrorCodeNoCandidate
	case errors.Is(err, domain.ErrInvalidArgument):
		return http.StatusBadRequest, ""
	default:
		return http.StatusInternalServerError, ""
	}
}

type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (crw *customResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}
