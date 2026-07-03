package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/abevz/af-coordinator/internal/api"
	"github.com/abevz/af-coordinator/internal/config"
)

func main() {
	cfg := config.Default()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.SlogLevel(),
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := api.RunDaemon(ctx, logger, cfg); err != nil {
		logger.Error("daemon failed", "error", err)
		os.Exit(1)
	}
}
