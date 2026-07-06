// Command example is a live smoke test for the mlflow-go-sdk against a running
// MLflow server. Set MLFLOW_TRACKING_URI (and optionally MLFLOW_TRACKING_TOKEN).
//
//	MLFLOW_TRACKING_URI=http://localhost:5000 go run ./example
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/NUH-Clinical-Innovation-Office/mlflow-go-sdk/pkg/mlflow"
)

func main() {
	uri := os.Getenv("MLFLOW_TRACKING_URI")
	if uri == "" {
		log.Fatal("set MLFLOW_TRACKING_URI (e.g. http://localhost:5000)")
	}
	ctx := context.Background()
	c := mlflow.New(mlflow.Options{TrackingURI: uri, Token: os.Getenv("MLFLOW_TRACKING_TOKEN")})

	exp, err := c.GetOrCreateExperiment(ctx, "mlflow-go-sdk (smoke test)")
	if err != nil {
		log.Fatalf("experiment: %v", err)
	}
	fmt.Printf("experiment %s (%s)\n", exp.Name, exp.ExperimentID)

	run, err := c.CreateRun(ctx, exp.ExperimentID, []mlflow.RunTag{{Key: "smoke", Value: "true"}})
	if err != nil {
		log.Fatalf("create run: %v", err)
	}
	fmt.Printf("run %s\n", run.Info.RunID)

	if err := c.LogBatch(ctx, run.Info.RunID,
		[]mlflow.Param{{Key: "model", Value: "smoke"}},
		[]mlflow.Metric{{Key: "value", Value: 1}},
		[]mlflow.RunTag{{Key: "phase", Value: "smoke"}},
	); err != nil {
		log.Fatalf("log batch: %v", err)
	}

	if err := c.LogArtifact(ctx, run.Info.RunID, "smoke.txt", []byte("ok\n")); err != nil {
		log.Fatalf("log artifact: %v (is the server run with --serve-artifacts?)", err)
	}

	if err := c.UpdateRun(ctx, run.Info.RunID, mlflow.RunStatusFinished); err != nil {
		log.Fatalf("update run: %v", err)
	}
	fmt.Println("smoke OK — metric + artifact logged")
}
