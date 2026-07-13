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
	Sequence    int64  `json:"sequence"`
	ID          string `json:"id"`
	IssueID     string `json:"issue_id,omitempty"`
	Actor       string `json:"actor"`
	EventType   string `json:"event_type"`
	PayloadJSON string `json:"payload_json"`
	CreatedAt   string `json:"created_at"`
}

// EventPage is a cursor-paginated page of global events.
type EventPage struct {
	Events    []Event `json:"events"`
	NextSince string  `json:"next_since"`
}

// IssueTypes lists the valid values for an issue's issue_type.
var IssueTypes = []string{"task", "bug", "feature", "epic", "chore"}

// ValidIssueType reports whether t is a known issue type.
func ValidIssueType(t string) bool {
	for _, v := range IssueTypes {
		if t == v {
			return true
		}
	}
	return false
}

// Issue represents a task or work item in a project.
type Issue struct {
	ID                 string       `json:"id"`
	ShortID            string       `json:"short_id"`
	ProjectID          string       `json:"project_id"`
	RepositoryID       string       `json:"repository_id,omitempty"`
	WorktreeID         string       `json:"worktree_id,omitempty"`
	ScopeKind          string       `json:"scope_kind"`
	IssueType          string       `json:"issue_type"`
	Title              string       `json:"title"`
	ExternalKey        string       `json:"external_key,omitempty"`
	Description        string       `json:"description,omitempty"`
	AcceptanceCriteria string       `json:"acceptance_criteria,omitempty"`
	Status             string       `json:"status"`
	Priority           int          `json:"priority"`
	Assignee           string       `json:"assignee,omitempty"`
	Version            int          `json:"version"`
	ClaimedAt          string       `json:"claimed_at,omitempty"`
	Holder             string       `json:"holder,omitempty"`
	LeaseExpiresAt     string       `json:"lease_expires_at,omitempty"`
	ClosedAt           string       `json:"closed_at,omitempty"`
	CreatedAt          string       `json:"created_at"`
	UpdatedAt          string       `json:"updated_at"`
	Dependencies       []Dependency `json:"dependencies,omitempty"`
}

// Dependency represents a relationship to another issue.
type Dependency struct {
	IssueID          string `json:"issue_id"`
	IssueShortID     string `json:"issue_short_id"`
	DependsOnID      string `json:"depends_on_id"`
	DependsOnShortID string `json:"depends_on_short_id"`
	Kind             string `json:"kind"`
}

// IssueLease represents the current lease on an issue (included in responses).
type IssueLease struct {
	Holder     string `json:"holder"`
	LeaseToken string `json:"-"`
	ExpiresAt  string `json:"expires_at"`
	AttemptID  string `json:"attempt_id"`
	SessionID  string `json:"session_id,omitempty"`
}

// CreateIssueRequest is the JSON body for POST /v1/issues.
type CreateIssueRequest struct {
	Project            string `json:"project"`
	ScopeKind          string `json:"scope_kind"`
	IssueType          string `json:"issue_type,omitempty"`
	Repo               string `json:"repo,omitempty"`
	Worktree           string `json:"worktree,omitempty"`
	Title              string `json:"title"`
	ExternalKey        string `json:"external_key,omitempty"`
	Description        string `json:"description,omitempty"`
	AcceptanceCriteria string `json:"acceptance_criteria,omitempty"`
	Priority           int    `json:"priority,omitempty"`
	Actor              string `json:"actor,omitempty"`
}

// IssueListParams represents query params for GET /v1/issues.
type IssueListParams struct {
	Project     string
	Repo        string
	Worktree    string
	Status      string
	Assignee    string
	IssueType   string
	ExternalKey string
}

// ClaimRequest is the JSON body for POST /v1/issues/{issue_id}/claim.
type ClaimRequest struct {
	Holder     string `json:"holder"`
	TTLSeconds int    `json:"ttl_seconds"`
	SessionID  string `json:"session_id,omitempty"`
}

// ClaimResponse is returned on successful claim.
type ClaimResponse struct {
	LeaseToken string `json:"lease_token"`
	ExpiresAt  string `json:"expires_at"`
	AttemptID  string `json:"attempt_id"`
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
	Title              string `json:"title,omitempty"`
	IssueType          string `json:"issue_type,omitempty"`
	ExternalKey        string `json:"external_key,omitempty"`
	Description        string `json:"description,omitempty"`
	AcceptanceCriteria string `json:"acceptance_criteria,omitempty"`
	Priority           int    `json:"priority,omitempty"`
	Assignee           string `json:"assignee,omitempty"`
	Status             string `json:"status,omitempty"`
	ExpectedVersion    int    `json:"expected_version"`
	LeaseToken         string `json:"lease_token,omitempty"`
	Actor              string `json:"actor,omitempty"`
}

// CloseIssueRequest is the JSON body for POST /v1/issues/{issue_id}/close.
type CloseIssueRequest struct {
	Resolution      string `json:"resolution"`
	Branch          string `json:"branch,omitempty"`
	PRURL           string `json:"pr_url,omitempty"`
	CommitSHA       string `json:"commit_sha,omitempty"`
	ExpectedVersion int    `json:"expected_version"`
	LeaseToken      string `json:"lease_token"`
	Actor           string `json:"actor,omitempty"`
	Note            string `json:"note,omitempty"`
}

// OperatorCloseIssueRequest closes an issue through the explicit local
// operator path. It never accepts a lease token.
type OperatorCloseIssueRequest struct {
	Resolution      string `json:"resolution"`
	ExpectedVersion int    `json:"expected_version"`
	Actor           string `json:"actor"`
	Reason          string `json:"reason"`
}

// OperatorReopenIssueRequest reopens terminal work through the explicit local
// operator path. It never accepts a lease token.
type OperatorReopenIssueRequest struct {
	ExpectedVersion int    `json:"expected_version"`
	Actor           string `json:"actor"`
	Reason          string `json:"reason"`
}

// CloseIssueResult is returned after a successful close.
type CloseIssueResult struct {
	Status      string `json:"status"`
	Resolution  string `json:"resolution"`
	Branch      string `json:"branch,omitempty"`
	PRURL       string `json:"pr_url,omitempty"`
	CommitSHA   string `json:"commit_sha,omitempty"`
	ExternalKey string `json:"external_key,omitempty"`
	ClosedAt    string `json:"closed_at"`
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

// UnlinkArtifactRequest holds the query params for DELETE /v1/issues/{issue_id}/links.
type UnlinkArtifactRequest struct {
	Artifact string // artifact ID or relative path
	Relation string // if empty, removes every relation to the artifact
	Actor    string
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
	if req.IssueType != "" && !ValidIssueType(req.IssueType) {
		errs = append(errs, "issue_type must be one of: "+strings.Join(IssueTypes, ", "))
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
		"done":        {"open"},
		"cancelled":   {"open"},
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
