// Package middleware provides HTTP middleware functions.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/your-org/go-backend-template/internal/domain"
	http2 "github.com/your-org/go-backend-template/internal/http"
)

type contextKey string

const (
	CurrentUserKey contextKey = "current_user"
	RequestIDKey   contextKey = "request_id"
	ClientIPKey    contextKey = "client_ip"
)

// GenerateRequestID returns a hex-encoded 128-bit random ID.
// On the (essentially impossible) failure of crypto/rand we fall back
// to a time-derived ID so the request is still observable in logs.
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%016x", time.Now().UnixNano())
}

// GetRealIP extracts the real client IP from X-Forwarded-For, X-Real-IP,
// or RemoteAddr. Proxy headers are only honored when r.RemoteAddr falls
// inside one of the trusted CIDRs (e.g. a load balancer). Port is
// stripped from RemoteAddr for consistency with the proxy-header paths.
func GetRealIP(r *http.Request, trustedProxies []net.IPNet) string {
	remote := remoteHost(r.RemoteAddr)
	if remote != "" && len(trustedProxies) > 0 {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			remote = host
		}
		if ip := net.ParseIP(remote); ip != nil {
			for _, cidr := range trustedProxies {
				if cidr.Contains(ip) {
					if h := r.Header.Get("X-Forwarded-For"); h != "" {
						parts := strings.Split(h, ",")
						return strings.TrimSpace(parts[0])
					}
					if h := r.Header.Get("X-Real-IP"); h != "" {
						return h
					}
					return remote
				}
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// AuthProvider defines the interface for auth service operations
type AuthProvider interface {
	GetUserFromToken(ctx context.Context, token string) (*domain.User, error)
}

// RequireAuth validates JWT Bearer token and injects user into context
func RequireAuth(authSvc AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				http2.RespondError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			user, err := authSvc.GetUserFromToken(r.Context(), token)
			if err != nil || user == nil {
				http2.RespondError(w, http.StatusUnauthorized, "could not validate credentials")
				return
			}

			ctx := context.WithValue(r.Context(), CurrentUserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin wraps RequireAuth and checks for admin role
func RequireAdmin(authSvc AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return RequireAuth(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http2.RespondError(w, http.StatusUnauthorized, "could not validate credentials")
				return
			}

			if !user.HasRole("admin") {
				http2.RespondError(w, http.StatusForbidden, "admin role required")
				return
			}

			next.ServeHTTP(w, r)
		}))
	}
}

// UserFromContext retrieves the current user from context
func UserFromContext(ctx context.Context) *domain.User {
	u, ok := ctx.Value(CurrentUserKey).(*domain.User)
	if !ok {
		return nil
	}
	return u
}

// RequestIDFromContext retrieves the request ID from context. The boolean
// is false if the context has no request ID (so callers can distinguish
// "no id" from "empty id").
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(RequestIDKey).(string)
	return id, ok
}

// ClientIPFromContext retrieves the client IP from context
func ClientIPFromContext(ctx context.Context) string {
	ip, ok := ctx.Value(ClientIPKey).(string)
	if !ok {
		return ""
	}
	return ip
}

// extractBearerToken extracts Bearer token from Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return parts[1]
}
