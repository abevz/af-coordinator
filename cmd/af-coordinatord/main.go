package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"path/filepath"

	"github.com/abevz/af-coordinator/internal/api"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/store/sqlite"
	"github.com/abevz/af-coordinator/migrations"
)

func main() {
	cfg := config.Default()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.SlogLevel(),
	}))

	// Ensure database directory exists before opening.
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		logger.Error("failed to create database directory", "error", err)
		os.Exit(1)
	}

	// Open database and run migrations.
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.Migrate(db, migrations.FS); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := api.RunDaemon(ctx, logger, cfg, db); err != nil {
		logger.Error("daemon failed", "error", err)
		os.Exit(1)
	}
}
