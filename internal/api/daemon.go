package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/abevz/af-coordinator/internal/build"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/core"
)

func RunDaemon(ctx context.Context, logger *slog.Logger, cfg config.Config, db *sql.DB) error {
	if err := os.MkdirAll(filepath.Dir(cfg.SocketPath), 0o755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	if err := removeStaleSocket(cfg.SocketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket: %w", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(cfg.SocketPath)
	}()

	if err := os.Chmod(cfg.SocketPath, 0o660); err != nil {
		logger.Warn("failed to chmod socket", "path", cfg.SocketPath, "error", err)
	}

	mux := http.NewServeMux()

	// Health endpoints.
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		h := core.Health{
			Name:       "af-coordinator",
			Status:     "ok",
			DBPath:     cfg.DBPath,
			SocketPath: cfg.SocketPath,
			Time:       time.Now().UTC(),
			Version:    build.Version,
		}

		if err := db.PingContext(r.Context()); err != nil {
			h.Status = "degraded"
			logger.Warn("health check db ping failed", "error", err)
		}

		writeJSON(w, http.StatusOK, h)
	}

	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/v1/health", healthHandler)

	// Project registration.
	mux.HandleFunc("POST /v1/projects", handleCreateProject(db, logger))
	mux.HandleFunc("GET /v1/projects", handleListProjects(db, logger))

	// Repository registration.
	mux.HandleFunc("POST /v1/repos", handleCreateRepo(db, logger))
	mux.HandleFunc("GET /v1/repos", handleListRepos(db, logger))

	// Worktree registration.
	mux.HandleFunc("POST /v1/worktrees", handleRegisterWorktree(db, logger))
	mux.HandleFunc("GET /v1/worktrees", handleListWorktrees(db, logger))

	// Artifact root registration.
	mux.HandleFunc("POST /v1/artifact-roots", handleCreateArtifactRoot(db, logger))
	mux.HandleFunc("GET /v1/artifact-roots", handleListArtifactRoots(db, logger))

	// Artifact registration.
	mux.HandleFunc("POST /v1/artifacts", handleCreateArtifact(db, logger))
	mux.HandleFunc("GET /v1/artifacts", handleListArtifacts(db, logger))

	// Issue registration.
	mux.HandleFunc("POST /v1/issues", handleCreateIssue(db, logger))
	mux.HandleFunc("GET /v1/issues/ready", handleListReadyIssues(db, logger))
	mux.HandleFunc("GET /v1/issues/{issue_id}", handleGetIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/claim", handleClaimIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/heartbeat", handleHeartbeatLease(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/release", handleReleaseLease(db, logger))
	mux.HandleFunc("PATCH /v1/issues/{issue_id}", handleUpdateIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/close", handleCloseIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/dependencies", handleAddDependency(db, logger))
	mux.HandleFunc("DELETE /v1/issues/{issue_id}/dependencies/{depends_on}", handleRemoveDependency(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/links", handleLinkArtifact(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/notes", handleCreateNote(db, logger))
	mux.HandleFunc("GET /v1/issues/{issue_id}/notes", handleListNotes(db, logger))
	mux.HandleFunc("GET /v1/issues/{issue_id}/events", handleListEvents(db, logger))
	mux.HandleFunc("GET /v1/issues", handleListIssues(db, logger))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("daemon started", "socket", cfg.SocketPath, "db", cfg.DBPath)
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve http: %w", err)
		}

		return nil
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("path exists and is not a socket: %s", path)
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove stale socket: %w", err)
		}

		return nil
	}

	if os.IsNotExist(err) {
		return nil
	}

	return fmt.Errorf("stat socket path: %w", err)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
