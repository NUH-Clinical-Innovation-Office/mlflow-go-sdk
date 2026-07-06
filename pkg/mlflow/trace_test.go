package mlflow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTraced_DisabledIsPassthrough(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	var ran bool
	err := c.Traced(context.Background(), "r1", "extraction", false, func(ctx context.Context) error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("Traced: %v", err)
	}
	if !ran {
		t.Error("fn did not run")
	}
	if hits != 0 {
		t.Errorf("disabled Traced made %d MLflow calls, want 0", hits)
	}
}

func TestTraced_EnabledLogsDurationAndOKTag(t *testing.T) {
	var loggedMetric, loggedTagKey, loggedTagValue string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		switch {
		case strings.HasSuffix(r.URL.Path, "runs/log-metric"):
			loggedMetric, _ = body["key"].(string)
		case strings.HasSuffix(r.URL.Path, "runs/set-tag"):
			loggedTagKey, _ = body["key"].(string)
			loggedTagValue, _ = body["value"].(string)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.Traced(context.Background(), "r1", "extraction", true, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Traced: %v", err)
	}
	if loggedMetric != "extraction.duration_ms" {
		t.Errorf("metric key = %q, want extraction.duration_ms", loggedMetric)
	}
	if loggedTagKey != "trace.extraction" || loggedTagValue != "ok" {
		t.Errorf("tag = %q/%q, want trace.extraction/ok", loggedTagKey, loggedTagValue)
	}
}

func TestTraced_EnabledPropagatesErrorAndTagsError(t *testing.T) {
	var loggedTagValue string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if strings.HasSuffix(r.URL.Path, "runs/set-tag") {
			loggedTagValue, _ = body["value"].(string)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	sentinel := errors.New("boom")
	err := c.Traced(context.Background(), "r1", "extraction", true, func(ctx context.Context) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("Traced err = %v, want sentinel", err)
	}
	if loggedTagValue != "error" {
		t.Errorf("tag value = %q, want error", loggedTagValue)
	}
}
