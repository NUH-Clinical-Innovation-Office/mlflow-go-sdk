package mlflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateRun_SendsExperimentAndTags(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if err := c.UpdateRun(context.Background(), "r1", RunStatusFinished); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	if reqBody["status"] != "FINISHED" {
		t.Errorf("status = %v", reqBody["status"])
	}
	if _, ok := reqBody["end_time"]; !ok {
		t.Error("end_time not sent")
	}
}
