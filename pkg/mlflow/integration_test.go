package mlflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeMLflow is an in-memory stand-in for an MLflow 3.x tracking server. It
// implements just enough of the REST + artifact-proxy surface for the SDK's
// full run lifecycle, and records every request so tests can assert on the
// end-to-end interaction rather than one endpoint at a time.
type fakeMLflow struct {
	mu sync.Mutex

	experiments map[string]string // name -> id
	nextExpID   int

	runs         map[string]string   // run_id -> status
	params       map[string][]Param  // run_id -> params
	metrics      map[string][]Metric // run_id -> metrics
	tags         map[string]map[string]string
	artifacts    map[string][]byte // "run_id/path" -> content
	requestPaths []string          // ordered API paths, for sequencing assertions
}

func newFakeMLflow() *fakeMLflow {
	return &fakeMLflow{
		experiments: map[string]string{},
		runs:        map[string]string{},
		params:      map[string][]Param{},
		metrics:     map[string][]Metric{},
		tags:        map[string]map[string]string{},
		artifacts:   map[string][]byte{},
	}
}

func (f *fakeMLflow) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/2.0/mlflow/experiments/get-by-name", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		name := r.URL.Query().Get("experiment_name")
		f.mu.Lock()
		id, ok := f.experiments[name]
		f.mu.Unlock()
		if !ok {
			writeAPIError(w, http.StatusNotFound, "RESOURCE_DOES_NOT_EXIST", "no experiment named "+name)
			return
		}
		writeJSON(w, map[string]any{"experiment": Experiment{ExperimentID: id, Name: name}})
	})

	mux.HandleFunc("/api/2.0/mlflow/experiments/create", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		var body struct {
			Name string `json:"name"`
		}
		decode(r, &body)
		f.mu.Lock()
		f.nextExpID++
		id := strconv.Itoa(f.nextExpID)
		f.experiments[body.Name] = id
		f.mu.Unlock()
		writeJSON(w, map[string]any{"experiment_id": id})
	})

	mux.HandleFunc("/api/2.0/mlflow/runs/create", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		var body struct {
			ExperimentID string   `json:"experiment_id"`
			Tags         []RunTag `json:"tags"`
		}
		decode(r, &body)
		f.mu.Lock()
		runID := "run-" + strconv.Itoa(len(f.runs)+1)
		f.runs[runID] = "RUNNING"
		f.ensureTags(runID)
		for _, t := range body.Tags {
			f.tags[runID][t.Key] = t.Value
		}
		f.mu.Unlock()
		writeJSON(w, map[string]any{"run": Run{Info: RunInfo{RunID: runID, ExperimentID: body.ExperimentID, Status: "RUNNING"}}})
	})

	mux.HandleFunc("/api/2.0/mlflow/runs/update", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		var body struct {
			RunID  string `json:"run_id"`
			Status string `json:"status"`
		}
		decode(r, &body)
		f.mu.Lock()
		f.runs[body.RunID] = body.Status
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc("/api/2.0/mlflow/runs/log-batch", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		var body struct {
			RunID   string   `json:"run_id"`
			Params  []Param  `json:"params"`
			Metrics []Metric `json:"metrics"`
			Tags    []RunTag `json:"tags"`
		}
		decode(r, &body)
		f.mu.Lock()
		f.params[body.RunID] = append(f.params[body.RunID], body.Params...)
		f.metrics[body.RunID] = append(f.metrics[body.RunID], body.Metrics...)
		f.ensureTags(body.RunID)
		for _, t := range body.Tags {
			f.tags[body.RunID][t.Key] = t.Value
		}
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc("/api/2.0/mlflow/runs/log-metric", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		var body struct {
			RunID     string  `json:"run_id"`
			Key       string  `json:"key"`
			Value     float64 `json:"value"`
			Timestamp int64   `json:"timestamp"`
			Step      int64   `json:"step"`
		}
		decode(r, &body)
		f.mu.Lock()
		f.metrics[body.RunID] = append(f.metrics[body.RunID], Metric{
			Key: body.Key, Value: body.Value, Timestamp: body.Timestamp, Step: body.Step,
		})
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc("/api/2.0/mlflow/runs/set-tag", func(w http.ResponseWriter, r *http.Request) {
		f.record(r.URL.Path)
		var body struct {
			RunID string `json:"run_id"`
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		decode(r, &body)
		f.mu.Lock()
		f.ensureTags(body.RunID)
		f.tags[body.RunID][body.Key] = body.Value
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc("/api/2.0/mlflow-artifacts/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		f.record("artifact-put")
		runID := r.URL.Query().Get("run_id")
		rel := strings.TrimPrefix(r.URL.Path, "/api/2.0/mlflow-artifacts/artifacts/")
		content, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.artifacts[runID+"/"+rel] = content
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func (f *fakeMLflow) ensureTags(runID string) {
	if f.tags[runID] == nil {
		f.tags[runID] = map[string]string{}
	}
}

func (f *fakeMLflow) record(path string) {
	f.mu.Lock()
	f.requestPaths = append(f.requestPaths, path)
	f.mu.Unlock()
}

// TestIntegration_FullRunLifecycle drives the SDK the way a real caller would:
// resolve an experiment, open a run, log a batch, trace a step, upload an
// artifact, and close the run — all against a single fake server, asserting the
// server's final state and the order of calls.
func TestIntegration_FullRunLifecycle(t *testing.T) {
	fake := newFakeMLflow()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	ctx := context.Background()

	exp, err := c.GetOrCreateExperiment(ctx, "integration")
	if err != nil {
		t.Fatalf("GetOrCreateExperiment: %v", err)
	}
	if exp.ExperimentID == "" {
		t.Fatal("empty experiment id")
	}

	run, err := c.CreateRun(ctx, exp.ExperimentID, []RunTag{{Key: "suite", Value: "integration"}})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	runID := run.Info.RunID

	if err := c.LogBatch(ctx, runID,
		[]Param{{Key: "model", Value: "opus-4-8"}},
		[]Metric{{Key: "accuracy", Value: 0.93}},
		[]RunTag{{Key: "phase", Value: "eval"}},
	); err != nil {
		t.Fatalf("LogBatch: %v", err)
	}

	if err := c.Traced(ctx, runID, "inference", true, func(ctx context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("Traced: %v", err)
	}

	if err := c.LogArtifact(ctx, runID, "reports/summary.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}

	if err := c.UpdateRun(ctx, runID, RunStatusFinished); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}

	// Assert final server state reflects every step.
	fake.mu.Lock()
	defer fake.mu.Unlock()

	if got := fake.runs[runID]; got != "FINISHED" {
		t.Errorf("run status = %q, want FINISHED", got)
	}
	if len(fake.params[runID]) != 1 || fake.params[runID][0].Value != "opus-4-8" {
		t.Errorf("params = %+v", fake.params[runID])
	}
	// One metric from LogBatch (accuracy) plus one from Traced (duration_ms).
	metricKeys := map[string]bool{}
	for _, m := range fake.metrics[runID] {
		metricKeys[m.Key] = true
		if m.Timestamp == 0 {
			t.Errorf("metric %q has zero timestamp", m.Key)
		}
	}
	if !metricKeys["accuracy"] || !metricKeys["inference.duration_ms"] {
		t.Errorf("metric keys = %v, want accuracy and inference.duration_ms", metricKeys)
	}
	if fake.tags[runID]["phase"] != "eval" || fake.tags[runID]["suite"] != "integration" {
		t.Errorf("tags = %+v", fake.tags[runID])
	}
	if fake.tags[runID]["trace.inference"] != "ok" {
		t.Errorf("trace tag = %q, want ok", fake.tags[runID]["trace.inference"])
	}
	if got := string(fake.artifacts[runID+"/reports/summary.json"]); got != `{"ok":true}` {
		t.Errorf("artifact = %q", got)
	}

	// GetOrCreate should have made exactly one create call, and the run must be
	// opened before anything is logged against it.
	assertOrder(t, fake.requestPaths,
		"/api/2.0/mlflow/runs/create",
		"/api/2.0/mlflow/runs/log-batch",
	)
	assertOrder(t, fake.requestPaths,
		"/api/2.0/mlflow/runs/log-batch",
		"/api/2.0/mlflow/runs/update",
	)
}

// TestIntegration_GetOrCreateReusesExisting verifies the second lookup after a
// miss does not create a duplicate experiment.
func TestIntegration_GetOrCreateReusesExisting(t *testing.T) {
	fake := newFakeMLflow()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	ctx := context.Background()

	first, err := c.GetOrCreateExperiment(ctx, "reuse")
	if err != nil {
		t.Fatalf("first GetOrCreate: %v", err)
	}
	second, err := c.GetOrCreateExperiment(ctx, "reuse")
	if err != nil {
		t.Fatalf("second GetOrCreate: %v", err)
	}
	if first.ExperimentID != second.ExperimentID {
		t.Errorf("ids differ: %q vs %q", first.ExperimentID, second.ExperimentID)
	}

	var creates int
	fake.mu.Lock()
	for _, p := range fake.requestPaths {
		if p == "/api/2.0/mlflow/experiments/create" {
			creates++
		}
	}
	fake.mu.Unlock()
	if creates != 1 {
		t.Errorf("experiments/create called %d times, want 1", creates)
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, code, msg string) {
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"error_code": code, "message": msg})
}

func decode(r *http.Request, v any) {
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, v)
}

// assertOrder checks that first appears before last in paths.
func assertOrder(t *testing.T, paths []string, first, last string) {
	t.Helper()
	firstIdx, lastIdx := -1, -1
	for i, p := range paths {
		if p == first && firstIdx == -1 {
			firstIdx = i
		}
		if p == last {
			lastIdx = i
		}
	}
	if firstIdx == -1 {
		t.Errorf("%q never called", first)
		return
	}
	if lastIdx == -1 {
		t.Errorf("%q never called", last)
		return
	}
	if firstIdx > lastIdx {
		t.Errorf("%q (idx %d) should come before %q (idx %d)", first, firstIdx, last, lastIdx)
	}
}
