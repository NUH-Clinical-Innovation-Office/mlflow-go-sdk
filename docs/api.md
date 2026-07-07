# API Reference

All methods hang off `*mlflow.Client` (from `mlflow.New`), take a
`context.Context` first, and return `*mlflow.APIError` on any non-2xx response.

## Construction

### `New`

```go
func New(opts Options) *Client
```

`Options` fields:

| Field | Description |
|-------|-------------|
| `TrackingURI` | MLflow server base URL, e.g. `http://localhost:5000`. Trailing slash is trimmed. |
| `Token` | Optional. When non-empty, sent as `Authorization: Bearer <token>` on every request. Leave empty for unauthenticated MLflow servers. Use Panacea's minted MLflow token for protected `/mlflow` proxy deployments. |
| `HTTPClient` | Overrides the default client (30s timeout) when non-nil. |

## Experiments

### `GetExperimentByName`

```go
func (c *Client) GetExperimentByName(ctx context.Context, name string) (*Experiment, error)
```

Returns the named experiment, or an `*APIError` with
`ErrorCode == "RESOURCE_DOES_NOT_EXIST"` if none exists.

### `CreateExperiment`

```go
func (c *Client) CreateExperiment(ctx context.Context, name string) (string, error)
```

Creates an experiment and returns its ID.

### `GetOrCreateExperiment`

```go
func (c *Client) GetOrCreateExperiment(ctx context.Context, name string) (*Experiment, error)
```

Returns the named experiment, creating it if absent. Any error other than
`RESOURCE_DOES_NOT_EXIST` is returned as-is.

## Runs

### `CreateRun`

```go
func (c *Client) CreateRun(ctx context.Context, experimentID string, tags []RunTag) (*Run, error)
```

Starts a run under `experimentID` (status `RUNNING`, `start_time` set to now)
with optional tags.

### `UpdateRun`

```go
func (c *Client) UpdateRun(ctx context.Context, runID string, status RunStatus) error
```

Sets a run's terminal status and `end_time`. Valid statuses:
`RunStatusRunning`, `RunStatusFinished`, `RunStatusFailed`.

### `SetTag`

```go
func (c *Client) SetTag(ctx context.Context, runID, key, value string) error
```

Sets a single key/value tag on a run.

## Params, metrics, batches

### `LogParam`

```go
func (c *Client) LogParam(ctx context.Context, runID, key, value string) error
```

Records a single string parameter.

### `LogMetric`

```go
func (c *Client) LogMetric(ctx context.Context, runID, key string, value float64, step int64) error
```

Records a single metric value at `step`, stamped with the current time.

### `LogBatch`

```go
func (c *Client) LogBatch(ctx context.Context, runID string, params []Param, metrics []Metric, tags []RunTag) error
```

Logs params, metrics, and tags in one call. Metrics with a zero `Timestamp` are
stamped now. MLflow caps a batch at 1000 metrics, 100 params, and 100 tags — the
caller must chunk larger sets.

## Artifacts

### `LogArtifact`

```go
func (c *Client) LogArtifact(ctx context.Context, runID, artifactPath string, content []byte) error
```

Uploads `content` as a run artifact at `artifactPath` (run-relative) via the
tracking server's artifact proxy. The server must run with `--serve-artifacts`;
otherwise this returns an `*APIError`.

## Tracing

### `Traced`

```go
func (c *Client) Traced(ctx context.Context, runID, name string, enabled bool, fn func(ctx context.Context) error) error
```

Runs `fn` and, when `enabled`, records its timing (`<name>.duration_ms` metric)
and outcome (`trace.<name>` tag of `ok`/`error`). When `enabled` is false, calls
`fn` directly and logs nothing. `fn`'s error is always authoritative: a logging
failure never hides `fn`'s outcome.

## Errors

Non-2xx responses are returned as `*APIError`:

```go
type APIError struct {
    StatusCode int
    ErrorCode  string
    Message    string
}
```

Inspect with `errors.As`:

```go
var apiErr *mlflow.APIError
if errors.As(err, &apiErr) && apiErr.ErrorCode == "RESOURCE_DOES_NOT_EXIST" {
    // ...
}
```
