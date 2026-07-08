package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/abevz/af-coordinator/internal/build"
	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/mcp"
)

func main() {
	cfg := config.Default()
	actor := os.Getenv("AF_COORDINATOR_ACTOR")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := mcp.NewServer(client.New(cfg.SocketPath), actor, build.Version)
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "afc-mcp: %v\n", err)
		os.Exit(1)
	}
}
