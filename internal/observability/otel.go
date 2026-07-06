// Package observability provides OpenTelemetry integration.
package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
)

// Setup initializes OpenTelemetry for distributed tracing.
// insecure controls whether the OTLP exporter uses plaintext (http) or TLS (https).
// exportTimeout caps the per-export call. The caller passes the server
// shutdown timeout minus a small headroom so the export always finishes
// inside the shutdown context.
func Setup(ctx context.Context, serviceName string, insecure bool, exportTimeout time.Duration, logger *zap.Logger) (*sdktrace.TracerProvider, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithContainer(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(getEnv("SERVICE_VERSION", "1.0.0")),
			semconv.DeploymentEnvironment(getEnv("ENVIRONMENT", "development")),
		),
	)
	if err != nil {
		logger.Warn("failed to create resource with auto-detection, using basic resource", zap.Error(err))
		res, err = resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceName(serviceName),
				semconv.ServiceVersion(getEnv("SERVICE_VERSION", "1.0.0")),
				semconv.DeploymentEnvironment(getEnv("ENVIRONMENT", "development")),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create basic resource: %w", err)
		}
	}

	// exportTimeout comes from the caller (typically
	// cfg.Server.ShutdownTimeout - 1s). The min keeps a sane lower bound
	// even if the operator misconfigures it.
	if exportTimeout < 2*time.Second {
		exportTimeout = 2 * time.Second
	}
	exporterOptions := []otlptracehttp.Option{
		otlptracehttp.WithTimeout(exportTimeout),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  5 * time.Minute,
		}),
	}
	if insecure {
		exporterOptions = append(exporterOptions, otlptracehttp.WithInsecure())
	}

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		exporterOptions = append(exporterOptions, otlptracehttp.WithEndpoint(endpoint))
		logger.Info("using custom OTLP endpoint", zap.String("endpoint", endpoint))
	}

	exporter, err := otlptracehttp.New(ctx, exporterOptions...)
	if err != nil {
		logger.Warn("failed to create OTLP trace exporter, tracing will be disabled", zap.Error(err))
		return sdktrace.NewTracerProvider(), nil
	}

	samplingRatio := getSamplingRatio()
	logger.Info("trace sampling configured", zap.Float64("ratio", samplingRatio))

	batchProcessor := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(2048),
		sdktrace.WithBatchTimeout(5*time.Second),
		sdktrace.WithMaxExportBatchSize(512),
		sdktrace.WithExportTimeout(30*time.Second),
	)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(batchProcessor),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(samplingRatio),
		)),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry tracing initialized successfully",
		zap.String("service_name", serviceName),
		zap.Float64("sampling_ratio", samplingRatio),
	)

	return tracerProvider, nil
}

func getSamplingRatio() float64 {
	defaultRatio := 0.1
	if env := os.Getenv("ENVIRONMENT"); env == "development" || env == "dev" || env == "" {
		defaultRatio = 1.0
	}

	ratio := float64(defaultRatio)
	if ratioStr := os.Getenv("OTEL_TRACE_SAMPLING_RATIO"); ratioStr != "" {
		if _, err := fmt.Sscanf(ratioStr, "%f", &ratio); err != nil {
			return float64(defaultRatio)
		}
		if ratio < 0.0 {
			ratio = 0.0
		}
		if ratio > 1.0 {
			ratio = 1.0
		}
	}

	return ratio
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
