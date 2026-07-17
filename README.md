# mlflow-go-sdk

A minimal Go client for the [MLflow](https://mlflow.org) 3.x tracking REST API.

Covers experiments, runs, params/metrics/tags, and proxied artifact upload —
the subset needed by NUH evaluation pipelines. Not a comprehensive MLflow SDK.

## Install

```bash
go get github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk/pkg/mlflow
```

## Quick start

```go
package main

import (
    "context"
    "log"

    "github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk/pkg/mlflow"
)

func main() {
    ctx := context.Background()
    c := mlflow.New(mlflow.Options{TrackingURI: "http://localhost:5000"})

    exp, err := c.GetOrCreateExperiment(ctx, "My Experiment")
    if err != nil {
        log.Fatal(err)
    }

    run, err := c.CreateRun(ctx, exp.ExperimentID, nil)
    if err != nil {
        log.Fatal(err)
    }

    _ = c.LogMetric(ctx, run.Info.RunID, "accuracy", 0.91, 0)
    _ = c.UpdateRun(ctx, run.Info.RunID, mlflow.RunStatusFinished)
}
```

## How it fits together

The client is a thin wrapper over the MLflow tracking server. Every method maps
to one REST call; artifacts go through the server's artifact proxy rather than
directly to blob storage.

```mermaid
graph LR
    App["Your Go code"] -->|mlflow.Client| SDK["mlflow-go-sdk"]
    SDK -->|"/api/2.0/mlflow/*"| Tracking["MLflow Tracking Server"]
    SDK -->|"/api/2.0/mlflow-artifacts/*"| Proxy["Artifact Proxy"]
    Tracking --> Store[("Backend Store")]
    Proxy --> Blob[("Artifact Store")]
```

## Run lifecycle

A typical evaluation run resolves an experiment, opens a run, logs data, then
closes the run with a terminal status.

```mermaid
sequenceDiagram
    participant C as Your code
    participant M as mlflow.Client
    participant S as MLflow Server

    C->>M: GetOrCreateExperiment(name)
    M->>S: experiments/get-by-name
    alt not found
        M->>S: experiments/create
    end
    M-->>C: Experiment

    C->>M: CreateRun(expID, tags)
    M->>S: runs/create (status RUNNING)
    M-->>C: Run

    C->>M: LogBatch / LogMetric / LogParam / SetTag
    M->>S: runs/log-batch, runs/log-metric, ...

    C->>M: LogArtifact(runID, path, bytes)
    M->>S: runs/get (resolve artifact_uri)
    M->>S: mlflow-artifacts/artifacts/<run-root>/... (PUT)

    C->>M: UpdateRun(runID, FINISHED)
    M->>S: runs/update (end_time set)
```

Run status transitions the SDK supports:

```mermaid
stateDiagram-v2
    [*] --> RUNNING: CreateRun
    RUNNING --> FINISHED: UpdateRun(RunStatusFinished)
    RUNNING --> FAILED: UpdateRun(RunStatusFailed)
    FINISHED --> [*]
    FAILED --> [*]
```

## Examples

### Log a batch of params, metrics, and tags

`LogBatch` sends everything in one request. Metrics with a zero `Timestamp` are
stamped with the current time. MLflow caps a batch at 1000 metrics, 100 params,
and 100 tags — chunk larger sets yourself.

```go
err := c.LogBatch(ctx, run.Info.RunID,
    []mlflow.Param{{Key: "model", Value: "opus-4-8"}},
    []mlflow.Metric{{Key: "accuracy", Value: 0.93}},
    []mlflow.RunTag{{Key: "phase", Value: "eval"}},
)
```

### Upload an artifact

Requires the tracking server to run with `--serve-artifacts`. The path is
run-relative; nested paths are created as needed.

```go
err := c.LogArtifact(ctx, run.Info.RunID, "reports/summary.json", []byte(`{"ok":true}`))
```

### Log a native trace with full span I/O

`LogTrace` records one trace with a single root span so a call's full input and
output are visible in the MLflow trace UI. It creates the trace (StartTraceV3),
then uploads the span data as the `traces.json` artifact — trace metadata values
are capped at 8000 chars server-side, so the full untruncated I/O lives in the
span artifact while only a short preview goes in metadata.

```go
id, err := c.LogTrace(ctx, mlflow.LogTraceParams{
    ExperimentID: exp.ExperimentID,
    Name:         "Patient 1",
    SpanType:     "LLM",
    Inputs:       map[string]any{"messages": messages}, // full input
    Outputs:      map[string]any{"messages": reply},     // full output
    Metadata:     map[string]string{"mlflow.sourceRun": run.Info.RunID},
})
```

### Per-call tracing (flag-gated)

Go has no decorators; `Traced` is the idiomatic equivalent — wrap a call and
toggle logging with a flag (config default or per-call override), so you trace
some steps and skip others within the same run. When enabled it logs
`<name>.duration_ms` and a `trace.<name>` tag of `ok`/`error`; `fn`'s error is
always authoritative — a logging failure never hides it.

```go
cfg := struct{ Trace bool }{Trace: true}

_ = c.Traced(ctx, run.Info.RunID, "extraction", cfg.Trace, func(ctx context.Context) error {
    return doExtraction(ctx) // logs extraction.duration_ms + trace.extraction=ok/error
})

_ = c.Traced(ctx, run.Info.RunID, "judge", false, func(ctx context.Context) error {
    return doJudge(ctx) // enabled=false → passthrough, nothing logged
})
```

### Authentication

Authentication is optional in the SDK. Plain local MLflow servers usually do
not require a token, so leave `Token` empty. Protected deployments, such as
Panacea's `/mlflow` proxy, require a bearer token so Panacea can authenticate
the caller before forwarding traces, metrics, params, runs, and artifacts to
MLflow.

Set `Token` to send an `Authorization: Bearer` header on every call:

```go
c := mlflow.New(mlflow.Options{
    TrackingURI: os.Getenv("MLFLOW_TRACKING_URI"),
    Token:       os.Getenv("MLFLOW_TRACKING_TOKEN"),
})
```

For Panacea, mint the MLflow token from the auth service
(`POST /api/v1/tokens/mlflow`) and pass the returned token as
`MLFLOW_TRACKING_TOKEN`.

### Custom HTTP client

By default the SDK uses an `http.Client` with a 30s timeout. Set
`Options.HTTPClient` to supply your own — e.g. a longer timeout for large
artifacts, a custom transport, or a proxy:

```go
c := mlflow.New(mlflow.Options{
    TrackingURI: os.Getenv("MLFLOW_TRACKING_URI"),
    HTTPClient:  &http.Client{Timeout: 2 * time.Minute},
})
```

The SDK identifies every tracking and artifact request as
`mlflow-go-client/<version>`. This avoids protected reverse proxies treating
Go's generic `Go-http-client/2.0` default as an unsupported automated client
and returning an HTML `403 Forbidden` before MLflow can validate the bearer
token.

## API surface

| Method | MLflow endpoint |
|--------|-----------------|
| `GetExperimentByName` | `experiments/get-by-name` |
| `CreateExperiment` | `experiments/create` |
| `GetOrCreateExperiment` | get-by-name, then create if absent |
| `CreateRun` | `runs/create` |
| `GetRun` | `runs/get` |
| `UpdateRun` | `runs/update` |
| `SetTag` | `runs/set-tag` |
| `LogParam` | `runs/log-parameter` |
| `LogMetric` | `runs/log-metric` |
| `LogBatch` | `runs/log-batch` |
| `LogArtifact` | `runs/get` then `mlflow-artifacts/artifacts/<run-root>/...` (proxy) |
| `LogTrace` | `3.0/mlflow/traces` (StartTraceV3) then `mlflow-artifacts/artifacts/<exp>/traces/<id>/artifacts/traces.json` |
| `Traced` | wraps `LogMetric` + `SetTag` |

Non-2xx responses are returned as `*mlflow.APIError` (carrying `StatusCode`,
`ErrorCode`, and `Message`); use `errors.As` to inspect them.

## Error handling

```go
exp, err := c.GetExperimentByName(ctx, "nope")
var apiErr *mlflow.APIError
if errors.As(err, &apiErr) && apiErr.ErrorCode == "RESOURCE_DOES_NOT_EXIST" {
    // handle missing experiment
}
```

## Development

```bash
make test     # go test -race ./...
make lint     # golangci-lint
make example  # live smoke test (needs MLFLOW_TRACKING_URI)
```

See `example/` for a full runnable smoke test.

## Releases

Releases are cut manually. Pick the next version from the
[Conventional Commit](https://www.conventionalcommits.org/) types since the last
tag:

| Commit type | Bump |
|-------------|------|
| `fix:` | patch (`x.y.Z`) |
| `feat:` | minor (`x.Y.0`) |
| `feat!:` / `BREAKING CHANGE:` | major (`X.0.0`) |

Then, from a clean `main`:

```bash
# 1. sync the version constant so the User-Agent matches the tag
#    edit clientVersion in pkg/mlflow/client.go to the new version
git commit -am "chore(release): v0.4.0"
git push

# 2. tag and publish (uses your account, not the CI token)
git tag v0.4.0
git push origin v0.4.0
gh release create v0.4.0 --generate-notes
```

`--generate-notes` builds the changelog from the conventional commits since the
last tag. Keep `clientVersion` in `pkg/mlflow/client.go` in step with the tag —
it is sent as the `User-Agent`.
