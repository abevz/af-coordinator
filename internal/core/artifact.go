package core

import (
	"fmt"
	"strings"
)

// ArtifactRoot represents a repository-local artifact root area such as "docs/specs/".
type ArtifactRoot struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	RootPath     string `json:"root_path"`
	Kind         string `json:"kind"`
	IsPrimary    bool   `json:"is_primary"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// Artifact represents a concrete file inside an artifact root.
type Artifact struct {
	ID             string `json:"id"`
	RepositoryID   string `json:"repository_id"`
	WorktreeID     string `json:"worktree_id,omitempty"`
	ArtifactRootID string `json:"artifact_root_id,omitempty"`
	Kind           string `json:"kind"`
	RelativePath   string `json:"relative_path"`
	Title          string `json:"title,omitempty"`
	ExternalKey    string `json:"external_key,omitempty"`
	Status         string `json:"status,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// CreateArtifactRootRequest is the JSON body for POST /v1/artifact-roots.
type CreateArtifactRootRequest struct {
	Repo     string `json:"repo"`
	RootPath string `json:"root_path"`
	Kind     string `json:"kind,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
}

// CreateArtifactRequest is the JSON body for POST /v1/artifacts.
type CreateArtifactRequest struct {
	Repo           string `json:"repo"`
	Worktree       string `json:"worktree,omitempty"`
	ArtifactRootID string `json:"artifact_root_id,omitempty"`
	Kind           string `json:"kind"`
	RelativePath   string `json:"relative_path"`
	Title          string `json:"title,omitempty"`
	ExternalKey    string `json:"external_key,omitempty"`
	Status         string `json:"status,omitempty"`
}

// ValidateCreateArtifactRoot checks required fields for a new artifact root.
func ValidateCreateArtifactRoot(req CreateArtifactRootRequest) error {
	var errs []string
	if req.Repo == "" {
		errs = append(errs, "repo is required")
	}
	if req.RootPath == "" {
		errs = append(errs, "root_path is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation_failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateCreateArtifact checks required fields for a new artifact.
func ValidateCreateArtifact(req CreateArtifactRequest) error {
	var errs []string
	if req.Repo == "" {
		errs = append(errs, "repo is required")
	}
	if req.RelativePath == "" {
		errs = append(errs, "relative_path is required")
	}
	if req.Kind == "" {
		errs = append(errs, "kind is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation_failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateArtifactKind checks that the artifact kind is one of the known types.
func ValidateArtifactKind(kind string) bool {
	switch kind {
	case "requirements",
		"design",
		"tasks",
		"review",
		"adr",
		"decision",
		"spec",
		"glossary",
		"traceability",
		"readme",
		"schema",
		"config",
		"code",
		"test",
		"doc",
		"other":
		return true
	}
	return false
}
