package mlflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const artifactProxyPrefix = "/api/2.0/mlflow-artifacts/artifacts/"

// escapeArtifactPath percent-encodes each non-empty segment of a run-relative
// artifact path, preserving "/" separators. Empty segments (from leading,
// trailing, or doubled slashes) are dropped so the proxy URL never contains a
// "//" that the server would reject or misroute.
func escapeArtifactPath(artifactPath string) string {
	segments := strings.Split(artifactPath, "/")
	escaped := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(seg))
	}
	return strings.Join(escaped, "/")
}

// proxyRootFromArtifactURI extracts the artifact-proxy path from a run's
// artifact_uri. MLflow returns proxy roots like
// "mlflow-artifacts:/<exp>/<run>/artifacts" or, with an explicit authority,
// "mlflow-artifacts://<host>/<exp>/<run>/artifacts". The proxy PUT endpoint is
// keyed by this path relative to the server's artifact root, so we return just
// the path portion (e.g. "<exp>/<run>/artifacts"). Any non-proxy scheme (s3://,
// file://, ...) means the server does not proxy this run's artifacts, so the
// caller falls back to the bare path.
func proxyRootFromArtifactURI(artifactURI string) (string, bool) {
	u, err := url.Parse(artifactURI)
	if err != nil || u.Scheme != "mlflow-artifacts" {
		return "", false
	}
	// u.Host is the optional authority; it names the same tracking server we
	// already target, so only the path matters for the proxy request.
	return strings.Trim(u.Path, "/"), true
}

// LogArtifact uploads content as a run artifact at artifactPath (run-relative),
// via the tracking server's artifact proxy. It first resolves the run's
// artifact_uri so the upload lands under the run's own artifact subtree (what
// the MLflow UI lists); without this the proxy would write to the server
// artifact root and the run's Artifacts tab would appear empty. The server must
// run with --serve-artifacts; otherwise this returns an *APIError.
func (c *Client) LogArtifact(ctx context.Context, runID, artifactPath string, content []byte) error {
	run, err := c.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("mlflow: resolve run artifact root: %w", err)
	}
	fullPath := escapeArtifactPath(artifactPath)
	if root, ok := proxyRootFromArtifactURI(run.Info.ArtifactURI); ok && root != "" {
		fullPath = escapeArtifactPath(root) + "/" + fullPath
	}
	u := c.trackingURI + artifactProxyPrefix + fullPath + "?run_id=" + url.QueryEscape(runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("mlflow: new artifact request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	c.setRequestHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow: upload artifact: %w", err)
	}
	defer func() {
		//nolint:errcheck // best-effort drain to allow keep-alive reuse.
		io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	return nil
}
