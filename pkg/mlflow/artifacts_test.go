package mlflow

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// artifactTestServer serves runs/get (so LogArtifact can resolve the run's
// artifact_uri) and forwards any other request to onProxy. artifactURI is the
// value returned for the run; pass "" to omit it.
func artifactTestServer(t *testing.T, artifactURI string, onProxy http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/2.0/mlflow/runs/get") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"run":{"info":{"run_id":"r1","artifact_uri":"` + artifactURI + `"}}}`))
			return
		}
		onProxy(w, r)
	}))
}

func TestLogArtifact_PutsBytesUnderRunArtifactRoot(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	srv := artifactTestServer(t, "mlflow-artifacts:/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("run_id")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.LogArtifact(context.Background(), "r1", "metrics.json", []byte(`{"acc":0.9}`))
	if err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s", gotMethod)
	}
	// Must land under the run's artifact subtree, not the bare server root.
	if !strings.HasSuffix(gotPath, "/api/2.0/mlflow-artifacts/artifacts/11/abc123/artifacts/metrics.json") {
		t.Errorf("path = %s", gotPath)
	}
	if gotQuery != "r1" {
		t.Errorf("run_id = %s", gotQuery)
	}
	if gotBody != `{"acc":0.9}` {
		t.Errorf("body = %s", gotBody)
	}
}

func TestLogArtifact_HandlesAuthorityInArtifactURI(t *testing.T) {
	var gotPath string
	srv := artifactTestServer(t, "mlflow-artifacts://host/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if err := c.LogArtifact(context.Background(), "r1", "metrics.json", []byte("x")); err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/artifacts/11/abc123/artifacts/metrics.json") {
		t.Errorf("path = %s", gotPath)
	}
}

func TestLogArtifact_FallsBackToBarePathForNonProxyURI(t *testing.T) {
	var gotPath string
	srv := artifactTestServer(t, "s3://bucket/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	if err := c.LogArtifact(context.Background(), "r1", "metrics.json", []byte("x")); err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/api/2.0/mlflow-artifacts/artifacts/metrics.json") {
		t.Errorf("path = %s", gotPath)
	}
}

func TestLogArtifact_SendsBearerWhenTokenSet(t *testing.T) {
	var gotAuth, gotUserAgent, gotClientVersion string
	srv := artifactTestServer(t, "mlflow-artifacts:/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUserAgent = r.Header.Get("User-Agent")
		gotClientVersion = r.Header.Get("X-MLflow-Client-Version")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL, Token: "pat-token"})
	if err := c.LogArtifact(context.Background(), "r1", "metrics.json", []byte(`{"acc":0.9}`)); err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if gotAuth != "Bearer pat-token" {
		t.Errorf("auth = %q, want Bearer pat-token", gotAuth)
	}
	if gotUserAgent != userAgent {
		t.Errorf("user-agent = %q, want %q", gotUserAgent, userAgent)
	}
	if gotClientVersion != clientVersion {
		t.Errorf("client version = %q, want %q", gotClientVersion, clientVersion)
	}
}

func TestLogArtifact_EscapesPathSegments(t *testing.T) {
	var gotEscapedPath string
	srv := artifactTestServer(t, "mlflow-artifacts:/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	err := c.LogArtifact(context.Background(), "r1", "sub dir/a b.txt", []byte("hi"))
	if err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if !strings.HasSuffix(gotEscapedPath, "/11/abc123/artifacts/sub%20dir/a%20b.txt") {
		t.Errorf("escaped path = %s", gotEscapedPath)
	}
}

func TestLogArtifact_DropsEmptySegments(t *testing.T) {
	var gotPath string
	srv := artifactTestServer(t, "mlflow-artifacts:/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := New(Options{TrackingURI: srv.URL})
	// Leading slash and a doubled slash must not produce "//" in the proxy URL.
	err := c.LogArtifact(context.Background(), "r1", "/a//b.txt", []byte("hi"))
	if err != nil {
		t.Fatalf("LogArtifact: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/11/abc123/artifacts/a/b.txt") {
		t.Errorf("path = %s, want .../artifacts/a/b.txt", gotPath)
	}
	if strings.Contains(strings.TrimPrefix(gotPath, "/api/2.0/mlflow-artifacts/artifacts/"), "//") {
		t.Errorf("path contains empty segment: %s", gotPath)
	}
}

func TestLogArtifact_APIErrorOnNon2xx(t *testing.T) {
	srv := artifactTestServer(t, "mlflow-artifacts:/11/abc123/artifacts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error_code":"ENDPOINT_NOT_FOUND","message":"serve-artifacts disabled"}`))
	})
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
