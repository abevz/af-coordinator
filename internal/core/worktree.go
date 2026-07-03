package core

import (
	"fmt"
	"strings"
)

// Worktree represents a concrete checkout on disk.
type Worktree struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	AbsolutePath string `json:"absolute_path"`
	Branch       string `json:"branch"`
	HeadCommit   string `json:"head_commit,omitempty"`
	RemoteName   string `json:"remote_name,omitempty"`
	RemoteBranch string `json:"remote_branch,omitempty"`
	IsMain       bool   `json:"is_main"`
	IsEphemeral  bool   `json:"is_ephemeral"`
	LastSeenAt   string `json:"last_seen_at"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CreateWorktreeRequest is the JSON body for POST /v1/worktrees.
type CreateWorktreeRequest struct {
	Repo         string `json:"repo"`
	AbsolutePath string `json:"absolute_path"`
	Branch       string `json:"branch,omitempty"`
	HeadCommit   string `json:"head_commit,omitempty"`
	RemoteName   string `json:"remote_name,omitempty"`
	RemoteBranch string `json:"remote_branch,omitempty"`
	IsMain       bool   `json:"is_main,omitempty"`
	IsEphemeral  bool   `json:"is_ephemeral,omitempty"`
}

// ValidateCreateWorktree checks required fields for worktree registration.
func ValidateCreateWorktree(req CreateWorktreeRequest) error {
	var errs []string
	if req.Repo == "" {
		errs = append(errs, "repo is required")
	}
	if req.AbsolutePath == "" {
		errs = append(errs, "absolute_path is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation_failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
