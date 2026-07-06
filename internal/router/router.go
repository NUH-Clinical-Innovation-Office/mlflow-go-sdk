// Package router provides HTTP router setup.
package router

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/your-org/go-backend-template/internal/api"
	"github.com/your-org/go-backend-template/internal/auth"
	"github.com/your-org/go-backend-template/internal/config"
	"github.com/your-org/go-backend-template/internal/logging"
	appmiddleware "github.com/your-org/go-backend-template/internal/middleware"
	"github.com/your-org/go-backend-template/internal/todo"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// HealthResponse is the JSON shape returned by /health. The Database
// field is omitempty so a future health check that does not depend on
// a database (e.g. a sidecar liveness probe) can omit it cleanly.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

// apiServer composes the domain handlers into a single api.ServerInterface.
// Named fields avoid the Handler-name collision between the two handler types.
type apiServer struct {
	todo *todo.Handler
	auth *auth.Handler
}

var _ api.ServerInterface = (*apiServer)(nil)

// New creates a new Chi router with all middleware and routes configured.
//
// Middleware layout:
//   - Root stack: requestID, realIP, logger, Recoverer, securityHeaders, CORS.
//   - /api/v1 group: + 30s timeout + per-IP rate limit.
//
// /health and /swagger intentionally stay on the root group so they are
// never rate-limited or subject to the API timeout. CORS is still
// applied so a browser can hit /health from a different origin.
func New(
	logger *zap.Logger,
	tracer trace.Tracer,
	authSvc appmiddleware.AuthProvider,
	authHandler *auth.Handler,
	todoHandler *todo.Handler,
	corsCfg *config.CORSConfig,
	rateLimitCfg config.RateLimitConfig,
	checkDBHealth func() error,
	swaggerEnabled bool,
	trustedProxies []net.IPNet,
) *chi.Mux {
	r := chi.NewMux()

	r.Use(
		requestIDMiddleware(),
		realIPMiddleware(trustedProxies),
		loggerMiddleware(logger),
		chimiddleware.Recoverer,
		appmiddleware.SecurityHeaders,
		corsMiddleware(corsCfg),
	)

	// Public root routes (not rate-limited, not under the API timeout).
	r.Get("/", rootHandler())
	r.Get("/health", healthHandlerWithDB(checkDBHealth))

	mountSwagger(r, swaggerEnabled)
	if swaggerEnabled {
		mountOpenAPISpec(r)
	}

	apiSrv := &apiServer{todo: todoHandler, auth: authHandler}
	wrapper := api.ServerInterfaceWrapper{
		Handler: apiSrv,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			openAPIValidationErrorHandler(w, err.Error(), http.StatusBadRequest)
		},
	}
	swagger, err := api.GetSwagger()
	if err != nil {
		swagger = nil
	}

	// API routes: timeouts + per-IP rate limit.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(
			timeoutMiddleware(30*time.Second),
			rateLimitMiddleware(rateLimitCfg),
		)
		if swagger != nil {
			r.Use(oapimiddleware.OapiRequestValidatorWithOptions(swagger, &oapimiddleware.Options{
				Options: openapi3filter.Options{
					AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
				},
				ErrorHandler:          openAPIValidationErrorHandler,
				SilenceServersWarning: true,
			}))
		}

		// Public endpoints
		r.Post("/auth/register", wrapper.RegisterUser)
		r.Post("/auth/login", wrapper.LoginUser)

		// Protected endpoints (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.RequireAuth(authSvc))

			r.Get("/todos", wrapper.ListTodos)
			r.Post("/todos", wrapper.CreateTodo)
			r.Get("/todos/{id}", wrapper.GetTodo)
			r.Patch("/todos/{id}", wrapper.UpdateTodo)
			r.Delete("/todos/{id}", wrapper.DeleteTodo)
			r.Get("/me", wrapper.GetCurrentUser)
		})

		// Admin-only endpoints
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.RequireAdmin(authSvc))

			r.Get("/admin/approved-users", wrapper.ListApprovedUsers)
			r.Post("/admin/approved-users", wrapper.CreateApprovedUser)
			r.Post("/admin/approved-users/bulk", wrapper.BulkCreateApprovedUsers)
			r.Delete("/admin/approved-users/{id}", wrapper.DeleteApprovedUser)
		})
	})

	_ = tracer // reserved for future OTel middleware; currently a no-op
	return r
}

// rootHandler returns API version info
func rootHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"version": "1.0.0",
			"status":  "running",
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// healthHandlerWithDB returns health status with database connectivity check
func healthHandlerWithDB(checkDB func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		resp := HealthResponse{Status: "healthy"}
		code := http.StatusOK

		if checkDB != nil {
			if err := checkDB(); err != nil {
				resp.Status = "unhealthy"
				resp.Database = "disconnected"
				code = http.StatusServiceUnavailable
			} else {
				resp.Database = "connected"
			}
		}

		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

// requestIDMiddleware generates a unique request ID for each request
func requestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := appmiddleware.GenerateRequestID()
			ctx := context.WithValue(r.Context(), appmiddleware.RequestIDKey, requestID)
			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// realIPMiddleware extracts the real client IP, honoring TrustedProxies
// when deciding whether to consult X-Forwarded-For.
func realIPMiddleware(trusted []net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := appmiddleware.GetRealIP(r, trusted)
			ctx := context.WithValue(r.Context(), appmiddleware.ClientIPKey, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// loggerMiddleware logs each request with trace context
func loggerMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logging.WithTraceContext(ctx, logger)
			log.Debug("request started",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)
			log.Debug("request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.status),
			)
		})
	}
}

// timeoutMiddleware sets a timeout for the request
func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// corsMiddleware handles Cross-Origin Resource Sharing. The middleware
// always sets Vary: Origin so caches do not mix credentialed responses
// across origins. When AllowCredentials is true, the wildcard origin
// is rejected (CORS spec disallows it). When the origin is not in
// the allow list, no Access-Control-Allow-Origin is set and the
// preflight is rejected with 403.
func corsMiddleware(corsCfg *config.CORSConfig) func(http.Handler) http.Handler {
	methods := strings.Join(corsCfg.AllowedMethods, ", ")
	headers := strings.Join(corsCfg.AllowedHeaders, ", ")
	if methods == "" {
		methods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	}
	if headers == "" {
		headers = "Accept, Authorization, Content-Type"
	}
	maxAge := corsCfg.MaxAge
	if maxAge == 0 {
		maxAge = 3600
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			valid := isValidOrigin(origin, corsCfg.AllowedOrigins)

			if valid {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if corsCfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))

			if r.Method == http.MethodOptions {
				if !valid {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isValidOrigin checks if the origin is in the allowed list. Exact
// match only; the "*" entry is no longer accepted here (it must be
// the sole entry AND AllowCredentials must be false; that case is
// filtered out by config.Validate at startup).
func isValidOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}
	return slices.Contains(allowedOrigins, origin)
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// rateLimitMiddleware converts RateLimitConfig into a per-IP token-bucket
// middleware. When rps <= 0 the middleware is a no-op so dev/test setups
// can opt out without changing the router wiring.
func rateLimitMiddleware(cfg config.RateLimitConfig) func(http.Handler) http.Handler {
	if cfg.Requests <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	ratePerSec := float64(cfg.Requests) / float64(cfg.Duration.Seconds())
	if ratePerSec < 1 {
		ratePerSec = 1
	}
	burst := cfg.Requests
	return appmiddleware.RateLimit(int(ratePerSec), burst, 10*time.Minute)
}
