package mlflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSON_SendsBearerAndDecodesBody(t *testing.T) {
	var gotAuth, gotPath, gotMethod, gotUserAgent, gotClientVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotUserAgent = r.Header.Get("User-Agent")
		gotClientVersion = r.Header.Get("X-MLflow-Client-Version")
		_ = json.NewEncoder(w).Encode(map[string]string{"experiment_id": "42"})
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL, Token: "tok"})
	var out struct {
		ExperimentID string `json:"experiment_id"`
	}
	if err := c.doJSON(context.Background(), http.MethodPost, "experiments/create", map[string]string{"name": "x"}, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q, want Bearer tok", gotAuth)
	}
	if gotPath != "/api/2.0/mlflow/experiments/create" {
		t.Errorf("path = %q", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q", gotMethod)
	}
	if gotUserAgent != userAgent {
		t.Errorf("user-agent = %q, want %q", gotUserAgent, userAgent)
	}
	if gotClientVersion != clientVersion {
		t.Errorf("client version = %q, want %q", gotClientVersion, clientVersion)
	}
	if out.ExperimentID != "42" {
		t.Errorf("experiment_id = %q", out.ExperimentID)
	}
}

func TestDoJSON_OmitsBearerWhenTokenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]string{"experiment_id": "42"})
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	var out struct {
		ExperimentID string `json:"experiment_id"`
	}
	if err := c.doJSON(context.Background(), http.MethodPost, "experiments/create", map[string]string{"name": "x"}, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("auth = %q, want empty", gotAuth)
	}
}

func TestDoJSON_DecodesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error_code": "RESOURCE_DOES_NOT_EXIST",
			"message":    "no such experiment",
		})
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.doJSON(context.Background(), http.MethodGet, "experiments/get", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 404 || apiErr.ErrorCode != "RESOURCE_DOES_NOT_EXIST" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}
