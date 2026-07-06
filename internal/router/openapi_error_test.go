package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAPIValidationErrorHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	openAPIValidationErrorHandler(rec, "title is required", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["detail"] != "title is required" {
		t.Fatalf("detail = %q, want %q", body["detail"], "title is required")
	}
}
