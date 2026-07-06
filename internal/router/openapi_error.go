package router

import (
	"net/http"

	http2 "github.com/your-org/go-backend-template/internal/http"
)

// openAPIValidationErrorHandler renders OapiRequestValidator failures using
// the project's standard {"detail": ...} error body.
func openAPIValidationErrorHandler(w http.ResponseWriter, message string, statusCode int) {
	http2.RespondError(w, statusCode, message)
}
