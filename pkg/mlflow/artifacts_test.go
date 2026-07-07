package mlflow

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogArtifact_PutsBytesToProxy(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("run_id")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.LogArtifact(context.Background(), "r1", "metrics.json", []byte(`{"acc":0.9}`))
	if err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/api/2.0/mlflow-artifacts/artifacts/metrics.json") {
		t.Errorf("path = %s", gotPath)
	}
	if gotQuery != "r1" {
		t.Errorf("run_id = %s", gotQuery)
	}
	if gotBody != `{"acc":0.9}` {
		t.Errorf("body = %s", gotBody)
	}
}

func TestLogArtifact_SendsBearerWhenTokenSet(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL, Token: "pat-token"})
	if err := c.LogArtifact(context.Background(), "r1", "metrics.json", []byte(`{"acc":0.9}`)); err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if gotAuth != "Bearer pat-token" {
		t.Errorf("auth = %q, want Bearer pat-token", gotAuth)
	}
}

func TestLogArtifact_EscapesPathSegments(t *testing.T) {
	var gotEscapedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.LogArtifact(context.Background(), "r1", "sub dir/a b.txt", []byte("hi"))
	if err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if !strings.HasSuffix(gotEscapedPath, "/artifacts/sub%20dir/a%20b.txt") {
		t.Errorf("escaped path = %s", gotEscapedPath)
	}
}

func TestLogArtifact_DropsEmptySegments(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	// Leading slash and a doubled slash must not produce "//" in the proxy URL.
	err := c.LogArtifact(context.Background(), "r1", "/a//b.txt", []byte("hi"))
	if err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/artifacts/a/b.txt") {
		t.Errorf("path = %s, want .../artifacts/a/b.txt", gotPath)
	}
	if strings.Contains(gotPath, "//artifacts") || strings.Contains(strings.TrimPrefix(gotPath, "/api/2.0/mlflow-artifacts/artifacts/"), "//") {
		t.Errorf("path contains empty segment: %s", gotPath)
	}
}

func TestLogArtifact_APIErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error_code":"ENDPOINT_NOT_FOUND","message":"serve-artifacts disabled"}`))
	}))
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.LogArtifact(context.Background(), "r1", "x.txt", []byte("hi"))
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T", err)
	}
	if apiErr.StatusCode != http.StatusNotImplemented {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}
