package http

import (
	"time"
)

// ParseDueDate parses an optional RFC3339 timestamp pointer.
// nil or empty string returns (nil, nil). Invalid format returns a
// non-nil error that handlers map to 400.
func ParseDueDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
