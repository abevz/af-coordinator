package core

import (
	"fmt"
	"strings"
)

// Issue represents a task or work item in a project.
type Issue struct {
	ID           string `json:"id"`
	ShortID      string `json:"short_id"`
	ProjectID    string `json:"project_id"`
	RepositoryID string `json:"repository_id,omitempty"`
	WorktreeID   string `json:"worktree_id,omitempty"`
	ScopeKind    string `json:"scope_kind"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Status       string `json:"status"`
	Priority     int    `json:"priority"`
	Assignee     string `json:"assignee,omitempty"`
	Version      int    `json:"version"`
	ClaimedAt    string `json:"claimed_at,omitempty"`
	ClosedAt     string `json:"closed_at,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
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
}

// IssueListParams represents query params for GET /v1/issues.
type IssueListParams struct {
	Project  string
	Repo     string
	Worktree string
	Status   string
	Assignee string
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
