package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	rl := RateLimit(10, 10, time.Minute)
	hits := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	})
	wrapped := rl(next)

	for range 5 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}
	assert.Equal(t, 5, hits)
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	// 1 rps with burst 1 — second request from the same IP within the
	// same second is rejected.
	rl := RateLimit(1, 1, time.Minute)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := rl(next)

	// First request allowed.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.2:1234"
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Immediate second request from the same IP is denied.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:1234"
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
	assert.Equal(t, "1", rec2.Header().Get("Retry-After"))
}

func TestRateLimit_DifferentIPsIsolated(t *testing.T) {
	rl := RateLimit(1, 1, time.Minute)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := rl(next)

	// IP A allowed.
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "10.0.0.10:1234"
	recA := httptest.NewRecorder()
	wrapped.ServeHTTP(recA, reqA)
	assert.Equal(t, http.StatusOK, recA.Code)

	// IP B still allowed (separate bucket).
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "10.0.0.11:1234"
	recB := httptest.NewRecorder()
	wrapped.ServeHTTP(recB, reqB)
	assert.Equal(t, http.StatusOK, recB.Code)
}

func TestRateLimit_UsesClientIPFromContext(t *testing.T) {
	rl := RateLimit(1, 1, time.Minute)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := rl(next)

	mkReq := func(ip string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), ClientIPKey, ip)
		return r.WithContext(ctx)
	}

	// First request with client IP set in context.
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, mkReq("192.168.1.42"))
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Second request from the same context-derived IP is blocked.
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, mkReq("192.168.1.42"))
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
}
