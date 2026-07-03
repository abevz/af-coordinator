package sqlite

import (
	"errors"
	"testing"

	"github.com/abevz/af-coordinator/internal/core"
)

func TestCreateArtifactRoot(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "my-repo",
		CanonicalGitDir: "/repos/my-repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	root, err := CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/specs",
		Kind:     "sdd",
		Primary:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if root.ID == "" {
		t.Error("expected non-empty artifact root ID")
	}
	if root.RepositoryID != repo.ID {
		t.Errorf("expected repository_id %q, got %q", repo.ID, root.RepositoryID)
	}
	if root.RootPath != "docs/specs" {
		t.Errorf("expected root_path 'docs/specs', got %q", root.RootPath)
	}
	if root.Kind != "sdd" {
		t.Errorf("expected kind 'sdd', got %q", root.Kind)
	}
	if !root.IsPrimary {
		t.Error("expected artifact root to be primary")
	}
}

func TestCreateArtifactRootDefaultsKind(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	root, err := CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/adr",
		// Kind defaults to "sdd"
	})
	if err != nil {
		t.Fatal(err)
	}
	if root.Kind != "sdd" {
		t.Errorf("expected default kind 'sdd', got %q", root.Kind)
	}
}

func TestCreateDuplicateArtifactRoot(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/specs",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/specs",
	})
	if err == nil {
		t.Fatal("expected error for duplicate artifact root path")
	}
	if !isSQLiteConstraintError(err) {
		t.Fatalf("expected a constraint error, got %T: %v", err, err)
	}
}

func TestGetArtifactRoot(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/design",
		Kind:     "design",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := GetArtifactRoot(db, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, got.ID)
	}
	if got.RootPath != "docs/design" {
		t.Errorf("expected root_path 'docs/design', got %q", got.RootPath)
	}
}

func TestGetArtifactRootNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := GetArtifactRoot(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent artifact root")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrNotFound {
		t.Errorf("expected code %q, got %q", core.ErrNotFound, apiErr.Code)
	}
}

func TestListArtifactRoots(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	// No roots yet.
	roots, err := ListArtifactRoots(db, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Errorf("expected 0 artifact roots, got %d", len(roots))
	}

	CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/specs",
	})
	CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/adr",
	})

	roots, err = ListArtifactRoots(db, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 {
		t.Errorf("expected 2 artifact roots, got %d", len(roots))
	}
}

func TestListArtifactRootsByRepo(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo1, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo-1",
		CanonicalGitDir: "/repos/repo-1.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo2, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo-2",
		CanonicalGitDir: "/repos/repo-2.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	CreateArtifactRoot(db, repo1.ID, core.CreateArtifactRootRequest{
		Repo:     repo1.ID,
		RootPath: "docs/r1",
	})
	CreateArtifactRoot(db, repo2.ID, core.CreateArtifactRootRequest{
		Repo:     repo2.ID,
		RootPath: "docs/r2",
	})

	roots, err := ListArtifactRoots(db, repo1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Errorf("expected 1 root for repo1, got %d", len(roots))
	}
}

func TestCreateArtifact(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	art, err := CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "spec",
		RelativePath: "docs/specs/api-v1.md",
		Title:        "API v1 Spec",
		Status:       "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.ID == "" {
		t.Error("expected non-empty artifact ID")
	}
	if art.RepositoryID != repo.ID {
		t.Errorf("expected repository_id %q, got %q", repo.ID, art.RepositoryID)
	}
	if art.Kind != "spec" {
		t.Errorf("expected kind 'spec', got %q", art.Kind)
	}
	if art.RelativePath != "docs/specs/api-v1.md" {
		t.Errorf("expected relative_path 'docs/specs/api-v1.md', got %q", art.RelativePath)
	}
	if art.Title != "API v1 Spec" {
		t.Errorf("expected title 'API v1 Spec', got %q", art.Title)
	}
	if art.Status != "draft" {
		t.Errorf("expected status 'draft', got %q", art.Status)
	}
}

func TestCreateArtifactWithRoot(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	root, err := CreateArtifactRoot(db, repo.ID, core.CreateArtifactRootRequest{
		Repo:     repo.ID,
		RootPath: "docs/specs",
	})
	if err != nil {
		t.Fatal(err)
	}

	art, err := CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:           repo.ID,
		Kind:           "spec",
		RelativePath:   "docs/specs/api.md",
		ArtifactRootID: root.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.ArtifactRootID != root.ID {
		t.Errorf("expected artifact_root_id %q, got %q", root.ID, art.ArtifactRootID)
	}
}

func TestCreateDuplicateArtifact(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "spec",
		RelativePath: "docs/spec.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "spec",
		RelativePath: "docs/spec.md",
	})
	if err == nil {
		t.Fatal("expected error for duplicate artifact relative_path")
	}
	if !isSQLiteConstraintError(err) {
		t.Fatalf("expected a constraint error, got %T: %v", err, err)
	}
}

func TestGetArtifact(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "adr",
		RelativePath: "docs/adr/001-choice.md",
		Title:        "ADR 001",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := GetArtifact(db, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, got.ID)
	}
	if got.RelativePath != "docs/adr/001-choice.md" {
		t.Errorf("expected relative_path 'docs/adr/001-choice.md', got %q", got.RelativePath)
	}
}

func TestGetArtifactNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := GetArtifact(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent artifact")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrNotFound {
		t.Errorf("expected code %q, got %q", core.ErrNotFound, apiErr.Code)
	}
}

func TestListArtifacts(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	// No artifacts yet.
	artifacts, err := ListArtifacts(db, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(artifacts))
	}

	CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "spec",
		RelativePath: "docs/specs/a.md",
	})
	CreateArtifact(db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "adr",
		RelativePath: "docs/adr/b.md",
	})

	artifacts, err = ListArtifacts(db, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(artifacts))
	}
}

func TestListArtifactsByRepo(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo1, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo-1",
		CanonicalGitDir: "/repos/repo-1.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo2, _, err := CreateRepo(db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo-2",
		CanonicalGitDir: "/repos/repo-2.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	CreateArtifact(db, repo1.ID, core.CreateArtifactRequest{
		Repo:         repo1.ID,
		Kind:         "spec",
		RelativePath: "docs/r1.md",
	})
	CreateArtifact(db, repo2.ID, core.CreateArtifactRequest{
		Repo:         repo2.ID,
		Kind:         "spec",
		RelativePath: "docs/r2.md",
	})

	artifacts, err := ListArtifacts(db, repo1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Errorf("expected 1 artifact for repo1, got %d", len(artifacts))
	}
}
