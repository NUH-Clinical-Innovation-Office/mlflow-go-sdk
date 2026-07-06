package mlflow

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

const artifactProxyPrefix = "/api/2.0/mlflow-artifacts/artifacts/"

// LogArtifact uploads content as a run artifact at artifactPath (run-relative),
// via the tracking server's artifact proxy. The server must run with
// --serve-artifacts; otherwise this returns an *APIError.
func (c *Client) LogArtifact(ctx context.Context, runID, artifactPath string, content []byte) error {
	u := c.trackingURI + artifactProxyPrefix + artifactPath + "?run_id=" + urlQueryEscape(runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("mlflow: new artifact request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow: upload artifact: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	return nil
}
