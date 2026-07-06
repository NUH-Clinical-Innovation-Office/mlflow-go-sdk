package mlflow

import (
	"context"
	"net/http"
)

// LogParam records a single string parameter on a run.
func (c *Client) LogParam(ctx context.Context, runID, key, value string) error {
	body := map[string]string{fieldRunID: runID, fieldKey: key, fieldValue: value}
	return c.doJSON(ctx, http.MethodPost, "runs/log-parameter", body, nil)
}

// LogMetric records a single metric value at the given step, stamped now.
func (c *Client) LogMetric(ctx context.Context, runID, key string, value float64, step int64) error {
	body := map[string]any{
		fieldRunID:  runID,
		fieldKey:    key,
		fieldValue:  value,
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
	body := map[string]any{fieldRunID: runID}
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
