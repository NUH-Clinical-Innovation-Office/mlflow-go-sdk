package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders returns middleware that sets baseline security
// response headers on every request.
//
// The default policy is strict (default-src 'none') so a JSON API
// cannot accidentally render as HTML. The /swagger/* path needs the
// Swagger UI assets, so it is exempted from CSP.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if !strings.HasPrefix(r.URL.Path, "/swagger") {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}
