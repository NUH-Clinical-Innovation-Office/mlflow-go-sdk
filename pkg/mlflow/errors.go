package mlflow

import "fmt"

// APIError is a non-2xx response from the MLflow REST API.
type APIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("mlflow: %d %s: %s", e.StatusCode, e.ErrorCode, e.Message)
}
