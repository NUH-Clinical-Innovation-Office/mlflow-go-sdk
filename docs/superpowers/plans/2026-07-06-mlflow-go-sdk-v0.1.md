# mlflow-go-sdk v0.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A minimal, reusable Go client for the MLflow 3.14 tracking REST API, covering exactly the endpoints the histopath eval (and sibling NUH projects) need: experiments, runs, params/metrics/tags, and proxied artifact upload.

**Architecture:** A single `pkg/mlflow` package. One `Client` holds the tracking URI, optional bearer token, and an `*http.Client`. Each endpoint group is one file with small methods that marshal a request struct, POST/GET JSON against `/api/2.0/mlflow/...`, and decode a typed response. Errors decode MLflow's `{error_code, message}` body into a typed `APIError`. Artifact upload uses the tracking server's artifact proxy. All tests run against `httptest.Server` — no live MLflow required. A `example/` binary is the live smoke check.

**Tech Stack:** Go 1.26, stdlib only (`net/http`, `encoding/json`, `mime/multipart`, `net/url`). Test with `testing` + `net/http/httptest`. No third-party deps in the core package.

## Global Constraints

- Module path: `github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk` (copied verbatim into `go.mod`).
- Go version floor: `go 1.26.4` (from the seeded `go.mod`).
- Core package `pkg/mlflow` imports **stdlib only** — no third-party dependencies.
- Commit messages follow Conventional Commits (repo enforces `commitlint` via lefthook `commit-msg`): `feat:`, `test:`, `refactor:`, `chore:`, `docs:`.
- Lint is strict (`.golangci.yml`): `errcheck`, `bodyclose`, `gocyclo`, `goconst`, `gocritic` all on. Always close `resp.Body`; check every error.
- All REST paths are under `/api/2.0/mlflow/` (tracking) and `/api/2.0/mlflow-artifacts/artifacts` (artifact proxy). MLflow keeps these stable across 2.x/3.x.
- Every exported symbol has a doc comment (godoc).

---

## Task 0: Strip the repo to a clean library module

**Files:**
- Delete: `api/`, `cmd/`, `internal/`, `migrations/`, `sql/`, `sqlc.yaml`, `Dockerfile`, `docker-compose.yml`, `.env.example`
- Modify: `go.mod` (reset module path + drop all requires), `Makefile` (trim to library targets), `README.md`
- Delete: `go.sum` (regenerated empty — no deps yet)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: a compiling empty module `github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk` with `make test`, `make lint`, `make fmt` targets.

- [ ] **Step 1: Remove backend scaffolding**

```bash
cd /Users/yongchenglow/nuh/mlflow-go-sdk
git rm -r api cmd internal migrations sql sqlc.yaml Dockerfile docker-compose.yml .env.example go.sum
```

- [ ] **Step 2: Reset `go.mod`**

Overwrite `go.mod` with:

```
module github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk

go 1.26.4
```

- [ ] **Step 3: Trim the `Makefile`**

Overwrite `Makefile` with:

```make
.PHONY: help test test-coverage lint fmt vet tidy clean example

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

test: ## Run all tests
	@go test -race ./...

test-coverage: ## Run tests with coverage report
	@go test -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	@golangci-lint run ./...

fmt: ## Format code
	@gofmt -w .

vet: ## Run go vet
	@go vet ./...

tidy: ## Tidy go modules
	@go mod tidy

clean: ## Clean build artifacts
	@rm -f coverage.out coverage.html

example: ## Run the live MLflow smoke example (needs MLFLOW_TRACKING_URI)
	@go run ./example
```

- [ ] **Step 4: Replace `README.md` with a stub**

```markdown
# mlflow-go-sdk

A minimal Go client for the [MLflow](https://mlflow.org) 3.x tracking REST API.

Covers experiments, runs, params/metrics/tags, and proxied artifact upload —
the subset needed by NUH evaluation pipelines. Not a comprehensive MLflow SDK.

```go
c := mlflow.New(mlflow.Options{TrackingURI: "http://localhost:5000"})
exp, _ := c.GetOrCreateExperiment(ctx, "My Experiment")
run, _ := c.CreateRun(ctx, exp.ExperimentID, nil)
_ = c.LogMetric(ctx, run.Info.RunID, "accuracy", 0.91, 0)
_ = c.UpdateRun(ctx, run.Info.RunID, mlflow.RunStatusFinished)
```

See `example/` for a full runnable smoke test.
```

- [ ] **Step 5: Verify the module compiles and is empty**

Run: `go build ./... && go vet ./...`
Expected: no output, exit 0 (nothing to build yet, but module resolves).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: strip backend template scaffolding to clean library module"
```

---

## Task 1: Client core + typed API errors

**Files:**
- Create: `pkg/mlflow/client.go`
- Create: `pkg/mlflow/errors.go`
- Create: `pkg/mlflow/client_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Options struct { TrackingURI string; Token string; HTTPClient *http.Client }`
  - `type Client struct { ... }`
  - `func New(opts Options) *Client`
  - `func (c *Client) doJSON(ctx context.Context, method, apiPath string, body any, out any) error` — unexported; POSTs/GETs JSON to `TrackingURI + "/api/2.0/mlflow/" + apiPath`, sets `Authorization: Bearer` when `Token != ""`, decodes `out` on 2xx, returns `*APIError` on non-2xx.
  - `type APIError struct { StatusCode int; ErrorCode string; Message string }` with `func (e *APIError) Error() string`.

- [ ] **Step 1: Write the failing test**

Create `pkg/mlflow/client_test.go`:

```go
package mlflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSON_SendsBearerAndDecodesBody(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
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
	if out.ExperimentID != "42" {
		t.Errorf("experiment_id = %q", out.ExperimentID)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mlflow/ -run TestDoJSON -v`
Expected: FAIL — `undefined: New`, `undefined: Options`, `undefined: APIError`.

- [ ] **Step 3: Write `errors.go`**

Create `pkg/mlflow/errors.go`:

```go
package mlflow

import "fmt"

// APIError is a non-2xx response from the MLflow REST API.
type APIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("mlflow: %d %s: %s", e.StatusCode, e.ErrorCode, e.Message)
}
```

- [ ] **Step 4: Write `client.go`**

Create `pkg/mlflow/client.go`:

```go
// Package mlflow is a minimal client for the MLflow 3.x tracking REST API.
package mlflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiPrefix = "/api/2.0/mlflow/"

// Options configures a Client.
type Options struct {
	// TrackingURI is the MLflow server base URL, e.g. http://localhost:5000.
	TrackingURI string
	// Token, when non-empty, is sent as an Authorization: Bearer header.
	Token string
	// HTTPClient overrides the default client (30s timeout) when non-nil.
	HTTPClient *http.Client
}

// Client talks to an MLflow tracking server.
type Client struct {
	trackingURI string
	token       string
	http        *http.Client
}

// New returns a Client for the given options.
func New(opts Options) *Client {
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		trackingURI: strings.TrimRight(opts.TrackingURI, "/"),
		token:       opts.Token,
		http:        hc,
	}
}

// doJSON sends body as JSON to apiPath under /api/2.0/mlflow/ and decodes the
// response into out (may be nil). Non-2xx responses become *APIError.
func (c *Client) doJSON(ctx context.Context, method, apiPath string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("mlflow: marshal request: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.trackingURI+apiPrefix+apiPath, reader)
	if err != nil {
		return fmt.Errorf("mlflow: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("mlflow: decode response: %w", err)
		}
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	var body struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &body)
	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  body.ErrorCode,
		Message:    body.Message,
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./pkg/mlflow/ -run TestDoJSON -v`
Expected: PASS (both subtests).

- [ ] **Step 6: Lint**

Run: `golangci-lint run ./pkg/mlflow/`
Expected: no findings.

- [ ] **Step 7: Commit**

```bash
git add pkg/mlflow/client.go pkg/mlflow/errors.go pkg/mlflow/client_test.go
git commit -m "feat: add mlflow client core and typed API errors"
```

---

## Task 2: Types

**Files:**
- Create: `pkg/mlflow/types.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Experiment struct { ExperimentID string \`json:"experiment_id"\`; Name string \`json:"name"\` }`
  - `type RunInfo struct { RunID string \`json:"run_id"\`; ExperimentID string \`json:"experiment_id"\`; Status string \`json:"status"\`; ArtifactURI string \`json:"artifact_uri"\` }`
  - `type Run struct { Info RunInfo \`json:"info"\` }`
  - `type Param struct { Key string \`json:"key"\`; Value string \`json:"value"\` }`
  - `type Metric struct { Key string \`json:"key"\`; Value float64 \`json:"value"\`; Timestamp int64 \`json:"timestamp"\`; Step int64 \`json:"step"\` }`
  - `type RunTag struct { Key string \`json:"key"\`; Value string \`json:"value"\` }`
  - `type RunStatus string` with consts `RunStatusRunning = "RUNNING"`, `RunStatusFinished = "FINISHED"`, `RunStatusFailed = "FAILED"`.

- [ ] **Step 1: Write `types.go`**

Create `pkg/mlflow/types.go`:

```go
package mlflow

// Experiment is an MLflow experiment.
type Experiment struct {
	ExperimentID string `json:"experiment_id"`
	Name         string `json:"name"`
}

// RunInfo is the metadata half of a Run.
type RunInfo struct {
	RunID        string `json:"run_id"`
	ExperimentID string `json:"experiment_id"`
	Status       string `json:"status"`
	ArtifactURI  string `json:"artifact_uri"`
}

// Run is a single MLflow run.
type Run struct {
	Info RunInfo `json:"info"`
}

// Param is a run parameter (string-valued).
type Param struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Metric is a run metric (float-valued, optionally stepped).
type Metric struct {
	Key       string  `json:"key"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
	Step      int64   `json:"step"`
}

// RunTag is a key/value tag on a run.
type RunTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// RunStatus is a run lifecycle status accepted by MLflow.
type RunStatus string

// Run lifecycle statuses.
const (
	RunStatusRunning  RunStatus = "RUNNING"
	RunStatusFinished RunStatus = "FINISHED"
	RunStatusFailed   RunStatus = "FAILED"
)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./pkg/mlflow/`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add pkg/mlflow/types.go
git commit -m "feat: add mlflow REST resource types"
```

---

## Task 3: Experiments (get-by-name, create, get-or-create)

**Files:**
- Create: `pkg/mlflow/experiments.go`
- Create: `pkg/mlflow/experiments_test.go`

**Interfaces:**
- Consumes: `Client.doJSON`, `Experiment`, `APIError`.
- Produces:
  - `func (c *Client) GetExperimentByName(ctx context.Context, name string) (*Experiment, error)` — returns `*APIError` with `ErrorCode == "RESOURCE_DOES_NOT_EXIST"` when absent.
  - `func (c *Client) CreateExperiment(ctx context.Context, name string) (string, error)` — returns the new experiment ID.
  - `func (c *Client) GetOrCreateExperiment(ctx context.Context, name string) (*Experiment, error)` — get-by-name, create on `RESOURCE_DOES_NOT_EXIST`, then re-fetch.

- [ ] **Step 1: Write the failing test**

Create `pkg/mlflow/experiments_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mlflow/ -run TestGetOrCreateExperiment -v`
Expected: FAIL — `undefined: (*Client).GetOrCreateExperiment`.

- [ ] **Step 3: Write `experiments.go`**

Create `pkg/mlflow/experiments.go`:

```go
package mlflow

import (
	"context"
	"errors"
	"net/http"
)

// GetExperimentByName returns the experiment with the given name, or an
// *APIError with ErrorCode "RESOURCE_DOES_NOT_EXIST" if none exists.
func (c *Client) GetExperimentByName(ctx context.Context, name string) (*Experiment, error) {
	var out struct {
		Experiment Experiment `json:"experiment"`
	}
	// MLflow accepts experiment_name as a query param on this GET; sending it
	// in a JSON body also works and keeps the call site uniform.
	err := c.doJSON(ctx, http.MethodGet, "experiments/get-by-name?experiment_name="+urlQueryEscape(name), nil, &out)
	if err != nil {
		return nil, err
	}
	return &out.Experiment, nil
}

// CreateExperiment creates an experiment and returns its ID.
func (c *Client) CreateExperiment(ctx context.Context, name string) (string, error) {
	var out struct {
		ExperimentID string `json:"experiment_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "experiments/create", map[string]string{"name": name}, &out); err != nil {
		return "", err
	}
	return out.ExperimentID, nil
}

// GetOrCreateExperiment returns the named experiment, creating it if absent.
func (c *Client) GetOrCreateExperiment(ctx context.Context, name string) (*Experiment, error) {
	exp, err := c.GetExperimentByName(ctx, name)
	if err == nil {
		return exp, nil
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != "RESOURCE_DOES_NOT_EXIST" {
		return nil, err
	}
	if _, err := c.CreateExperiment(ctx, name); err != nil {
		return nil, err
	}
	return c.GetExperimentByName(ctx, name)
}
```

- [ ] **Step 4: Add the query-escape helper to `client.go`**

Add to `pkg/mlflow/client.go` imports: `"net/url"`. Add this function at the end of `client.go`:

```go
func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./pkg/mlflow/ -run TestGetOrCreateExperiment -v`
Expected: PASS (both subtests).

- [ ] **Step 6: Lint + full test**

Run: `golangci-lint run ./pkg/mlflow/ && go test -race ./pkg/mlflow/`
Expected: no findings; all pass.

- [ ] **Step 7: Commit**

```bash
git add pkg/mlflow/experiments.go pkg/mlflow/experiments_test.go pkg/mlflow/client.go
git commit -m "feat: add experiment get-by-name, create, get-or-create"
```

---

## Task 4: Runs (create, update, set-tag)

**Files:**
- Create: `pkg/mlflow/runs.go`
- Create: `pkg/mlflow/runs_test.go`

**Interfaces:**
- Consumes: `Client.doJSON`, `Run`, `RunTag`, `RunStatus`.
- Produces:
  - `func (c *Client) CreateRun(ctx context.Context, experimentID string, tags []RunTag) (*Run, error)` — sets `start_time` to now (ms).
  - `func (c *Client) UpdateRun(ctx context.Context, runID string, status RunStatus) error` — sets `end_time` to now (ms).
  - `func (c *Client) SetTag(ctx context.Context, runID, key, value string) error`.

- [ ] **Step 1: Write the failing test**

Create `pkg/mlflow/runs_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mlflow/ -run 'TestCreateRun|TestUpdateRun' -v`
Expected: FAIL — `undefined: (*Client).CreateRun`.

- [ ] **Step 3: Write `runs.go`**

Create `pkg/mlflow/runs.go`:

```go
package mlflow

import (
	"context"
	"net/http"
	"time"
)

func nowMillis() int64 { return time.Now().UnixMilli() }

// CreateRun starts a new run under experimentID with optional tags.
func (c *Client) CreateRun(ctx context.Context, experimentID string, tags []RunTag) (*Run, error) {
	body := map[string]any{
		"experiment_id": experimentID,
		"start_time":    nowMillis(),
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}
	var out struct {
		Run Run `json:"run"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "runs/create", body, &out); err != nil {
		return nil, err
	}
	return &out.Run, nil
}

// UpdateRun sets a run's terminal status and end time.
func (c *Client) UpdateRun(ctx context.Context, runID string, status RunStatus) error {
	body := map[string]any{
		"run_id":   runID,
		"status":   string(status),
		"end_time": nowMillis(),
	}
	return c.doJSON(ctx, http.MethodPost, "runs/update", body, nil)
}

// SetTag sets a single key/value tag on a run.
func (c *Client) SetTag(ctx context.Context, runID, key, value string) error {
	body := map[string]string{"run_id": runID, "key": key, "value": value}
	return c.doJSON(ctx, http.MethodPost, "runs/set-tag", body, nil)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mlflow/ -run 'TestCreateRun|TestUpdateRun' -v`
Expected: PASS.

- [ ] **Step 5: Lint + full test**

Run: `golangci-lint run ./pkg/mlflow/ && go test -race ./pkg/mlflow/`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add pkg/mlflow/runs.go pkg/mlflow/runs_test.go
git commit -m "feat: add run create, update, set-tag"
```

---

## Task 5: Params & metrics (log-parameter, log-metric, log-batch)

**Files:**
- Create: `pkg/mlflow/metrics.go`
- Create: `pkg/mlflow/metrics_test.go`

**Interfaces:**
- Consumes: `Client.doJSON`, `Param`, `Metric`, `RunTag`, `nowMillis`.
- Produces:
  - `func (c *Client) LogParam(ctx context.Context, runID, key, value string) error`.
  - `func (c *Client) LogMetric(ctx context.Context, runID, key string, value float64, step int64) error` — stamps `timestamp` = now (ms).
  - `func (c *Client) LogBatch(ctx context.Context, runID string, params []Param, metrics []Metric, tags []RunTag) error` — metrics missing a `Timestamp` get now (ms); batch capped at 1000 metrics / 100 params / 100 tags per MLflow limits (caller's responsibility to chunk; documented).

- [ ] **Step 1: Write the failing test**

Create `pkg/mlflow/metrics_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mlflow/ -run 'TestLogBatch|TestLogMetric' -v`
Expected: FAIL — `undefined: (*Client).LogBatch`.

- [ ] **Step 3: Write `metrics.go`**

Create `pkg/mlflow/metrics.go`:

```go
package mlflow

import (
	"context"
	"net/http"
)

// LogParam records a single string parameter on a run.
func (c *Client) LogParam(ctx context.Context, runID, key, value string) error {
	body := map[string]string{"run_id": runID, "key": key, "value": value}
	return c.doJSON(ctx, http.MethodPost, "runs/log-parameter", body, nil)
}

// LogMetric records a single metric value at the given step, stamped now.
func (c *Client) LogMetric(ctx context.Context, runID, key string, value float64, step int64) error {
	body := map[string]any{
		"run_id":    runID,
		"key":       key,
		"value":     value,
		"timestamp": nowMillis(),
		"step":      step,
	}
	return c.doJSON(ctx, http.MethodPost, "runs/log-metric", body, nil)
}

// LogBatch logs params, metrics, and tags in one call. Metrics with a zero
// Timestamp are stamped with the current time. MLflow caps a batch at 1000
// metrics, 100 params, and 100 tags — the caller must chunk larger sets.
func (c *Client) LogBatch(ctx context.Context, runID string, params []Param, metrics []Metric, tags []RunTag) error {
	stamped := make([]Metric, len(metrics))
	now := nowMillis()
	for i, m := range metrics {
		if m.Timestamp == 0 {
			m.Timestamp = now
		}
		stamped[i] = m
	}
	body := map[string]any{"run_id": runID}
	if len(params) > 0 {
		body["params"] = params
	}
	if len(stamped) > 0 {
		body["metrics"] = stamped
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}
	return c.doJSON(ctx, http.MethodPost, "runs/log-batch", body, nil)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mlflow/ -run 'TestLogBatch|TestLogMetric' -v`
Expected: PASS.

- [ ] **Step 5: Lint + full test**

Run: `golangci-lint run ./pkg/mlflow/ && go test -race ./pkg/mlflow/`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add pkg/mlflow/metrics.go pkg/mlflow/metrics_test.go
git commit -m "feat: add log-parameter, log-metric, log-batch"
```

---

## Task 6: Artifact upload (proxied)

**Files:**
- Create: `pkg/mlflow/artifacts.go`
- Create: `pkg/mlflow/artifacts_test.go`

**Interfaces:**
- Consumes: `Client` fields (`trackingURI`, `token`, `http`), `APIError`.
- Produces:
  - `func (c *Client) LogArtifact(ctx context.Context, runID, artifactPath string, content []byte) error` — uploads bytes to the tracking server's artifact proxy at `PUT /api/2.0/mlflow-artifacts/artifacts/{run-relative-path}?run_id={runID}`. Returns `*APIError` on non-2xx (e.g. when the server isn't run with `--serve-artifacts`).

> **Note on the proxy path:** MLflow's artifact proxy is served at
> `/api/2.0/mlflow-artifacts/artifacts/<path>`. The `<path>` is resolved
> against the run's artifact root; passing `?run_id=` lets the server place it
> under that run. Confirm the exact query/path contract against the running
> 3.14 server via the `example/` binary (Task 8) — if it differs, this is the
> one method to adjust, and the risk is called out in the spec.

- [ ] **Step 1: Write the failing test**

Create `pkg/mlflow/artifacts_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mlflow/ -run TestLogArtifact -v`
Expected: FAIL — `undefined: (*Client).LogArtifact`.

- [ ] **Step 3: Write `artifacts.go`**

Create `pkg/mlflow/artifacts.go`:

```go
package mlflow

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

const artifactProxyPrefix = "/api/2.0/mlflow-artifacts/artifacts/"

// LogArtifact uploads content as a run artifact at artifactPath (run-relative),
// via the tracking server's artifact proxy. The server must run with
// --serve-artifacts; otherwise this returns an *APIError.
func (c *Client) LogArtifact(ctx context.Context, runID, artifactPath string, content []byte) error {
	u := c.trackingURI + artifactProxyPrefix + artifactPath + "?run_id=" + urlQueryEscape(runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("mlflow: new artifact request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow: upload artifact: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mlflow/ -run TestLogArtifact -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Lint + full test**

Run: `golangci-lint run ./pkg/mlflow/ && go test -race ./pkg/mlflow/`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add pkg/mlflow/artifacts.go pkg/mlflow/artifacts_test.go
git commit -m "feat: add proxied artifact upload"
```

---

## Task 7: `example/` live smoke binary

**Files:**
- Create: `example/main.go`

**Interfaces:**
- Consumes: the whole `pkg/mlflow` public API.
- Produces: a runnable program; exits 0 after logging one param, one metric, one tag, and one artifact to a "(smoke test)" experiment.

- [ ] **Step 1: Write `example/main.go`**

Create `example/main.go`:

```go
// Command example is a live smoke test for the mlflow-go-sdk against a running
// MLflow server. Set MLFLOW_TRACKING_URI (and optionally MLFLOW_TRACKING_TOKEN).
//
//	MLFLOW_TRACKING_URI=http://localhost:5000 go run ./example
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk/pkg/mlflow"
)

func main() {
	uri := os.Getenv("MLFLOW_TRACKING_URI")
	if uri == "" {
		log.Fatal("set MLFLOW_TRACKING_URI (e.g. http://localhost:5000)")
	}
	ctx := context.Background()
	c := mlflow.New(mlflow.Options{TrackingURI: uri, Token: os.Getenv("MLFLOW_TRACKING_TOKEN")})

	exp, err := c.GetOrCreateExperiment(ctx, "mlflow-go-sdk (smoke test)")
	if err != nil {
		log.Fatalf("experiment: %v", err)
	}
	fmt.Printf("experiment %s (%s)\n", exp.Name, exp.ExperimentID)

	run, err := c.CreateRun(ctx, exp.ExperimentID, []mlflow.RunTag{{Key: "smoke", Value: "true"}})
	if err != nil {
		log.Fatalf("create run: %v", err)
	}
	fmt.Printf("run %s\n", run.Info.RunID)

	if err := c.LogBatch(ctx, run.Info.RunID,
		[]mlflow.Param{{Key: "model", Value: "smoke"}},
		[]mlflow.Metric{{Key: "value", Value: 1}},
		[]mlflow.RunTag{{Key: "phase", Value: "smoke"}},
	); err != nil {
		log.Fatalf("log batch: %v", err)
	}

	if err := c.LogArtifact(ctx, run.Info.RunID, "smoke.txt", []byte("ok\n")); err != nil {
		log.Fatalf("log artifact: %v (is the server run with --serve-artifacts?)", err)
	}

	if err := c.UpdateRun(ctx, run.Info.RunID, mlflow.RunStatusFinished); err != nil {
		log.Fatalf("update run: %v", err)
	}
	fmt.Println("smoke OK — metric + artifact logged")
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./example/`
Expected: no output, exit 0.

- [ ] **Step 3: (Optional, needs a live server) Run against local MLflow**

If a local MLflow 3.14 is available:

```bash
mlflow server --backend-store-uri sqlite:///mlflow.db --serve-artifacts --host 127.0.0.1 --port 5000 &
MLFLOW_TRACKING_URI=http://127.0.0.1:5000 go run ./example
```
Expected: prints `smoke OK — metric + artifact logged`, exit 0. If the artifact
step fails, confirm `--serve-artifacts` and adjust `artifacts.go` per its Note.

- [ ] **Step 4: Commit**

```bash
git add example/main.go
git commit -m "feat: add live smoke example binary"
```

---

## Task 8: Tag v0.1.0

**Files:** none (git tag only).

**Interfaces:**
- Consumes: a green `make test` + `make lint` across the module.
- Produces: pushed tag `v0.1.0` so consumers can pin it and drop replace directives.

- [ ] **Step 1: Full verify**

Run: `go mod tidy && make test && make lint && go build ./...`
Expected: `go.mod`/`go.sum` clean (still no third-party deps), all tests pass, no lint findings.

- [ ] **Step 2: Tag and push**

```bash
git push origin main
git tag v0.1.0
git push origin v0.1.0
```
Expected: tag visible at `github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk`.

---

## Self-Review

**Spec coverage (spec §8):**
- Repo cleanup → Task 0. ✅
- `client.go` / `experiments.go` / `runs.go` / `metrics.go` / `artifacts.go` / `types.go` / `errors.go` → Tasks 1–6. ✅
- `example/` smoke → Task 7. ✅
- `httptest` unit tests → in each of Tasks 1,3,4,5,6. ✅
- Local-vs-remote via one option (`Options.TrackingURI`/`Token`) → Task 1. ✅
- Proxied artifact upload + graceful non-2xx error → Task 6. ✅
- Tag v0.1.0 + drop replace (consumer side) → Task 8 (tag); replace-drop is in the eval plan. ✅

**Placeholder scan:** No TBD/TODO; every code step shows complete code; every command has expected output.

**Type consistency:** `Client.doJSON`, `nowMillis`, `urlQueryEscape`, `decodeAPIError`, `APIError`, `Run`/`RunInfo`/`Param`/`Metric`/`RunTag`/`RunStatus` used consistently across tasks. `GetOrCreateExperiment` matches the eval adapter's expectation in the sibling plan.

**Deferred/confirm-live items:** exact artifact-proxy path/query (Task 6 Note + Task 7 Step 3) — the one contract to verify against the running 3.14 server; isolated to `artifacts.go`.
