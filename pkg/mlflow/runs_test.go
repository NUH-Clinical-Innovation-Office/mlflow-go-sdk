package mlflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRun_FetchesArtifactURI(t *testing.T) {
	var gotMethod, gotPath, gotRunID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRunID = r.URL.Query().Get("run_id")
		_ = json.NewEncoder(w).Encode(map[string]any{"run": map[string]any{"info": map[string]string{
			"run_id": "r1", "artifact_uri": "mlflow-artifacts:/11/r1/artifacts",
		}}})
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	run, err := c.GetRun(context.Background(), "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/2.0/mlflow/runs/get" {
		t.Errorf("path = %q", gotPath)
	}
	if gotRunID != "r1" {
		t.Errorf("run_id query = %q", gotRunID)
	}
	if run.Info.ArtifactURI != "mlflow-artifacts:/11/r1/artifacts" {
		t.Errorf("artifact_uri = %q", run.Info.ArtifactURI)
	}
}

func TestCreateRun_SendsExperimentAndTags(t *testing.T) {
	var reqBody map[string]any
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"run": map[string]any{"info": map[string]string{"run_id": "r1", "experiment_id": "9"}}})
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	run, err := c.CreateRun(context.Background(), "9", []RunTag{{Key: "version", Value: "v1"}})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/2.0/mlflow/runs/create" {
		t.Errorf("path = %q", gotPath)
	}
	if run.Info.RunID != "r1" {
		t.Errorf("run id = %q", run.Info.RunID)
	}
	if reqBody["experiment_id"] != "9" {
		t.Errorf("experiment_id = %v", reqBody["experiment_id"])
	}
	if _, ok := reqBody["start_time"]; !ok {
		t.Error("start_time not sent")
	}
	tags, _ := reqBody["tags"].([]any)
	if len(tags) != 1 {
		t.Errorf("tags = %v", reqBody["tags"])
	}
}

func TestUpdateRun_SendsStatusAndEndTime(t *testing.T) {
	var reqBody map[string]any
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if err := c.UpdateRun(context.Background(), "r1", RunStatusFinished); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/2.0/mlflow/runs/update" {
		t.Errorf("path = %q", gotPath)
	}
	if reqBody["status"] != "FINISHED" {
		t.Errorf("status = %v", reqBody["status"])
	}
	if _, ok := reqBody["end_time"]; !ok {
		t.Error("end_time not sent")
	}
}

func TestSetTag(t *testing.T) {
	var reqBody map[string]any
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if err := c.SetTag(context.Background(), "r1", "stage", "prod"); err != nil {
		t.Fatalf("SetTag: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/2.0/mlflow/runs/set-tag" {
		t.Errorf("path = %q", gotPath)
	}
	if reqBody["run_id"] != "r1" {
		t.Errorf("run_id = %v", reqBody["run_id"])
	}
	if reqBody["key"] != "stage" {
		t.Errorf("key = %v", reqBody["key"])
	}
	if reqBody["value"] != "prod" {
		t.Errorf("value = %v", reqBody["value"])
	}
}
