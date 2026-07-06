package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// RateLimit returns a middleware that applies a per-client-IP token-bucket
// rate limit. The first request from a new IP gets a fresh bucket sized to
// burst; subsequent requests are limited to rps tokens per second.
//
// Concurrency: a sync.Map holds the limiter per IP so the hot path
// (Allow() check) is lock-free for repeat callers. Limiter creation
// happens under a one-time per-IP lock inside LoadOrStore. Map size is
// bounded by a periodic eviction sweep (amortized — every N requests
// or every T seconds, whichever comes first) so a flood of unique IPs
// does not cost O(N) work on the request hot path.
func RateLimit(rps, burst int, idleTimeout time.Duration) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = rps
	}
	if idleTimeout <= 0 {
		idleTimeout = 10 * time.Minute
	}

	type entry struct {
		limiter  *rate.Limiter
		lastSeen atomic.Int64 // unix nano; atomic for lock-free updates
	}

	var (
		buckets    sync.Map // ip string -> *entry
		opsSince   atomic.Int64
		lastSweep  atomic.Int64
		sweepEvery = int64(4096)
	)

	now := time.Now
	mkEntry := func() *entry {
		e := &entry{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
		e.lastSeen.Store(now().UnixNano())
		return e
	}

	// sweep deletes entries idle longer than idleTimeout. Called from a
	// goroutine so it never blocks the request hot path.
	sweep := func() {
		cutoff := now().Add(-idleTimeout).UnixNano()
		buckets.Range(func(k, v any) bool {
			e, ok := v.(*entry)
			if ok && e.lastSeen.Load() < cutoff {
				buckets.Delete(k)
			}
			return true
		})
		lastSweep.Store(now().UnixNano())
		opsSince.Store(0)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _ := r.Context().Value(ClientIPKey).(string) //nolint:errcheck
			if ip == "" {
				ip = GetRealIP(r, nil)
			}

			// Amortize eviction: every sweepEvery requests OR every 60s.
			if opsSince.Add(1) >= sweepEvery || now().UnixNano()-lastSweep.Load() > int64(time.Minute) {
				go sweep()
			}

			actual, _ := buckets.LoadOrStore(ip, mkEntry())
			e, ok := actual.(*entry)
			if !ok {
				e = mkEntry()
			}
			e.lastSeen.Store(now().UnixNano())

			if !e.limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"detail":"rate limit exceeded"}`)) //nolint:errcheck
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
