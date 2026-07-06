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
