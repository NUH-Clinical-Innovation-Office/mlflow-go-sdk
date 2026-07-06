package mlflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogBatch_StampsMetricTimestamps(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/2.0/mlflow/runs/log-batch" {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.LogBatch(context.Background(), "r1",
		[]Param{{Key: "model", Value: "sonnet-5"}},
		[]Metric{{Key: "acc", Value: 0.9}},
		[]RunTag{{Key: "version", Value: "v1"}},
	)
	if err != nil {
		t.Fatalf("LogBatch: %v", err)
	}
	metrics, _ := reqBody["metrics"].([]any)
	if len(metrics) != 1 {
		t.Fatalf("metrics = %v", reqBody["metrics"])
	}
	m0, _ := metrics[0].(map[string]any)
	if ts, _ := m0["timestamp"].(float64); ts <= 0 {
		t.Errorf("metric timestamp not stamped: %v", m0["timestamp"])
	}
}

func TestLogMetric_SendsValueAndStep(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if err := c.LogMetric(context.Background(), "r1", "acc", 0.91, 2); err != nil {
		t.Fatalf("LogMetric: %v", err)
	}
	if v, _ := reqBody["value"].(float64); v != 0.91 {
		t.Errorf("value = %v", reqBody["value"])
	}
	if s, _ := reqBody["step"].(float64); s != 2 {
		t.Errorf("step = %v", reqBody["step"])
	}
}
