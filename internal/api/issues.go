package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/abevz/af-coordinator/internal/store/sqlite"
)

func handleCreateIssue(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req core.CreateIssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if err := core.ValidateCreateIssue(req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, err.Error())
			return
		}

		if req.Actor == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "actor is required")
			return
		}

		issue, err := sqlite.CreateIssue(r.Context(), db, req.Project, req)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				if apiErr.Code == core.ErrNotFound {
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				}
			}
			if isUniqueConstraintError(err) {
				writeError(w, http.StatusConflict, "short_id_taken",
					"an issue with this short_id already exists")
				return
			}
			logger.Error("failed to create issue", "project", req.Project, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create issue")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]core.Issue{"issue": issue})
	}
}

// resolveIssueID resolves the issue_id path parameter (supports both UUID and short_id)
// and writes an error response if the issue is not found. Returns the UUID id and true on success.
func resolveIssueID(db *sql.DB, w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := sqlite.ResolveIssueID(r.Context(), db, r.PathValue("issue_id"))
	if err != nil {
		if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
			writeError(w, http.StatusNotFound, core.ErrNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve issue")
		}
		return "", false
	}
	return id, true
}

func handleGetIssue(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		issue, lease, err := sqlite.GetIssue(r.Context(), db, issueID)
		if err != nil {
			logger.Error("failed to get issue", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get issue")
			return
		}

		resp := map[string]interface{}{
			"issue": issue,
		}
		if lease != nil {
			resp["lease"] = lease
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func handleListIssues(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := core.IssueListParams{
			Project:  r.URL.Query().Get("project"),
			Repo:     r.URL.Query().Get("repo"),
			Worktree: r.URL.Query().Get("worktree"),
			Status:   r.URL.Query().Get("status"),
			Assignee: r.URL.Query().Get("assignee"),
		}

		issues, err := sqlite.ListIssues(r.Context(), db, params)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
				return
			}
			logger.Error("failed to list issues", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list issues")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Issue{"issues": issues})
	}
}

func handleClaimIssue(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.ClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.Holder == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "holder is required")
			return
		}
		if req.TTLSeconds <= 0 {
			req.TTLSeconds = 3600
		}

		resp, err := sqlite.ClaimIssue(r.Context(), db, issueID, req.Holder, req.TTLSeconds)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrLeaseHeld:
					writeError(w, http.StatusConflict, core.ErrLeaseHeld, apiErr.Message)
					return
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				}
			}
			logger.Error("failed to claim issue", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to claim issue")
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func handleHeartbeatLease(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.LeaseToken == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "lease_token is required")
			return
		}
		if req.TTLSeconds <= 0 {
			req.TTLSeconds = 3600
		}

		newExpiresAt, err := sqlite.HeartbeatLease(r.Context(), db, issueID, req.LeaseToken, req.TTLSeconds)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrLeaseExpired:
					writeError(w, http.StatusGone, core.ErrLeaseExpired, apiErr.Message)
					return
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				}
			}
			logger.Error("failed to heartbeat lease", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to heartbeat lease")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"expires_at": newExpiresAt})
	}
}

func handleReleaseLease(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.ReleaseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.LeaseToken == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "lease_token is required")
			return
		}

		err := sqlite.ReleaseLease(r.Context(), db, issueID, req.LeaseToken)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrLeaseExpired:
					writeError(w, http.StatusGone, core.ErrLeaseExpired, apiErr.Message)
					return
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				}
			}
			logger.Error("failed to release lease", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to release lease")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListReadyIssues(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectFilter := r.URL.Query().Get("project")

		// Resolve project key to ID if provided.
		var projectID string
		if projectFilter != "" {
			proj, err := sqlite.GetProjectByKey(r.Context(), db, projectFilter)
			if err != nil {
				if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
					writeError(w, http.StatusNotFound, core.ErrNotFound,
						"project not found: "+projectFilter)
					return
				}
				logger.Error("failed to resolve project for ready issues", "project", projectFilter, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve project")
				return
			}
			projectID = proj.ID
		}

		issues, err := sqlite.ListReadyIssues(r.Context(), db, projectID)
		if err != nil {
			logger.Error("failed to list ready issues", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list ready issues")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Issue{"issues": issues})
	}
}

func handleUpdateIssue(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.UpdateIssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.Actor == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "actor is required")
			return
		}

		updated, err := sqlite.UpdateIssue(r.Context(), db, issueID, req)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrConflict:
					writeError(w, http.StatusConflict, core.ErrConflict, apiErr.Message)
					return
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				case core.ErrLeaseExpired:
					writeError(w, http.StatusGone, core.ErrLeaseExpired, apiErr.Message)
					return
				case core.ErrValidationFailed:
					writeError(w, http.StatusBadRequest, core.ErrValidationFailed, apiErr.Message)
					return
				}
			}
			logger.Error("failed to update issue", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update issue")
			return
		}

		writeJSON(w, http.StatusOK, map[string]core.Issue{"issue": updated})
	}
}

func handleCloseIssue(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.CloseIssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.Actor == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "actor is required")
			return
		}

		err := sqlite.CloseIssue(r.Context(), db, issueID, req)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrConflict:
					writeError(w, http.StatusConflict, core.ErrConflict, apiErr.Message)
					return
				case core.ErrLeaseExpired:
					writeError(w, http.StatusGone, core.ErrLeaseExpired, apiErr.Message)
					return
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				case core.ErrValidationFailed:
					writeError(w, http.StatusBadRequest, core.ErrValidationFailed, apiErr.Message)
					return
				}
			}
			logger.Error("failed to close issue", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to close issue")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
	}
}

func handleAddDependency(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.AddDependencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.DependsOn == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "depends_on is required")
			return
		}
		if req.Kind == "" {
			req.Kind = "blocks"
		}

		if req.Actor == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "actor is required")
			return
		}

		err := sqlite.AddDependency(r.Context(), db, issueID, req)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrDependencyCycle:
					writeError(w, http.StatusUnprocessableEntity, core.ErrDependencyCycle, apiErr.Message)
					return
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				case core.ErrConflict:
					writeError(w, http.StatusConflict, core.ErrConflict, apiErr.Message)
					return
				}
			}
			logger.Error("failed to add dependency", "issue_id", issueID, "depends_on", req.DependsOn, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to add dependency")
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func handleRemoveDependency(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}
		dependsOn := r.PathValue("depends_on")
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "blocks"
		}

		actor := r.URL.Query().Get("actor")

		err := sqlite.RemoveDependency(r.Context(), db, issueID, dependsOn, kind, actor)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
				return
			}
			logger.Error("failed to remove dependency", "issue_id", issueID, "depends_on", dependsOn, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to remove dependency")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func handleLinkArtifact(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.LinkArtifactRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.Artifact == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "artifact is required")
			return
		}

		createdAt, err := sqlite.LinkArtifact(r.Context(), db, issueID, req)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
					return
				case core.ErrAlreadyLinked:
					writeError(w, http.StatusConflict, core.ErrAlreadyLinked, apiErr.Message)
					return
				}
			}
			logger.Error("failed to link artifact", "issue_id", issueID, "artifact", req.Artifact, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to link artifact")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"created_at": createdAt})
	}
}

func handleListIssueLinks(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		links, err := sqlite.ListIssueArtifacts(r.Context(), db, issueID)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound, apiErr.Message)
				return
			}
			logger.Error("failed to list issue links", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list issue links")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.ArtifactRef{"links": links})
	}
}

func handleCreateNote(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		var req core.CreateNoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "invalid JSON body")
			return
		}

		if req.Author == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "author is required")
			return
		}
		if req.Body == "" {
			writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "body is required")
			return
		}

		note, err := sqlite.CreateNote(r.Context(), db, issueID, req)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound, "issue not found: "+issueID)
				return
			}
			logger.Error("failed to create note", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create note")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]core.Note{"note": note})
	}
}

func handleListNotes(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		notes, err := sqlite.ListNotes(r.Context(), db, issueID)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound, "issue not found: "+issueID)
				return
			}
			logger.Error("failed to list notes", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list notes")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Note{"notes": notes})
	}
}

func handleListEvents(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issueID, ok := resolveIssueID(db, w, r)
		if !ok {
			return
		}

		events, err := sqlite.ListEvents(r.Context(), db, issueID)
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok && apiErr.Code == core.ErrNotFound {
				writeError(w, http.StatusNotFound, core.ErrNotFound, "issue not found: "+issueID)
				return
			}
			logger.Error("failed to list events", "issue_id", issueID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list events")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]core.Event{"events": events})
	}
}
