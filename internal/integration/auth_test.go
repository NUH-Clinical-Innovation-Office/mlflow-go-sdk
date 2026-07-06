//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/go-backend-template/internal/auth"
	"github.com/your-org/go-backend-template/internal/config"
	"github.com/your-org/go-backend-template/internal/logging"
	"github.com/your-org/go-backend-template/internal/router"
	"github.com/your-org/go-backend-template/internal/todo"
	"go.opentelemetry.io/otel/trace/noop"
)

// newTestRouter builds a router for tests with permissive defaults.
func newTestRouter(authSvc *auth.Service, authHandler *auth.Handler, todoHandler *todo.Handler) http.Handler {
	logger, _ := logging.New("debug", "console")
	return router.New(
		logger,
		noop.NewTracerProvider().Tracer("test"),
		authSvc,
		authHandler,
		todoHandler,
		&config.CORSConfig{},
		config.RateLimitConfig{Requests: 0}, // disabled
		func() error { return nil },
		false,
		nil,
	)
}

func TestAuthRegister(t *testing.T) {
	pool, _, _, _, _, _, authHandler, todoHandler := setupTestDeps(t)
	defer pool.Close()

	// First create an approved user
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000001'::uuid, 'test@example.com', 'Test')")
	require.NoError(t, err)

	r := newTestRouter(nil, authHandler, todoHandler)

	t.Run("successful registration", func(t *testing.T) {
		body := map[string]string{
			"email":       "test@example.com",
			"password":    "Password123",
			"approved_id": "00000000-0000-0000-0000-000000000001",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "token")
	})

	t.Run("missing approved_id", func(t *testing.T) {
		body := map[string]string{
			"email":    "test@example.com",
			"password": "Password123",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non-existent approved user", func(t *testing.T) {
		body := map[string]string{
			"email":       "test2@example.com",
			"password":    "Password123",
			"approved_id": "00000000-0000-0000-0000-000000000999",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("empty email", func(t *testing.T) {
		body := map[string]string{
			"email":       "",
			"password":    "Password123",
			"approved_id": "00000000-0000-0000-0000-000000000001",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid email format", func(t *testing.T) {
		body := map[string]string{
			"email":       "invalid-email",
			"password":    "Password123",
			"approved_id": "00000000-0000-0000-0000-000000000001",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("weak password (too short)", func(t *testing.T) {
		body := map[string]string{
			"email":       "weak@example.com",
			"password":    "short",
			"approved_id": "00000000-0000-0000-0000-000000000001",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("password without uppercase", func(t *testing.T) {
		body := map[string]string{
			"email":       "nouppercase@example.com",
			"password":    "password123",
			"approved_id": "00000000-0000-0000-0000-000000000001",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("duplicate registration", func(t *testing.T) {
		body := map[string]string{
			"email":       "dupreg3@example.com",
			"password":    "Password123",
			"approved_id": "00000000-0000-0000-0000-000000000001",
		}
		// First registration
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// Duplicate registration - now returns 409
		req = newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "already exists")
	})

	t.Run("invalid uuid format for approved_id", func(t *testing.T) {
		body := map[string]string{
			"email":       "baduuid3@example.com",
			"password":    "Password123",
			"approved_id": "not-a-uuid",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Handler returns BadRequest for invalid UUID format
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty approved_id string", func(t *testing.T) {
		body := map[string]string{
			"email":       "emptyapproved@example.com",
			"password":    "Password123",
			"approved_id": "",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/register", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthLogin(t *testing.T) {
	pool, _, authService, _, _, _, authHandler, todoHandler := setupTestDeps(t)
	defer pool.Close()

	// Create approved user and registered user
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000002'::uuid, 'login@example.com', 'Login')")
	require.NoError(t, err)

	// Register the user
	_, err = authService.Register(ctx, "login@example.com", "Password123", "00000000-0000-0000-0000-000000000002")
	require.NoError(t, err)

	r := newTestRouter(nil, authHandler, todoHandler)

	t.Run("successful login", func(t *testing.T) {
		body := map[string]string{
			"email":    "login@example.com",
			"password": "Password123",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/login", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "token")
	})

	t.Run("invalid password", func(t *testing.T) {
		body := map[string]string{
			"email":    "login@example.com",
			"password": "Wrongpassword1",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/login", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-existent user", func(t *testing.T) {
		body := map[string]string{
			"email":    "notfound@example.com",
			"password": "Password123",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/login", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("empty email", func(t *testing.T) {
		body := map[string]string{
			"email":    "",
			"password": "Password123",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/login", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid email format", func(t *testing.T) {
		body := map[string]string{
			"email":    "invalid-email",
			"password": "Password123",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/login", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty password", func(t *testing.T) {
		body := map[string]string{
			"email":    "login@example.com",
			"password": "",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/auth/login", body)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func newJSONRequest(method, target string, body any) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(mustJSON(body)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestAdminApprovedUsers(t *testing.T) {
	pool, _, authService, _, _, _, authHandler, todoHandler := setupTestDeps(t)
	defer pool.Close()

	ctx := context.Background()

	// Create admin user (use unique email/ID to avoid conflict with seed data)
	_, err := pool.Exec(ctx,
		"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000040'::uuid, 'admin-test@example.com', 'Admin')")
	require.NoError(t, err)

	adminToken, err := authService.Register(ctx, "admin-test@example.com", "Password123", "00000000-0000-0000-0000-000000000040")
	require.NoError(t, err)

	// Assign admin role - get role by name since ID may vary
	_, err = pool.Exec(ctx,
		"INSERT INTO user_roles (user_id, role_id) VALUES ((SELECT id FROM users WHERE email = 'admin-test@example.com'), (SELECT id FROM roles WHERE name = 'admin'))")
	require.NoError(t, err)

	r := newTestRouter(authService, authHandler, todoHandler)

	t.Run("create approved user", func(t *testing.T) {
		body := map[string]string{
			"email":      "newuser@example.com",
			"first_name": "New",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "newuser@example.com")
	})

	t.Run("list approved users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/approved-users", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "email")
	})

	t.Run("unauthorized access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/approved-users", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-admin access returns forbidden", func(t *testing.T) {
		// Create non-admin user
		_, err := pool.Exec(ctx,
			"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000041'::uuid, 'nonadmin@example.com', 'NonAdmin')")
		require.NoError(t, err)

		nonAdminToken, err := authService.Register(ctx, "nonadmin@example.com", "Password123", "00000000-0000-0000-0000-000000000041")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/approved-users", nil)
		req.Header.Set("Authorization", "Bearer "+nonAdminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("create approved user with duplicate email", func(t *testing.T) {
		body := map[string]string{
			"email":      "dupemail@example.com",
			"first_name": "First",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// Try duplicate
		req = newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "already in approved list")
	})

	t.Run("create approved user with invalid email", func(t *testing.T) {
		body := map[string]string{
			"email":      "invalid-email",
			"first_name": "Invalid",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("create approved user with empty email", func(t *testing.T) {
		body := map[string]string{
			"email":      "",
			"first_name": "Empty",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("create approved user with empty first name", func(t *testing.T) {
		body := map[string]string{
			"email":      "noname@example.com",
			"first_name": "",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("delete approved user", func(t *testing.T) {
		// Create user to delete
		_, err := pool.Exec(ctx,
			"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000050'::uuid, 'todelete@example.com', 'ToDelete')")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approved-users/00000000-0000-0000-0000-000000000050", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("delete non-existent approved user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approved-users/00000000-0000-0000-0000-000000000099", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("delete with invalid uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/approved-users/invalid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("bulk create approved users", func(t *testing.T) {
		body := map[string]interface{}{
			"users": []map[string]string{
				{"email": "bulk1@example.com", "first_name": "BulkOne"},
				{"email": "bulk2@example.com", "first_name": "BulkTwo"},
			},
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users/bulk", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("bulk create with empty array", func(t *testing.T) {
		body := map[string]interface{}{
			"users": []map[string]string{},
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users/bulk", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("bulk create with invalid email", func(t *testing.T) {
		body := map[string]interface{}{
			"users": []map[string]string{
				{"email": "invalid-email", "first_name": "Invalid"},
			},
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/admin/approved-users/bulk", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
