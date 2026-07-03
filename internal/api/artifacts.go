package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/abevz/af-coordinator/internal/store/sqlite"
)

func handleCreateArtifactRoot(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req core.CreateArtifactRootRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if err := core.ValidateCreateArtifactRoot(req); err != nil {
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
			logger.Error("failed to resolve repo for artifact root", "repo", req.Repo, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
			return
		}

		root, err := sqlite.CreateArtifactRoot(db, repo.ID, req)
		if err != nil {
			if isUniqueConstraintError(err) {
				writeError(w, http.StatusConflict, "artifact_root_exists",
					"an artifact root with this path already exists in the repository")
				return
			}
			logger.Error("failed to create artifact root", "path", req.RootPath, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create artifact root")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]core.ArtifactRoot{"artifact_root": root})
	}
}

func handleListArtifactRoots(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoFilter := r.URL.Query().Get("repo")

		var roots []core.ArtifactRoot
		var err error

		if repoFilter != "" {
			// Verify the repo exists first.
			if _, err := sqlite.GetRepo(db, repoFilter); err != nil {
				if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
					writeError(w, http.StatusNotFound, core.ErrNotFound,
						"repository not found: "+repoFilter)
					return
				}
				logger.Error("failed to resolve repo for artifact root list", "repo", repoFilter, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
				return
			}
			roots, err = sqlite.ListArtifactRoots(db, repoFilter)
		} else {
			roots, err = sqlite.ListArtifactRoots(db, "")
		}

		if err != nil {
			logger.Error("failed to list artifact roots", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list artifact roots")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.ArtifactRoot{"artifact_roots": roots})
	}
}

func handleCreateArtifact(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req core.CreateArtifactRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if err := core.ValidateCreateArtifact(req); err != nil {
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
			logger.Error("failed to resolve repo for artifact", "repo", req.Repo, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
			return
		}

		artifact, err := sqlite.CreateArtifact(db, repo.ID, req)
		if err != nil {
			if isUniqueConstraintError(err) {
				writeError(w, http.StatusConflict, "artifact_exists",
					"an artifact with this relative_path already exists in the repository")
				return
			}
			logger.Error("failed to create artifact", "path", req.RelativePath, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create artifact")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]core.Artifact{"artifact": artifact})
	}
}

func handleListArtifacts(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoFilter := r.URL.Query().Get("repo")

		var artifacts []core.Artifact
		var err error

		if repoFilter != "" {
			// Verify the repo exists first.
			if _, err := sqlite.GetRepo(db, repoFilter); err != nil {
				if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
					writeError(w, http.StatusNotFound, core.ErrNotFound,
						"repository not found: "+repoFilter)
					return
				}
				logger.Error("failed to resolve repo for artifact list", "repo", repoFilter, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve repository")
				return
			}
			artifacts, err = sqlite.ListArtifacts(db, repoFilter)
		} else {
			artifacts, err = sqlite.ListArtifacts(db, "")
		}

		if err != nil {
			logger.Error("failed to list artifacts", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list artifacts")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Artifact{"artifacts": artifacts})
	}
}
