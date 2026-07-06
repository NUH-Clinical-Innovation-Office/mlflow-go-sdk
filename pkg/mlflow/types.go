package mlflow

// Experiment is an MLflow experiment.
type Experiment struct {
	ExperimentID string `json:"experiment_id"`
	Name         string `json:"name"`
}

// RunInfo is the metadata half of a Run.
type RunInfo struct {
	RunID        string `json:"run_id"`
	ExperimentID string `json:"experiment_id"`
	Status       string `json:"status"`
	ArtifactURI  string `json:"artifact_uri"`
}

// Run is a single MLflow run.
type Run struct {
	Info RunInfo `json:"info"`
}

// Param is a run parameter (string-valued).
type Param struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Metric is a run metric (float-valued, optionally stepped).
type Metric struct {
	Key       string  `json:"key"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
	Step      int64   `json:"step"`
}

// RunTag is a key/value tag on a run.
type RunTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// RunStatus is a run lifecycle status accepted by MLflow.
type RunStatus string

// Run lifecycle statuses.
const (
	RunStatusRunning  RunStatus = "RUNNING"
	RunStatusFinished RunStatus = "FINISHED"
	RunStatusFailed   RunStatus = "FAILED"
)
