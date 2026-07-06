package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	command := flag.String("command", "up", "Migration command (up, down, reset, force, version)")
	version := flag.Int("version", 0, "Target version for force command")
	migrationsPath := flag.String("path", "file://migrations", "Path to migrations directory")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	m, err := migrate.New(*migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			fmt.Fprintf(os.Stderr, "warning: error closing migrate: src=%v, db=%v\n", srcErr, dbErr)
		}
	}()

	switch *command {
	case "up":
		return runUp(m)
	case "down":
		return runDown(m)
	case "reset":
		return runReset(m)
	case "force":
		return runForce(m, *version)
	case "version":
		return runVersion(m)
	default:
		return fmt.Errorf("unknown command: %s", *command)
	}
}

func runUp(m *migrate.Migrate) error {
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	fmt.Println("migrations applied successfully")
	return nil
}

func runDown(m *migrate.Migrate) error {
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration down failed: %w", err)
	}
	fmt.Println("migration down completed")
	return nil
}

func runReset(m *migrate.Migrate) error {
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration down failed: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	fmt.Println("migration reset completed")
	return nil
}

func runForce(m *migrate.Migrate, version int) error {
	if version == 0 {
		return fmt.Errorf("-version flag is required for force command")
	}
	if err := m.Force(version); err != nil {
		return fmt.Errorf("migration force failed: %w", err)
	}
	fmt.Printf("migration forced to version %d\n", version)
	return nil
}

func runVersion(m *migrate.Migrate) error {
	v, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}
	if dirty {
		fmt.Printf("version: %d (dirty)\n", v)
	} else {
		fmt.Printf("version: %d\n", v)
	}
	return nil
}
