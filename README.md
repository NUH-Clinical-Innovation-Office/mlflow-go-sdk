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
