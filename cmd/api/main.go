// Command api is the main entry point for the API server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/go-backend-template/internal/auth"
	"github.com/your-org/go-backend-template/internal/config"
	"github.com/your-org/go-backend-template/internal/db"
	dbSQLC "github.com/your-org/go-backend-template/internal/db/sqlc"
	"github.com/your-org/go-backend-template/internal/logging"
	"github.com/your-org/go-backend-template/internal/observability"
	"github.com/your-org/go-backend-template/internal/router"
	"github.com/your-org/go-backend-template/internal/todo"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize logger
	logger, err := logging.New(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			fmt.Fprintf(os.Stderr, "failed to sync logger: %v\n", syncErr)
		}
	}()

	logger.Info("starting go-backend-template API")

	// Initialize OpenTelemetry tracing. Export timeout = shutdown
	// timeout minus 1s so the batch export can complete inside Shutdown.
	tracerProvider, err := observability.Setup(
		context.Background(),
		cfg.Observability.ServiceName,
		cfg.Observability.OTLPInsecure,
		cfg.Server.ShutdownTimeout-time.Second,
		logger,
	)
	if err != nil {
		logger.Warn("failed to initialize tracing", zap.Error(err))
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			shutdownErr := tracerProvider.Shutdown(ctx)
			if shutdownErr != nil {
				logger.Warn("tracer provider shutdown failed", zap.Error(shutdownErr))
			}
		}()
		logger.Info("OpenTelemetry tracing initialized")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := db.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	logger.Info("database connection established")

	// Initialize SQLC queries
	queries := dbSQLC.New(pool.Pool)

	// Initialize auth repository and service
	authRepo := auth.NewRepository(queries)
	if err := auth.InitDefaultRoles(ctx, authRepo); err != nil {
		logger.Warn("default role cache not populated (migrations may be missing); Register will lazy-load on first call", zap.Error(err))
	}
	authService := auth.NewService(authRepo, cfg.Auth.JWTSecretKey, time.Duration(cfg.Auth.JWTExpireMinutes)*time.Minute, cfg.Auth.BcryptCost)
	authHandler := auth.NewHandler(authService, authService, logger)

	// Initialize todo repository and service
	todoRepo := todo.NewRepository(queries)
	todoService := todo.NewService(todoRepo)
	todoHandler := todo.NewHandler(todoService)

	// Get tracer
	tracer := noop.NewTracerProvider().Tracer(cfg.Observability.ServiceName)
	if tracerProvider != nil {
		tracer = tracerProvider.Tracer(cfg.Observability.ServiceName)
	}

	// Build router with all dependencies
	mux := router.New(
		logger,
		tracer,
		authService,
		authHandler,
		todoHandler,
		&cfg.CORS,
		cfg.RateLimit,
		func() error { return pool.Ping(context.Background()) },
		cfg.Swagger.Enabled,
		cfg.Server.TrustedProxies,
	)

	// Start HTTP server
	return startHTTPServer(cfg, mux, logger)
}

func startHTTPServer(cfg *config.Config, handler http.Handler, logger *zap.Logger) error {
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("HTTP server starting",
			zap.String("addr", server.Addr),
			zap.Int("port", cfg.Server.Port),
		)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
			return fmt.Errorf("graceful shutdown: %w", err)
		}

		logger.Info("HTTP server stopped")
		return nil
	}
}
