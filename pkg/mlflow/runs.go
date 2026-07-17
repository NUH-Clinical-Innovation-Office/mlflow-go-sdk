package mlflow

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

func nowMillis() int64 { return time.Now().UnixMilli() }

// JSON field names shared across run request bodies (extracted to satisfy goconst).
const (
	fieldExperimentID = "experiment_id"
	fieldRunID        = "run_id"
	fieldKey          = "key"
	fieldValue        = "value"
)

// CreateRun starts a new run under experimentID with optional tags.
func (c *Client) CreateRun(ctx context.Context, experimentID string, tags []RunTag) (*Run, error) {
	body := map[string]any{
		fieldExperimentID: experimentID,
		"start_time":      nowMillis(),
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

// GetRun fetches a run by ID, including its resolved artifact_uri.
func (c *Client) GetRun(ctx context.Context, runID string) (*Run, error) {
	var out struct {
		Run Run `json:"run"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "runs/get?"+fieldRunID+"="+url.QueryEscape(runID), nil, &out); err != nil {
		return nil, err
	}
	return &out.Run, nil
}

// UpdateRun sets a run's terminal status and end time.
func (c *Client) UpdateRun(ctx context.Context, runID string, status RunStatus) error {
	body := map[string]any{
		fieldRunID: runID,
		"status":   string(status),
		"end_time": nowMillis(),
	}
	return c.doJSON(ctx, http.MethodPost, "runs/update", body, nil)
}

// SetTag sets a single key/value tag on a run.
func (c *Client) SetTag(ctx context.Context, runID, key, value string) error {
	body := map[string]string{fieldRunID: runID, fieldKey: key, fieldValue: value}
	return c.doJSON(ctx, http.MethodPost, "runs/set-tag", body, nil)
}
