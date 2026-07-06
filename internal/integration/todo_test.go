//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoCRUD(t *testing.T) {
	pool, _, authService, _, _, _, authHandler, todoHandler := setupTestDeps(t)
	defer pool.Close()

	// Setup: create approved user and registered user
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000010'::uuid, 'todo@example.com', 'Todo')")
	require.NoError(t, err)

	// Register user
	token, err := authService.Register(ctx, "todo@example.com", "Password123", "00000000-0000-0000-0000-000000000010")
	require.NoError(t, err)

	// Setup router
	r := newTestRouter(authService, authHandler, todoHandler)

	t.Run("create todo", func(t *testing.T) {
		body := map[string]string{
			"title": "Test todo",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/todos", body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "Test todo")
	})

	t.Run("create todo with empty title", func(t *testing.T) {
		body := map[string]string{
			"title": "",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/todos", body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("create todo with very long title", func(t *testing.T) {
		body := map[string]string{
			"title": string(make([]byte, 1001)), // Over 1000 char limit
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/todos", body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("create todo with valid due_date", func(t *testing.T) {
		body := map[string]string{
			"title":    "Todo with due date",
			"due_date": "2026-12-31T23:59:59Z",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/todos", body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		// Response contains user_id - just verify it created successfully
		assert.Contains(t, w.Body.String(), "user_id")
	})

	t.Run("create todo with invalid due_date format", func(t *testing.T) {
		body := map[string]string{
			"title":    "Todo with bad date",
			"due_date": "invalid-date",
		}
		req := newJSONRequest(http.MethodPost, "/api/v1/todos", body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("list todos", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Test todo")
	})

	t.Run("get todo by id", func(t *testing.T) {
		// First create a todo to get its ID
		createBody := map[string]string{"title": "Get test"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Now get it
		req := httptest.NewRequest(http.MethodGet, "/api/v1/todos/"+todoID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Get test")
	})

	t.Run("get non-existent todo", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/todos/00000000-0000-0000-0000-000000000099", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("get todo with invalid uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/todos/invalid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("update todo", func(t *testing.T) {
		// Create a todo first
		createBody := map[string]string{"title": "Update test"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Update it
		updateBody := map[string]interface{}{
			"title":        "Updated title",
			"is_completed": true,
		}
		updateReq := newJSONRequest(http.MethodPatch, "/api/v1/todos/"+todoID, updateBody)
		updateReq.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, updateReq)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Updated title")
	})

	t.Run("update todo preserves omitted field", func(t *testing.T) {
		// Create a todo with description; then PATCH only the title; verify
		// description is preserved (regression test for the PUT-wipes-fields bug).
		createBody := map[string]interface{}{
			"title":       "Original",
			"description": "Keep me",
		}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)
		require.Equal(t, http.StatusCreated, createW.Code)
		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// PATCH with only the title — description should NOT be wiped.
		patchBody := map[string]interface{}{"title": "Renamed"}
		patchReq := newJSONRequest(http.MethodPatch, "/api/v1/todos/"+todoID, patchBody)
		patchReq.Header.Set("Authorization", "Bearer "+token)
		patchW := httptest.NewRecorder()
		r.ServeHTTP(patchW, patchReq)
		require.Equal(t, http.StatusOK, patchW.Code)

		var patched map[string]interface{}
		json.Unmarshal(patchW.Body.Bytes(), &patched)
		assert.Equal(t, "Renamed", patched["title"])
		assert.Equal(t, "Keep me", patched["description"])
	})

	t.Run("update todo with empty title", func(t *testing.T) {
		// Create a todo first
		createBody := map[string]string{"title": "Update test"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Update with empty title
		updateBody := map[string]interface{}{
			"title":        "",
			"is_completed": false,
		}
		updateReq := newJSONRequest(http.MethodPatch, "/api/v1/todos/"+todoID, updateBody)
		updateReq.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, updateReq)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("update non-existent todo", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"title":        "Updated title",
			"is_completed": true,
		}
		updateReq := newJSONRequest(http.MethodPatch, "/api/v1/todos/00000000-0000-0000-0000-000000000099", updateBody)
		updateReq.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, updateReq)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("delete todo", func(t *testing.T) {
		// Create a todo first
		createBody := map[string]string{"title": "Delete test"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Delete it
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/todos/"+todoID, nil)
		delReq.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, delReq)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify it's gone
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/todos/"+todoID, nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getW := httptest.NewRecorder()
		r.ServeHTTP(getW, getReq)
		assert.Equal(t, http.StatusNotFound, getW.Code)
	})

	t.Run("delete non-existent todo", func(t *testing.T) {
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/todos/00000000-0000-0000-0000-000000000099", nil)
		delReq.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, delReq)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("delete with invalid uuid", func(t *testing.T) {
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/todos/invalid-uuid", nil)
		delReq.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, delReq)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("cannot access another user's todo", func(t *testing.T) {
		// Create second user
		_, err := pool.Exec(ctx,
			"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000012'::uuid, 'other@example.com', 'Other')")
		require.NoError(t, err)

		otherToken, err := authService.Register(ctx, "other@example.com", "Password123", "00000000-0000-0000-0000-000000000012")
		require.NoError(t, err)

		// Create todo for first user
		createBody := map[string]string{"title": "My todo"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Try to get other user's todo
		req := httptest.NewRequest(http.MethodGet, "/api/v1/todos/"+todoID, nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("cannot update another user's todo", func(t *testing.T) {
		// Create second user
		_, err := pool.Exec(ctx,
			"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000013'::uuid, 'other2@example.com', 'Other2')")
		require.NoError(t, err)

		otherToken, err := authService.Register(ctx, "other2@example.com", "Password123", "00000000-0000-0000-0000-000000000013")
		require.NoError(t, err)

		// Create todo for first user
		createBody := map[string]string{"title": "My todo"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Try to update other user's todo
		updateBody := map[string]interface{}{
			"title":        "Hacked title",
			"is_completed": true,
		}
		updateReq := newJSONRequest(http.MethodPatch, "/api/v1/todos/"+todoID, updateBody)
		updateReq.Header.Set("Authorization", "Bearer "+otherToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, updateReq)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("cannot delete another user's todo", func(t *testing.T) {
		// Create second user
		_, err := pool.Exec(ctx,
			"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000014'::uuid, 'other3@example.com', 'Other3')")
		require.NoError(t, err)

		otherToken, err := authService.Register(ctx, "other3@example.com", "Password123", "00000000-0000-0000-0000-000000000014")
		require.NoError(t, err)

		// Create todo for first user
		createBody := map[string]string{"title": "My todo"}
		createReq := newJSONRequest(http.MethodPost, "/api/v1/todos", createBody)
		createReq.Header.Set("Authorization", "Bearer "+token)
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)

		var created map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &created)
		todoID := created["id"].(string)

		// Try to delete other user's todo
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/todos/"+todoID, nil)
		delReq.Header.Set("Authorization", "Bearer "+otherToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, delReq)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthGetMe(t *testing.T) {
	pool, _, authService, _, _, _, authHandler, _ := setupTestDeps(t)
	defer pool.Close()

	// Setup user
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		"INSERT INTO approved_users (id, email, first_name) VALUES ('00000000-0000-0000-0000-000000000011'::uuid, 'me@example.com', 'Me')")
	require.NoError(t, err)

	token, err := authService.Register(ctx, "me@example.com", "Password123", "00000000-0000-0000-0000-000000000011")
	require.NoError(t, err)

	r := newTestRouter(authService, authHandler, nil)

	t.Run("get current user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "me@example.com")
	})
}
