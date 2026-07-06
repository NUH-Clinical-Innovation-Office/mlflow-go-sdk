package mlflow

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const artifactProxyPrefix = "/api/2.0/mlflow-artifacts/artifacts/"

// escapeArtifactPath percent-encodes each non-empty segment of a run-relative
// artifact path, preserving "/" separators.
func escapeArtifactPath(artifactPath string) string {
	segments := strings.Split(artifactPath, "/")
	for i, seg := range segments {
		if seg != "" {
			segments[i] = url.PathEscape(seg)
		}
	}
	return strings.Join(segments, "/")
}

// LogArtifact uploads content as a run artifact at artifactPath (run-relative),
// via the tracking server's artifact proxy. The server must run with
// --serve-artifacts; otherwise this returns an *APIError.
func (c *Client) LogArtifact(ctx context.Context, runID, artifactPath string, content []byte) error {
	u := c.trackingURI + artifactProxyPrefix + escapeArtifactPath(artifactPath) + "?run_id=" + urlQueryEscape(runID)
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
