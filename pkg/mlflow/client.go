// Package mlflow is a minimal client for the MLflow 3.x tracking REST API.
package mlflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiPrefix = "/api/2.0/mlflow/"

// Options configures a Client.
type Options struct {
	// TrackingURI is the MLflow server base URL, e.g. http://localhost:5000.
	TrackingURI string
	// Token, when non-empty, is sent as an Authorization: Bearer header.
	Token string
	// HTTPClient overrides the default client (30s timeout) when non-nil.
	HTTPClient *http.Client
}

// Client talks to an MLflow tracking server.
type Client struct {
	trackingURI string
	token       string
	http        *http.Client
}

// New returns a Client for the given options.
func New(opts Options) *Client {
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		trackingURI: strings.TrimRight(opts.TrackingURI, "/"),
		token:       opts.Token,
		http:        hc,
	}
}

// doJSON sends body as JSON to apiPath under /api/2.0/mlflow/ and decodes the
// response into out (may be nil). Non-2xx responses become *APIError.
func (c *Client) doJSON(ctx context.Context, method, apiPath string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("mlflow: marshal request: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.trackingURI+apiPrefix+apiPath, reader)
	if err != nil {
		return fmt.Errorf("mlflow: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow: do request: %w", err)
	}
	defer func() {
		// Drain any unread body before closing so the underlying TCP
		// connection returns to the keep-alive pool. Go only reuses a
		// connection whose body was fully read; without this, an SDK issuing
		// many small calls (e.g. LogMetric per step) leaks connections.
		//nolint:errcheck // best-effort drain; a copy error just means the
		// connection won't be reused, which is not worth surfacing.
		io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("mlflow: decode response: %w", err)
		}
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	var body struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	// Best-effort: if the error body can't be read or parsed, we still
	// return an APIError with the status code and empty fields.
	if raw, readErr := io.ReadAll(resp.Body); readErr == nil {
		if unmarshalErr := json.Unmarshal(raw, &body); unmarshalErr != nil {
			body.Message = string(raw)
		} else if body.ErrorCode == "" && body.Message == "" {
			// Valid JSON, but not in the {error_code, message} shape MLflow
			// normally uses. Fall back to the raw text so it isn't dropped.
			body.Message = string(raw)
		}
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  body.ErrorCode,
		Message:    body.Message,
	}
}
