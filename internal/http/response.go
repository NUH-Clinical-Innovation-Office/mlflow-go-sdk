// Package http provides HTTP response helpers.
package http

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// RespondJSON writes a JSON response
func RespondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// RespondError writes a JSON error response
func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"detail": message})
}

// RespondInternalError logs the underlying error and returns a generic 500
// to the client. msg is the user-facing message; err is the cause to log.
// Use this instead of inline RespondError(500, "internal server error").
func RespondInternalError(w http.ResponseWriter, logger *zap.Logger, err error, msg string) {
	if logger != nil {
		logger.Error(msg, zap.Error(err))
	}
	RespondError(w, http.StatusInternalServerError, "internal server error")
}

// ErrorResponse is the standard error body returned by all endpoints.
// Used as the response type in Swagger @Failure annotations.
type ErrorResponse struct {
	Detail string `json:"detail"`
}
