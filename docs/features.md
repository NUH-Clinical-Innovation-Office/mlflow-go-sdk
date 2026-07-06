# Features

Scope: the subset of the MLflow 3.x tracking REST API needed by NUH evaluation
pipelines. Not a comprehensive MLflow SDK.

## Supported

| Feature | Status | Description |
|---------|--------|-------------|
| Experiments | stable | Get by name, create, get-or-create |
| Runs | stable | Create (RUNNING), update to terminal status (FINISHED/FAILED) |
| Params | stable | `LogParam` single string parameter |
| Metrics | stable | `LogMetric` single stepped/stamped value |
| Tags | stable | `SetTag` single key/value on a run |
| Batch logging | stable | `LogBatch` params + metrics + tags in one call |
| Artifacts | stable | `LogArtifact` via the server's artifact proxy (`--serve-artifacts`) |
| Per-call tracing | stable | Flag-gated `Traced` wrapper: duration metric + ok/error tag |
| Bearer auth | stable | Optional `Token` sent as `Authorization: Bearer` |
| Typed errors | stable | Non-2xx responses returned as `*APIError` |
| Custom HTTP client | stable | `Options.HTTPClient` override (default 30s timeout) |
| Keep-alive reuse | stable | Response bodies drained before close for connection reuse |

## Not implemented

Out of scope for the pipelines this SDK serves. File an issue if needed.

| Feature | Notes |
|---------|-------|
| Search / list runs | No `runs/search` |
| Delete / restore runs or experiments | Lifecycle limited to create + terminal update |
| Metric history retrieval | Write-only; no `metrics/get-history` |
| Model registry | No registered models / versions |
| Direct artifact store access | Only the tracking-server artifact proxy is used |
| Automatic batch chunking | Caller must chunk beyond MLflow's per-batch caps |
