package mlflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// traceTestServer records the StartTraceV3 POST and the traces.json artifact PUT.
// startStatus/putStatus let a test force a non-2xx on either step. The captured
// artifact body is returned via the closure vars the caller passes.
type traceCapture struct {
	startHit    bool
	startBody   string
	putHit      bool
	putPath     string
	putBody     string
	startStatus int
	putStatus   int
}

func traceTestServer(t *testing.T, cap *traceCapture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/3.0/mlflow/traces") && r.Method == http.MethodPost:
			cap.startHit = true
			raw, _ := io.ReadAll(r.Body)
			cap.startBody = string(raw)
			if cap.startStatus != 0 && cap.startStatus != http.StatusOK {
				w.WriteHeader(cap.startStatus)
				_, _ = w.Write([]byte(`{"error_code":"BAD_REQUEST","message":"boom"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			// Echo back a trace with an artifact location tag, as the real server does.
			_, _ = w.Write([]byte(`{"trace":{"trace_info":{"trace_id":"tr-echoed"}}}`))
		case strings.Contains(r.URL.Path, "/mlflow-artifacts/artifacts/") && r.Method == http.MethodPut:
			cap.putHit = true
			cap.putPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			cap.putBody = string(raw)
			if cap.putStatus != 0 && cap.putStatus != http.StatusOK {
				w.WriteHeader(cap.putStatus)
				_, _ = w.Write([]byte(`{"error_code":"ENDPOINT_NOT_FOUND","message":"no artifacts"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func baseParams() LogTraceParams {
	return LogTraceParams{
		ExperimentID: "13",
		Name:         "Patient 1",
		Inputs:       map[string]any{"messages": []any{map[string]string{"role": "user", "content": "hi"}}},
		Outputs:      map[string]any{"messages": []any{map[string]string{"role": "assistant", "content": "yo"}}},
	}
}

func TestLogTrace_PostsStartThenPutsArtifact(t *testing.T) {
	var cap traceCapture
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	p := baseParams()
	p.TraceID = "tr-abc"
	id, err := c.LogTrace(context.Background(), p)
	if err != nil {
		t.Fatalf("LogTrace: %v", err)
	}
	if !cap.startHit {
		t.Error("StartTraceV3 not called")
	}
	if !cap.putHit {
		t.Error("traces.json artifact not PUT")
	}
	// Server echoes tr-echoed, which wins over the client-supplied tr-abc.
	if id != "tr-echoed" {
		t.Errorf("trace id = %q, want tr-echoed (server-echoed)", id)
	}
	// Artifact lands under the (echoed) trace's deterministic path.
	if !strings.HasSuffix(cap.putPath, "/mlflow-artifacts/artifacts/13/traces/tr-echoed/artifacts/traces.json") {
		t.Errorf("artifact path = %s", cap.putPath)
	}
}

func TestLogTrace_ArtifactBodyHasFullUntruncatedInputs(t *testing.T) {
	var cap traceCapture
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	big := strings.Repeat("S", 14000)
	c := New(Options{TrackingURI: srv.URL})
	p := baseParams()
	p.Inputs = map[string]any{"system": big}
	if _, err := c.LogTrace(context.Background(), p); err != nil {
		t.Fatalf("LogTrace: %v", err)
	}

	var art struct {
		Spans []struct {
			Attributes map[string]string `json:"attributes"`
		} `json:"spans"`
	}
	if err := json.Unmarshal([]byte(cap.putBody), &art); err != nil {
		t.Fatalf("artifact body not JSON: %v\n%s", err, cap.putBody[:200])
	}
	if len(art.Spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(art.Spans))
	}
	in := art.Spans[0].Attributes["mlflow.spanInputs"]
	// Attribute value is a JSON string that itself decodes to the inputs; the
	// full 14000-char system prompt must survive untruncated.
	if !strings.Contains(in, big) {
		t.Errorf("spanInputs missing full untruncated input (len=%d)", len(in))
	}
}

func TestLogTrace_MetadataPreviewTruncatedUnderLimit(t *testing.T) {
	var cap traceCapture
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	p := baseParams()
	p.Inputs = map[string]any{"system": strings.Repeat("S", 14000)}
	if _, err := c.LogTrace(context.Background(), p); err != nil {
		t.Fatalf("LogTrace: %v", err)
	}

	var start struct {
		Trace struct {
			TraceInfo struct {
				TraceMetadata map[string]string `json:"trace_metadata"`
			} `json:"trace_info"`
		} `json:"trace"`
	}
	if err := json.Unmarshal([]byte(cap.startBody), &start); err != nil {
		t.Fatalf("start body not JSON: %v", err)
	}
	preview := start.Trace.TraceInfo.TraceMetadata["mlflow.traceInputs"]
	if len([]rune(preview)) > previewMaxChars {
		t.Errorf("preview len = %d runes, want <= %d", len([]rune(preview)), previewMaxChars)
	}
	if preview == "" {
		t.Error("preview is empty")
	}
}

func TestLogTrace_StartFailureSkipsArtifact(t *testing.T) {
	cap := traceCapture{startStatus: http.StatusBadRequest}
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if _, err := c.LogTrace(context.Background(), baseParams()); err == nil {
		t.Fatal("expected error on StartTraceV3 failure")
	}
	if cap.putHit {
		t.Error("artifact PUT attempted after StartTraceV3 failed")
	}
}

func TestLogTrace_ArtifactFailureReturnsError(t *testing.T) {
	cap := traceCapture{putStatus: http.StatusNotImplemented}
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	_, err := c.LogTrace(context.Background(), baseParams())
	if err == nil {
		t.Fatal("expected error on artifact PUT failure")
	}
	if !strings.Contains(err.Error(), "artifact") {
		t.Errorf("error should name the artifact step, got: %v", err)
	}
}

func TestLogTrace_DefaultsTraceIDAndSpanType(t *testing.T) {
	var cap traceCapture
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	p := baseParams() // no TraceID, no SpanType
	id, err := c.LogTrace(context.Background(), p)
	if err != nil {
		t.Fatalf("LogTrace: %v", err)
	}
	if !strings.HasPrefix(id, "tr-") {
		t.Errorf("generated id = %q, want tr- prefix", id)
	}
	if !strings.Contains(cap.putBody, `\"LLM\"`) {
		t.Errorf("default spanType LLM not in artifact: %s", cap.putBody)
	}
}

func TestLogTrace_SetsTraceNameTagAndCustomMetadata(t *testing.T) {
	var cap traceCapture
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	p := baseParams()
	p.Metadata = map[string]string{"mlflow.sourceRun": "run-9", "version": "v1"}
	if _, err := c.LogTrace(context.Background(), p); err != nil {
		t.Fatalf("LogTrace: %v", err)
	}
	if !strings.Contains(cap.startBody, `"mlflow.sourceRun":"run-9"`) {
		t.Errorf("custom metadata missing: %s", cap.startBody)
	}
	if !strings.Contains(cap.startBody, `"mlflow.traceName":"Patient 1"`) {
		t.Errorf("traceName tag missing: %s", cap.startBody)
	}
}

func TestLogTrace_AppliesDefaultTimes(t *testing.T) {
	var cap traceCapture
	srv := traceTestServer(t, &cap)
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	before := time.Now().Add(-time.Second)
	if _, err := c.LogTrace(context.Background(), baseParams()); err != nil {
		t.Fatalf("LogTrace: %v", err)
	}
	var art struct {
		Spans []struct {
			Start int64 `json:"start_time_unix_nano"`
			End   int64 `json:"end_time_unix_nano"`
		} `json:"spans"`
	}
	if err := json.Unmarshal([]byte(cap.putBody), &art); err != nil {
		t.Fatalf("artifact body: %v", err)
	}
	if art.Spans[0].Start < before.UnixNano() || art.Spans[0].End < art.Spans[0].Start {
		t.Errorf("default times invalid: start=%d end=%d", art.Spans[0].Start, art.Spans[0].End)
	}
}
