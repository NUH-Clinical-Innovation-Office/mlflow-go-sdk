# mlflow-go-sdk v0.2 — Flag-gated Traced wrapper

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Give the SDK the ergonomics of Python's `@mlflow.trace` decorator in idiomatic Go: a flag-gated higher-order wrapper that, when enabled, auto-logs a wrapped call's timing and success/error to the current run, and when disabled is a zero-overhead passthrough.

**Architecture:** Go has no decorators/monkeypatching, so this is a higher-order function `Traced` on `*Client`. It wraps `fn func(context.Context) error`; when `enabled` it times the call and logs `<name>.duration_ms` as a stepped metric plus a `trace.<name>` status tag (`ok`/`error`) on the run; when `!enabled` it calls `fn` directly and logs nothing. This mirrors russell-gpt's explicit per-call trace flag (which deliberately avoids MLflow autolog because a global patch can't tell inference calls from judge calls). The enable/disable knob is a plain `bool` argument, so callers get both a config-default (pass a config field) and per-call override (pass a literal) with no hidden global state.

**Tech Stack:** Go 1.26, stdlib only. Uses the existing v0.1 `LogMetric`/`SetTag` methods — no new REST endpoints. Tested against `httptest.Server`.

## Global Constraints

- Module `github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk`; `pkg/mlflow` stdlib-only.
- Every exported symbol has a godoc comment.
- Lint strict (`.golangci.yml`: errcheck check-blank, bodyclose, goconst, gocyclo, gocritic).
- Additive and non-breaking — no existing exported signature changes. Ships as `v0.2.0`.
- Conventional Commits (`feat:`/`test:` …).

---

## Task 1: `Traced` wrapper

**Files:**
- Create: `pkg/mlflow/trace.go`
- Create: `pkg/mlflow/trace_test.go`

**Interfaces:**
- Consumes: `Client.LogMetric`, `Client.SetTag`, `nowMillis` (from runs.go).
- Produces:
  - `func (c *Client) Traced(ctx context.Context, runID, name string, enabled bool, fn func(ctx context.Context) error) error`
    - `enabled == false`: returns `fn(ctx)` directly — no MLflow calls, nothing logged.
    - `enabled == true`: records `time.Since(start)` in ms, calls `LogMetric(ctx, runID, name+".duration_ms", ms, 0)`, then `SetTag(ctx, runID, "trace."+name, status)` where status is `"ok"` when `fn` returned nil else `"error"`. **Always returns `fn`'s error**; a logging failure must not mask the wrapped call's outcome — on a logging error when `fn` succeeded, return the logging error; when `fn` failed, return `fn`'s error (logging error is secondary, dropped). The wrapped call's result is authoritative.

- [ ] **Step 1: Write the failing test**

Create `pkg/mlflow/trace_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mlflow/ -run TestTraced -v`
Expected: FAIL — `undefined: (*Client).Traced`.

- [ ] **Step 3: Write `trace.go`**

Create `pkg/mlflow/trace.go`:

```go
package mlflow

import (
	"context"
	"time"
)

// Traced runs fn and, when enabled, records its timing and success/error to the
// run — the Go analog of Python's mlflow trace decorator, gated by an explicit
// flag so callers trace some calls (e.g. inference) and skip others (e.g. a
// judge pass) within the same run.
//
// When enabled is false, Traced calls fn directly and logs nothing (zero
// overhead). When enabled is true, it logs "<name>.duration_ms" as a metric and
// a "trace.<name>" tag of "ok" or "error" on runID. fn's error is always
// authoritative: a logging failure never hides fn's outcome — if fn failed, its
// error is returned even when logging also failed.
func (c *Client) Traced(ctx context.Context, runID, name string, enabled bool, fn func(ctx context.Context) error) error {
	if !enabled {
		return fn(ctx)
	}
	start := time.Now()
	fnErr := fn(ctx)
	ms := float64(time.Since(start).Milliseconds())

	status := "ok"
	if fnErr != nil {
		status = "error"
	}

	logErr := c.LogMetric(ctx, runID, name+".duration_ms", ms, 0)
	if tagErr := c.SetTag(ctx, runID, "trace."+name, status); tagErr != nil && logErr == nil {
		logErr = tagErr
	}

	if fnErr != nil {
		return fnErr
	}
	return logErr
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mlflow/ -run TestTraced -v`
Expected: PASS (all three).

- [ ] **Step 5: Lint + full test**

Run: `golangci-lint run ./pkg/mlflow/ && go test -race ./pkg/mlflow/`
(If golangci-lint emits spurious "directory not found" noise, run `rtk proxy golangci-lint run ./pkg/mlflow/`.)
Expected: no findings; all pass. If `goconst` flags a repeated literal (e.g. `"ok"`/`"error"`), extract a named const behavior-preservingly (the codebase's established pattern) and note it.

- [ ] **Step 6: Commit**

```bash
git add pkg/mlflow/trace.go pkg/mlflow/trace_test.go
git commit -m "feat: add flag-gated Traced wrapper for per-call timing/status logging"
```

---

## Task 2: Document in README + tag v0.2.0

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add a Traced usage snippet to README.md**

Add under the existing usage example:

```markdown
### Per-call tracing (flag-gated)

Go has no decorators; `Traced` is the idiomatic equivalent — wrap a call and
toggle logging with a flag (config default or per-call override), so you trace
some queries and skip others in the same run:

```go
cfg := struct{ Trace bool }{Trace: true}
_ = c.Traced(ctx, run.Info.RunID, "extraction", cfg.Trace, func(ctx context.Context) error {
    return doExtraction(ctx) // logs extraction.duration_ms + trace.extraction=ok/error
})
_ = c.Traced(ctx, run.Info.RunID, "judge", false, func(ctx context.Context) error {
    return doJudge(ctx) // enabled=false → passthrough, nothing logged
})
```
```

- [ ] **Step 2: Verify build + full suite**

Run: `go build ./... && go test -race ./... && golangci-lint run ./...`
Expected: clean.

- [ ] **Step 3: Commit, merge, tag**

```bash
git add README.md
git commit -m "docs: document Traced wrapper"
# controller/human step: merge to main, then:
git tag -a v0.2.0 -m "v0.2.0: flag-gated Traced wrapper"
git push origin main && git push origin v0.2.0
```

---

## Self-Review

**Spec coverage:** flag-gated wrapper (enabled→log timing+status, disabled→passthrough) → Task 1; config-default + per-call override achieved via the plain `bool` arg (caller passes a config field or a literal) → Task 1 interface + README example. v0.1-endpoints-only (LogMetric/SetTag) → Task 1. Additive/non-breaking → new file, no signature changes.

**Placeholder scan:** none — full test + impl code present.

**Type consistency:** `Traced` uses existing `LogMetric(ctx, runID, key, value float64, step int64)` and `SetTag(ctx, runID, key, value string)` signatures from v0.1 exactly.

**Design note:** error semantics are explicit and tested — `fn`'s error is authoritative; logging errors never mask it. The one judgment call (return logging error when `fn` succeeded but logging failed) is intentional: a silent logging failure would make "traced" runs unreliable, so it surfaces.
