package mlflow

import (
	"context"
	"errors"
	"net/http"
)

// errCodeResourceNotExist is the MLflow API error_code returned when an
// experiment lookup by name finds nothing.
const errCodeResourceNotExist = "RESOURCE_DOES_NOT_EXIST"

// nameField is the JSON field name for an experiment's display name.
const nameField = "name"

// GetExperimentByName returns the experiment with the given name, or an
// *APIError with ErrorCode "RESOURCE_DOES_NOT_EXIST" if none exists.
func (c *Client) GetExperimentByName(ctx context.Context, name string) (*Experiment, error) {
	var out struct {
		Experiment Experiment `json:"experiment"`
	}
	// MLflow accepts experiment_name as a query param on this GET; sending it
	// in a JSON body also works and keeps the call site uniform.
	err := c.doJSON(ctx, http.MethodGet, "experiments/get-by-name?experiment_name="+urlQueryEscape(name), nil, &out)
	if err != nil {
		return nil, err
	}
	return &out.Experiment, nil
}

// CreateExperiment creates an experiment and returns its ID.
func (c *Client) CreateExperiment(ctx context.Context, name string) (string, error) {
	var out struct {
		ExperimentID string `json:"experiment_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "experiments/create", map[string]string{nameField: name}, &out); err != nil {
		return "", err
	}
	return out.ExperimentID, nil
}

// GetOrCreateExperiment returns the named experiment, creating it if absent.
func (c *Client) GetOrCreateExperiment(ctx context.Context, name string) (*Experiment, error) {
	exp, err := c.GetExperimentByName(ctx, name)
	if err == nil {
		return exp, nil
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != errCodeResourceNotExist {
		return nil, err
	}
	if _, err := c.CreateExperiment(ctx, name); err != nil {
		return nil, err
	}
	return c.GetExperimentByName(ctx, name)
}
