package mlflow

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"
)

const (
	// v3TracePath is the StartTraceV3 endpoint (creates TraceInfo). The MLflow
	// server exposes traces under /api/3.0 even though other APIs are 2.0.
	v3TracePath = "/api/3.0/mlflow/traces"
	// traceDataFileName is MLflow's fixed artifact name for a trace's span data.
	traceDataFileName = "traces.json"
	// previewMaxChars caps the traceInputs/traceOutputs metadata preview. Trace
	// metadata values are stored in a varchar(8000) column, so full I/O cannot
	// live there; MLflow's own client truncates the preview (~1000 chars) and
	// keeps the full payload in the traces.json artifact instead.
	previewMaxChars = 1000
	defaultSpanType = "LLM"
	statusCodeOK    = "STATUS_CODE_OK"
	messageField    = "message"
)

// LogTraceParams describes one trace with a single root span. Inputs and Outputs
// are marshaled to JSON: the full value goes into the span's mlflow.spanInputs/
// spanOutputs attributes (untruncated, in the traces.json artifact) and a
// truncated copy into the trace metadata preview.
type LogTraceParams struct {
	ExperimentID string            // required
	TraceID      string            // optional; generated ("tr-"+32 hex) if empty
	Name         string            // trace/span name shown in the UI
	SpanType     string            // defaults to "LLM"
	Inputs       any               // full LLM input
	Outputs      any               // full LLM output
	StartTime    time.Time         // defaults to now
	EndTime      time.Time         // defaults to now
	Metadata     map[string]string // extra trace_metadata (e.g. mlflow.sourceRun)
	Tags         map[string]string // extra tags; mlflow.traceName is set from Name
}

// LogTrace creates a trace via StartTraceV3, then uploads its span data as the
// traces.json artifact so the full untruncated Inputs/Outputs are visible in the
// MLflow trace UI. Trace-metadata previews are truncated to stay under the
// server's 8000-char metadata limit. Returns the trace id (server-echoed when
// available, else the client-supplied/generated one). On a StartTraceV3 failure
// no artifact is uploaded; an artifact failure is returned with the step named.
//
//nolint:gocritic // by-value params struct is the ergonomic public API; the heavy-struct copy is negligible next to two network round-trips.
func (c *Client) LogTrace(ctx context.Context, p LogTraceParams) (string, error) {
	return c.logTrace(ctx, &p)
}

func (c *Client) logTrace(ctx context.Context, p *LogTraceParams) (string, error) {
	traceID := p.TraceID
	if traceID == "" {
		traceID = "tr-" + randomHex(16)
	}
	spanType := p.SpanType
	if spanType == "" {
		spanType = defaultSpanType
	}
	start := p.StartTime
	if start.IsZero() {
		start = time.Now()
	}
	end := p.EndTime
	if end.IsZero() {
		end = time.Now()
	}

	inputsJSON, err := json.Marshal(p.Inputs)
	if err != nil {
		return "", fmt.Errorf("mlflow: marshal trace inputs: %w", err)
	}
	outputsJSON, err := json.Marshal(p.Outputs)
	if err != nil {
		return "", fmt.Errorf("mlflow: marshal trace outputs: %w", err)
	}

	echoedID, err := c.startTraceV3(ctx, p, traceID, start, end, inputsJSON, outputsJSON)
	if err != nil {
		return "", err
	}
	if echoedID != "" {
		traceID = echoedID
	}

	artifact, err := buildTracesJSON(traceID, p.Name, spanType, start, end, inputsJSON, outputsJSON)
	if err != nil {
		return "", err
	}
	// Deterministic per-trace artifact path the MLflow UI reads span data from.
	relPath := escapeArtifactPath(fmt.Sprintf("%s/traces/%s/artifacts/%s", p.ExperimentID, traceID, traceDataFileName))
	if err := c.putArtifact(ctx, relPath, "", artifact); err != nil {
		return "", fmt.Errorf("mlflow: upload trace artifact: %w", err)
	}
	return traceID, nil
}

// startTraceV3 posts the TraceInfo (metadata + truncated previews) and returns
// the server-echoed trace id when present.
func (c *Client) startTraceV3(ctx context.Context, p *LogTraceParams, traceID string, start, end time.Time, inputsJSON, outputsJSON []byte) (string, error) {
	metadata := map[string]string{
		"mlflow.traceInputs":  truncateRunes(string(inputsJSON), previewMaxChars),
		"mlflow.traceOutputs": truncateRunes(string(outputsJSON), previewMaxChars),
	}
	maps.Copy(metadata, p.Metadata)
	tags := map[string]string{"mlflow.traceName": p.Name}
	maps.Copy(tags, p.Tags)

	traceInfo := map[string]any{
		"trace_id": traceID,
		"trace_location": map[string]any{
			"type":              "MLFLOW_EXPERIMENT",
			"mlflow_experiment": map[string]string{fieldExperimentID: p.ExperimentID},
		},
		"request_time":       start.UTC().Format(time.RFC3339Nano),
		"execution_duration": fmt.Sprintf("%.3fs", end.Sub(start).Seconds()),
		"state":              "OK",
		"trace_metadata":     metadata,
		"tags":               tags,
	}
	body := map[string]any{"trace": map[string]any{"trace_info": traceInfo}}

	var parsed struct {
		Trace struct {
			TraceInfo struct {
				TraceID string `json:"trace_id"`
			} `json:"trace_info"`
		} `json:"trace"`
	}
	if err := c.doTraceJSON(ctx, http.MethodPost, v3TracePath, body, &parsed); err != nil {
		return "", err
	}
	return parsed.Trace.TraceInfo.TraceID, nil
}

// buildTracesJSON serializes the single root span into the traces.json artifact
// body ({"spans":[...]}). Attribute values are JSON strings (double-encoded), and
// span/trace ids are base64 of raw OTEL bytes, matching MLflow's on-disk format.
func buildTracesJSON(traceID, name, spanType string, start, end time.Time, inputsJSON, outputsJSON []byte) ([]byte, error) {
	quotedType, err := json.Marshal(spanType)
	if err != nil {
		return nil, fmt.Errorf("mlflow: marshal span type: %w", err)
	}
	quotedReqID, err := json.Marshal(traceID)
	if err != nil {
		return nil, fmt.Errorf("mlflow: marshal request id: %w", err)
	}
	span := map[string]any{
		"trace_id":             base64.StdEncoding.EncodeToString(randomBytes(16)),
		"span_id":              base64.StdEncoding.EncodeToString(randomBytes(8)),
		"parent_span_id":       nil,
		nameField:              name,
		"start_time_unix_nano": start.UnixNano(),
		"end_time_unix_nano":   end.UnixNano(),
		"events":               []any{},
		"status":               map[string]string{"code": statusCodeOK, messageField: ""},
		"attributes": map[string]string{
			"mlflow.traceRequestId": string(quotedReqID),
			"mlflow.spanType":       string(quotedType),
			"mlflow.spanInputs":     string(inputsJSON),
			"mlflow.spanOutputs":    string(outputsJSON),
		},
		"links": []any{},
	}
	out, err := json.Marshal(map[string]any{"spans": []any{span}})
	if err != nil {
		return nil, fmt.Errorf("mlflow: marshal traces.json: %w", err)
	}
	return out, nil
}

// doTraceJSON is like doJSON but targets an absolute API path (the trace API
// lives under /api/3.0, not the client's /api/2.0/mlflow/ prefix).
func (c *Client) doTraceJSON(ctx context.Context, method, apiPath string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("mlflow: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.trackingURI+apiPath, strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("mlflow: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setRequestHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow: do request: %w", err)
	}
	defer func() {
		// Drain any unread body before closing so the connection returns to the
		// keep-alive pool cleanly. Without this, a partially-read response (e.g.
		// json.Decoder stops at the first value) leaves the pooled connection in
		// a bad state and the next request on it can be rejected mid-stream.
		//nolint:errcheck // best-effort drain; a copy error just prevents reuse.
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

// truncateRunes returns s limited to limit runes (never splitting a rune).
func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error on supported platforms
	return b
}

func randomHex(n int) string {
	return hex.EncodeToString(randomBytes(n))
}
