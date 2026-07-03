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
		repo, err := sqlite.GetRepo(db, req.Repo)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound,
					"repository not found: "+req.Repo)
				return
			}
			logger.Error("failed to resolve repo for worktree", "repo", req.Repo, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
			return
		}

		wt, isNew, err := sqlite.UpsertWorktree(db, repo.ID, req)
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
			if _, err := sqlite.GetRepo(db, repoFilter); err != nil {
				if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
					writeError(w, http.StatusNotFound, core.ErrNotFound,
						"repository not found: "+repoFilter)
					return
				}
				logger.Error("failed to resolve repo for worktree list", "repo", repoFilter, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
				return
			}
			worktrees, err = sqlite.ListWorktrees(db, repoFilter)
		} else {
			worktrees, err = sqlite.ListWorktrees(db, "")
		}

		if err != nil {
			logger.Error("failed to list worktrees", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list worktrees")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Worktree{"worktrees": worktrees})
	}
}
