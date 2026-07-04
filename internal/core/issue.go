package core

import (
	"fmt"
	"strings"
)

// Note represents a comment attached to an issue.
type Note struct {
	ID        string `json:"id"`
	IssueID   string `json:"issue_id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// CreateNoteRequest is the JSON body for POST /v1/issues/{issue_id}/notes.
type CreateNoteRequest struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

// Event represents an event in the issue activity timeline.
type Event struct {
	ID          string `json:"id"`
	IssueID     string `json:"issue_id,omitempty"`
	Actor       string `json:"actor"`
	EventType   string `json:"event_type"`
	PayloadJSON string `json:"payload_json"`
	CreatedAt   string `json:"created_at"`
}

// Issue represents a task or work item in a project.
type Issue struct {
	ID             string `json:"id"`
	ShortID        string `json:"short_id"`
	ProjectID      string `json:"project_id"`
	RepositoryID   string `json:"repository_id,omitempty"`
	WorktreeID     string `json:"worktree_id,omitempty"`
	ScopeKind      string `json:"scope_kind"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Status         string `json:"status"`
	Priority       int    `json:"priority"`
	Assignee       string `json:"assignee,omitempty"`
	Version        int    `json:"version"`
	ClaimedAt      string `json:"claimed_at,omitempty"`
	Holder         string `json:"holder,omitempty"`
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
	ClosedAt       string `json:"closed_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// IssueLease represents the current lease on an issue (included in responses).
type IssueLease struct {
	Holder     string `json:"holder"`
	LeaseToken string `json:"lease_token"`
	ExpiresAt  string `json:"expires_at"`
}

// CreateIssueRequest is the JSON body for POST /v1/issues.
type CreateIssueRequest struct {
	Project     string `json:"project"`
	ScopeKind   string `json:"scope_kind"`
	Repo        string `json:"repo,omitempty"`
	Worktree    string `json:"worktree,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Actor       string `json:"actor,omitempty"`
}

// IssueListParams represents query params for GET /v1/issues.
type IssueListParams struct {
	Project  string
	Repo     string
	Worktree string
	Status   string
	Assignee string
}

// ClaimRequest is the JSON body for POST /v1/issues/{issue_id}/claim.
type ClaimRequest struct {
	Holder     string `json:"holder"`
	TTLSeconds int    `json:"ttl_seconds"`
}

// ClaimResponse is returned on successful claim.
type ClaimResponse struct {
	LeaseToken string `json:"lease_token"`
	ExpiresAt  string `json:"expires_at"`
}

// HeartbeatRequest is the JSON body for POST /v1/issues/{issue_id}/heartbeat.
type HeartbeatRequest struct {
	LeaseToken string `json:"lease_token"`
	TTLSeconds int    `json:"ttl_seconds"`
}

// ReleaseRequest is the JSON body for POST /v1/issues/{issue_id}/release.
type ReleaseRequest struct {
	LeaseToken string `json:"lease_token"`
}

// UpdateIssueRequest is the JSON body for PATCH /v1/issues/{issue_id}.
type UpdateIssueRequest struct {
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	Assignee        string `json:"assignee,omitempty"`
	Status          string `json:"status,omitempty"`
	ExpectedVersion int    `json:"expected_version"`
	LeaseToken      string `json:"lease_token,omitempty"`
	Actor           string `json:"actor,omitempty"`
}

// CloseIssueRequest is the JSON body for POST /v1/issues/{issue_id}/close.
type CloseIssueRequest struct {
	Resolution      string `json:"resolution"`
	ExpectedVersion int    `json:"expected_version"`
	LeaseToken      string `json:"lease_token"`
	Actor           string `json:"actor,omitempty"`
	Note            string `json:"note,omitempty"`
}

// AddDependencyRequest is the JSON body for POST /v1/issues/{issue_id}/dependencies.
type AddDependencyRequest struct {
	DependsOn string `json:"depends_on"`
	Kind      string `json:"kind"`
	Actor     string `json:"actor,omitempty"`
}

// RemoveDependencyRequest holds the path and query params for DELETE /v1/issues/{issue_id}/dependencies/{depends_on}.
type RemoveDependencyRequest struct {
	DependsOn string
	Kind      string
	Actor     string
}

// LinkArtifactRequest is the JSON body for POST /v1/issues/{issue_id}/links.
type LinkArtifactRequest struct {
	Artifact string `json:"artifact"`           // artifact ID or relative path
	Relation string `json:"relation,omitempty"` // default: "implements"
}

// ArtifactRef is a linked artifact with relation info.
type ArtifactRef struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	Kind         string `json:"kind"`
	Relation     string `json:"relation"`
}

// ValidateCreateIssue checks required fields for creating an issue.
func ValidateCreateIssue(req CreateIssueRequest) error {
	var errs []string
	if req.Project == "" {
		errs = append(errs, "project is required")
	}
	if req.ScopeKind == "" {
		errs = append(errs, "scope_kind is required")
	} else if req.ScopeKind != "project" && req.ScopeKind != "repository" && req.ScopeKind != "worktree" {
		errs = append(errs, "scope_kind must be 'project', 'repository', or 'worktree'")
	}
	if req.Title == "" {
		errs = append(errs, "title is required")
	}
	// For repo/worktree scope, repo is required
	if req.ScopeKind != "project" && req.Repo == "" {
		errs = append(errs, "repo is required when scope_kind is not 'project'")
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation_failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateStatusTransition checks if a status transition is valid.
func ValidateStatusTransition(current, target string) error {
	validTransitions := map[string][]string{
		"open":        {"in_progress", "blocked", "deferred", "done", "cancelled"},
		"in_progress": {"open", "blocked", "deferred", "done", "cancelled"},
		"blocked":     {"open", "in_progress", "deferred", "done", "cancelled"},
		"deferred":    {"open", "in_progress", "blocked", "done", "cancelled"},
	}
	valid, ok := validTransitions[current]
	if !ok {
		return fmt.Errorf("validation_failed: invalid current status: %s", current)
	}
	for _, v := range valid {
		if target == v {
			return nil
		}
	}
	return fmt.Errorf("validation_failed: cannot transition from '%s' to '%s'", current, target)
}
