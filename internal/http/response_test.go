package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		data           interface{}
		expectedStatus int
	}{
		{"string response", http.StatusOK, "hello", http.StatusOK},
		{"map response", http.StatusOK, map[string]string{"key": "value"}, http.StatusOK},
		{"struct response", http.StatusCreated, struct{ Name string }{Name: "Test"}, http.StatusCreated},
		{"nil response", http.StatusOK, nil, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondJSON(w, tt.status, tt.data)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		})
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		message        string
		expectedStatus int
	}{
		{"bad request", http.StatusBadRequest, "invalid input", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized, "unauthorized", http.StatusUnauthorized},
		{"not found", http.StatusNotFound, "resource not found", http.StatusNotFound},
		{"internal error", http.StatusInternalServerError, "internal error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondError(w, tt.status, tt.message)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			body := w.Body.String()
			assert.Contains(t, body, tt.message)
		})
	}
}
