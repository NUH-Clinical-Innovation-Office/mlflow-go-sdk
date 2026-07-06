//go:build integration

// Package integration provides integration tests for the API.
package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/your-org/go-backend-template/internal/auth"
	"github.com/your-org/go-backend-template/internal/config"
	"github.com/your-org/go-backend-template/internal/db"
	dbSQLC "github.com/your-org/go-backend-template/internal/db/sqlc"
	"github.com/your-org/go-backend-template/internal/logging"
	"github.com/your-org/go-backend-template/internal/todo"
)

var (
	testContainer *postgres.PostgresContainer
	testConfig    *config.Config
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	if err := setupTestContainer(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup test container: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Cleanup
	if err := testContainer.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to terminate container: %v\n", err)
	}

	os.Exit(code)
}

func setupTestContainer(ctx context.Context) error {
	var err error
	testContainer, err = postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Get connection string
	connStr, err := testContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("failed to get connection string: %w", err)
	}

	// Setup test config
	testConfig = &config.Config{
		Database: config.DatabaseConfig{
			URL:             connStr,
			MaxConns:        10,
			MinConns:        2,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Auth: config.AuthConfig{
			JWTSecretKey:     "test-secret-key-for-integration-tests-32+bytes!",
			JWTExpireMinutes: 60,
			BcryptCost:       4, // Fast for tests
		},
	}

	// Run migrations
	if err := runMigrations(connStr); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func runMigrations(connStr string) error {
	return runMigrationsWithConnStr(connStr)
}

// runMigrationsWithConnStr runs the project migrations using the same
// golang-migrate library as cmd/migrate, so the test schema can never
// drift from production.
func runMigrationsWithConnStr(connStr string) error {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	migrationsURL := "file://" + filepath.Join(repoRoot, "migrations")

	m, err := migrate.New(migrationsURL, connStr)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		if sErr, dErr := m.Close(); sErr != nil || dErr != nil {
			fmt.Fprintf(os.Stderr, "migrator close: src=%v db=%v\n", sErr, dErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func setupTestDeps(t *testing.T) (*db.Pool, *dbSQLC.Queries, *auth.Service, *auth.Repository, *todo.Service, *todo.Repository, *auth.Handler, *todo.Handler) {
	t.Helper()

	ctx := context.Background()
	logger, _ := logging.New("debug", "console")

	pool, err := db.New(ctx, testConfig.Database)
	if err != nil {
		t.Fatalf("Failed to create db pool: %v", err)
	}

	queries := dbSQLC.New(pool.Pool)

	authRepo := auth.NewRepository(queries)
	authService := auth.NewService(authRepo, testConfig.Auth.JWTSecretKey, time.Duration(testConfig.Auth.JWTExpireMinutes)*time.Minute, 4)
	authHandler := auth.NewHandler(authService, authService, logger)

	todoRepo := todo.NewRepository(queries)
	todoService := todo.NewService(todoRepo)
	todoHandler := todo.NewHandler(todoService)

	return pool, queries, authService, authRepo, todoService, todoRepo, authHandler, todoHandler
}
