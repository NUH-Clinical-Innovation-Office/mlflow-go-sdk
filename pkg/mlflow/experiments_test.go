package mlflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOrCreateExperiment_CreatesWhenMissing(t *testing.T) {
	var createCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/2.0/mlflow/experiments/get-by-name":
			if !createCalled {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error_code": "RESOURCE_DOES_NOT_EXIST", "message": "nope"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"experiment": map[string]string{"experiment_id": "7", "name": "Exp"}})
		case "/api/2.0/mlflow/experiments/create":
			createCalled = true
			_ = json.NewEncoder(w).Encode(map[string]string{"experiment_id": "7"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	exp, err := c.GetOrCreateExperiment(context.Background(), "Exp")
	if err != nil {
		t.Fatalf("GetOrCreateExperiment: %v", err)
	}
	if !createCalled {
		t.Error("expected create to be called")
	}
	if exp.ExperimentID != "7" || exp.Name != "Exp" {
		t.Errorf("exp = %+v", exp)
	}
}

func TestGetOrCreateExperiment_ReturnsExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/2.0/mlflow/experiments/create" {
			t.Error("create should not be called when experiment exists")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"experiment": map[string]string{"experiment_id": "3", "name": "Exp"}})
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	exp, err := c.GetOrCreateExperiment(context.Background(), "Exp")
	if err != nil {
		t.Fatalf("GetOrCreateExperiment: %v", err)
	}
	if exp.ExperimentID != "3" {
		t.Errorf("exp = %+v", exp)
	}
}
