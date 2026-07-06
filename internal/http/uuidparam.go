package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ParseUUIDParam reads a chi URL parameter and parses it as a UUID.
// Returns (uuid.Nil, false) when the parameter is missing or invalid.
// Handlers map false to a 400 response.
func ParseUUIDParam(r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
