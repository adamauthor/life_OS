package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

const defaultMigrationsSource = "file://migrations"

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: go run ./cmd/migrate <up|down|steps|force|version> [arg]")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	sourceURL := os.Getenv("MIGRATIONS_SOURCE")
	if sourceURL == "" {
		sourceURL = defaultMigrationsSource
	}

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil {
			logger.Warn("migration source close failed", "error", sourceErr)
		}
		if databaseErr != nil {
			logger.Warn("migration database close failed", "error", databaseErr)
		}
	}()

	switch args[0] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		logger.Info("migrations are up to date")
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		logger.Info("migrations rolled back")
	case "steps":
		if len(args) < 2 {
			return fmt.Errorf("steps requires signed integer argument")
		}
		steps, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("parse steps: %w", err)
		}
		if err := m.Steps(steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		logger.Info("migration steps applied", "steps", steps)
	case "force":
		if len(args) < 2 {
			return fmt.Errorf("force requires version argument")
		}
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("parse version: %w", err)
		}
		if err := m.Force(version); err != nil {
			return err
		}
		logger.Info("migration version forced", "version", version)
	case "version":
		version, dirty, err := m.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			return err
		}
		logger.Info("migration version", "version", version, "dirty", dirty)
	default:
		return fmt.Errorf("unknown migration command %q", args[0])
	}

	return nil
}
