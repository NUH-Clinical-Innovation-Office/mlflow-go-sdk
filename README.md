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

See `example/` for a full runnable smoke test.
