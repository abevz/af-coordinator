package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/abevz/af-coordinator/internal/store/sqlite"
)

func handleRegisterWorktree(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req core.CreateWorktreeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if err := core.ValidateCreateWorktree(req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, err.Error())
			return
		}

		// Resolve repo ID from the body.
		repo, err := sqlite.GetRepo(r.Context(), db, req.Repo)
		if err != nil {
			if writeRepoLookupError(w, err, req.Repo) {
				return
			}
			logger.Error("failed to resolve repo for worktree", "repo", req.Repo, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
			return
		}

		wt, isNew, err := sqlite.UpsertWorktree(r.Context(), db, repo.ID, req)
		if err != nil {
			logger.Error("failed to upsert worktree", "path", req.AbsolutePath, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to register worktree")
			return
		}

		status := http.StatusOK
		if isNew {
			status = http.StatusCreated
		}

		writeJSON(w, status, map[string]core.Worktree{"worktree": wt})
	}
}

func handleListWorktrees(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoFilter := r.URL.Query().Get("repo")

		var worktrees []core.Worktree
		var err error

		if repoFilter != "" {
			// Verify the repo exists first.
			if _, err := sqlite.GetRepo(r.Context(), db, repoFilter); err != nil {
				if writeRepoLookupError(w, err, repoFilter) {
					return
				}
				logger.Error("failed to resolve repo for worktree list", "repo", repoFilter, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
				return
			}
			worktrees, err = sqlite.ListWorktrees(r.Context(), db, repoFilter)
		} else {
			worktrees, err = sqlite.ListWorktrees(r.Context(), db, "")
		}

		if err != nil {
			logger.Error("failed to list worktrees", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list worktrees")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Worktree{"worktrees": worktrees})
	}
}

func handleDeleteWorktree(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		worktreeID := r.PathValue("worktree_id")
		if worktreeID == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "worktree_id is required")
			return
		}

		wt, err := sqlite.DeleteWorktree(r.Context(), db, worktreeID)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				case core.ErrConflict:
					writeError(w, http.StatusConflict, core.ErrConflict, apiErr.Message)
					return
				}
			}

			logger.Error("failed to delete worktree", "worktree_id", worktreeID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete worktree")
			return
		}

		writeJSON(w, http.StatusOK, map[string]core.Worktree{"worktree": wt})
	}
}
