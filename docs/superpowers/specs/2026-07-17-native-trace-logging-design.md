# mlflow-go-sdk — Native trace logging with full span I/O

**Status:** Design approved (pending spec review)
**Date:** 2026-07-17
**Target release:** v0.3.0 (additive, non-breaking)

## Problem

The consuming eval pipeline (dashboards-backend) needs each patient's full LLM
input (14 KB system prompt + report text) and output visible in the MLflow trace
UI. The current hand-rolled code in `internal/eval/mlflow.go` puts the full I/O
into `trace_metadata` (`mlflow.traceInputs` / `mlflow.traceOutputs`) on the
`StartTraceV3` call. That fails against the panacea server:

```
BAD_REQUEST: value too long for type character varying(8000)
INSERT INTO trace_request_metadata (key, value, request_id) ...
```

`trace_metadata` values are capped at 8000 chars (Postgres `varchar(8000)`), so
full I/O cannot live there.

## How MLflow itself does it (verified against panacea, MLflow 3.14.0)

Reverse-engineered by tracing the Python SDK's HTTP calls to the same server:

1. `POST /api/3.0/mlflow/traces` — creates **TraceInfo** only. Metadata
   `traceInputs`/`traceOutputs` hold a **truncated preview** (Python caps at
   ~1000 chars), so the 8000-char limit is never hit. Response echoes the
   trace's `artifact_location` tag,
   `mlflow-artifacts:/{exp}/traces/{traceID}/artifacts`.
2. `PUT /api/2.0/mlflow-artifacts/artifacts/{exp}/traces/{traceID}/artifacts/traces.json`
   — uploads **TraceData** (the spans) as an artifact. Full untruncated I/O
   lives here. The trace UI reads Inputs/Outputs from this artifact.

The server exposes no OTLP receiver (`POST /v1/traces` → 404); artifact upload
is the working path.

### `traces.json` body (verified shape)

```json
{"spans": [ {
  "trace_id": "<base64 of 16-byte id>",
  "span_id": "<base64 of 8-byte id>",
  "parent_span_id": null,
  "name": "Patient 1",
  "start_time_unix_nano": 1784284960264750000,
  "end_time_unix_nano": 1784284960304989000,
  "events": [],
  "status": {"code": "STATUS_CODE_OK", "message": ""},
  "attributes": {
    "mlflow.traceRequestId": "\"tr-...\"",
    "mlflow.spanType": "\"LLM\"",
    "mlflow.spanInputs": "{\"messages\":[...]}",
    "mlflow.spanOutputs": "{\"messages\":[...]}"
  },
  "links": []
} ] }
```

Note: every `attributes` **value is a JSON string** — i.e. the inner objects are
JSON-encoded again (double-encoded). `trace_id`/`span_id` are base64 of the raw
OTEL binary ids.

## Scope

**In:** a reusable `LogTrace` method on `*Client` in mlflow-go-sdk that performs
both steps and returns the trace id. Span types. Preview truncation.

**Out (this change):** assessments (stay hand-rolled in dashboards-backend);
multi-span traces (single root span is enough for the eval); OTLP.

## API

New file `pkg/mlflow/traces.go` (the existing `trace.go` `Traced` timing wrapper
is unrelated and stays).

```go
// LogTraceParams describes one trace with a single root span.
type LogTraceParams struct {
    ExperimentID string            // required
    TraceID      string            // optional; generated if empty ("tr-" + 32 hex)
    Name         string            // trace/span name shown in the UI
    SpanType     string            // defaults to "LLM"
    Inputs       any               // marshaled to spanInputs (full) + preview
    Outputs      any               // marshaled to spanOutputs (full) + preview
    StartTime    time.Time         // defaults to now
    EndTime      time.Time         // defaults to now
    Metadata     map[string]string // extra trace_metadata (e.g. mlflow.sourceRun, version)
    Tags         map[string]string // extra tags (mlflow.traceName is set from Name)
}

// LogTrace creates a trace (StartTraceV3) then uploads its span data as the
// traces.json artifact, so the full untruncated Inputs/Outputs are visible in
// the MLflow trace UI. trace_metadata previews are truncated to stay under the
// server's 8000-char metadata limit. Returns the server-assigned trace id.
func (c *Client) LogTrace(ctx context.Context, p LogTraceParams) (string, error)
```

### Behaviour

- **Preview truncation:** `previewMaxChars = 1000`. `mlflow.traceInputs`/
  `traceOutputs` metadata = first 1000 chars of the marshaled JSON (rune-safe,
  no mid-rune cut).
- **IDs:** `TraceID` default = `"tr-" + 32 lowercase hex`. The base64
  `trace_id`/`span_id` in the span come from 16/8 random bytes; they need not
  match the `tr-...` id (the artifact is keyed by path, and MLflow's own
  export uses independent OTEL ids). `mlflow.traceRequestId` attr = the `tr-...`
  id (JSON-quoted).
- **Attributes:** values double-encoded (`spanInputs` = `string(json.Marshal(inputs))`;
  the whole attributes map is then marshaled as part of the span). `spanType`
  and `traceRequestId` are JSON-quoted strings.
- **Two requests:** StartTraceV3 (reuse `doJSON`-style POST with
  `setRequestHeaders`), then artifact PUT (reuse `LogArtifact`'s proxy-path
  logic, generalized for a trace path rather than a run path).
- **Errors:** StartTraceV3 failure returns immediately (no artifact attempt).
  Artifact failure returns an error naming the step; the trace still exists but
  without full I/O — caller decides.

### Artifact path helper

`LogArtifact` today resolves a **run's** artifact_uri via `GetRun`. Traces use a
different, deterministic path: `{exp}/traces/{traceID}/artifacts/traces.json`.
Add an internal `putArtifact(ctx, proxyRelPath, content, query)` that both
`LogArtifact` and `LogTrace` call, so the raw PUT + proxy prefix + escaping live
in one place. `LogArtifact` keeps its run-resolution then delegates; `LogTrace`
builds the trace path directly and delegates.

## Consumer changes (dashboards-backend)

`internal/eval/mlflow.go`:
- `createTrace` — replaced by a call to `client.LogTrace(...)` passing the chat
  inputs/outputs, `Metadata` (`mlflow.sourceRun`, `mlflow.llm.model`, `version`,
  `test_id`, `mlflow.chat.tokenUsage`), and `Tags` (`mlflow.traceName`).
- The existing full chat-message inputs/outputs (already built in `createTrace`)
  are passed straight through — no truncation in the consumer.
- `logPatientAssessments` / `createAssessment` — unchanged.

## Testing (SDK, TDD, httptest)

- `LogTrace` posts StartTraceV3 then PUTs traces.json; assert both endpoints hit,
  correct methods, and the artifact body parses to `{"spans":[...]}` with
  `mlflow.spanInputs` carrying the full (untruncated) inputs.
- Metadata preview is truncated to ≤1000 chars for a >1000-char input.
- Full inputs in the span are NOT truncated (>1000-char input round-trips whole
  in `mlflow.spanInputs`).
- StartTraceV3 non-2xx → error, no artifact PUT attempted.
- Artifact PUT non-2xx → error naming the artifact step.
- Default TraceID / SpanType / times applied when zero.
- Existing `LogArtifact` tests still pass after the `putArtifact` refactor.

## Non-goals / constraints

- stdlib-only (`pkg/mlflow`); no otel deps — base64 ids via `encoding/base64`,
  hex via `encoding/hex`, random via `crypto/rand`.
- Additive: no existing exported signature changes. Ships as `v0.3.0`.
- Strict lint (errcheck check-blank, bodyclose, goconst, gocyclo, gocritic).
