package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/google/uuid"
)

// CreateRepo inserts a new repository and its remotes in a transaction.
// The projectKey is resolved to a project ID internally.
func CreateRepo(db *sql.DB, projectKey string, req core.CreateRepoRequest) (core.Repository, []core.RepoRemote, error) {
	// Resolve project key to ID.
	proj, err := GetProjectByKey(db, projectKey)
	if err != nil {
		return core.Repository{}, nil, fmt.Errorf("resolve project: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	repoID := uuid.New().String()

	tx, err := db.Begin()
	if err != nil {
		return core.Repository{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, hosting_kind, hosting_slug, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, '', '', ?, ?)`,
		repoID, proj.ID, req.LogicalName, req.CanonicalGitDir, req.DefaultBranch, now, now,
	)
	if err != nil {
		return core.Repository{}, nil, fmt.Errorf("insert repo: %w", err)
	}

	var remotes []core.RepoRemote
	for _, r := range req.Remotes {
		remoteID := uuid.New().String()
		isPrimary := 0
		if r.IsPrimary {
			isPrimary = 1
		}
		_, err := tx.Exec(
			`INSERT INTO repo_remotes (id, repository_id, remote_name, fetch_url, push_url, is_primary, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			remoteID, repoID, r.RemoteName, r.FetchURL, r.PushURL, isPrimary, now, now,
		)
		if err != nil {
			return core.Repository{}, nil, fmt.Errorf("insert remote %s: %w", r.RemoteName, err)
		}
		remotes = append(remotes, core.RepoRemote{
			ID:           remoteID,
			RepositoryID: repoID,
			RemoteName:   r.RemoteName,
			FetchURL:     r.FetchURL,
			PushURL:      r.PushURL,
			IsPrimary:    r.IsPrimary,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	if err := tx.Commit(); err != nil {
		return core.Repository{}, nil, fmt.Errorf("commit tx: %w", err)
	}

	repo := core.Repository{
		ID:              repoID,
		ProjectID:       proj.ID,
		LogicalName:     req.LogicalName,
		CanonicalGitDir: req.CanonicalGitDir,
		DefaultBranch:   req.DefaultBranch,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return repo, remotes, nil
}

// GetRepo retrieves a repository by ID or logical name.
func GetRepo(db *sql.DB, idOrName string) (core.Repository, error) {
	row := db.QueryRow(
		`SELECT id, project_id, logical_name, canonical_git_dir, default_branch, hosting_kind, hosting_slug, created_at, updated_at
		 FROM repositories WHERE id = ? OR logical_name = ?`, idOrName, idOrName,
	)
	return scanRepo(row)
}

// ListRepos lists repositories optionally filtered by project ID.
func ListRepos(db *sql.DB, projectID string) ([]core.Repository, error) {
	var rows *sql.Rows
	var err error

	if projectID != "" {
		rows, err = db.Query(
			`SELECT id, project_id, logical_name, canonical_git_dir, default_branch, hosting_kind, hosting_slug, created_at, updated_at
			 FROM repositories WHERE project_id = ? ORDER BY logical_name`, projectID,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, project_id, logical_name, canonical_git_dir, default_branch, hosting_kind, hosting_slug, created_at, updated_at
			 FROM repositories ORDER BY logical_name`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()

	var repos []core.Repository
	for rows.Next() {
		r, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repos: %w", err)
	}
	if repos == nil {
		repos = []core.Repository{}
	}
	return repos, nil
}

// ListReposByProjectKey lists repositories by project key (resolved internally).
func ListReposByProjectKey(db *sql.DB, projectKey string) ([]core.Repository, error) {
	proj, err := GetProjectByKey(db, projectKey)
	if err != nil {
		return nil, err
	}
	return ListRepos(db, proj.ID)
}

func scanRepo(s scanner) (core.Repository, error) {
	var r core.Repository
	err := s.Scan(&r.ID, &r.ProjectID, &r.LogicalName, &r.CanonicalGitDir, &r.DefaultBranch, &r.HostingKind, &r.HostingSlug, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.Repository{}, core.NewAPIError(core.ErrNotFound, "repository not found")
		}
		return core.Repository{}, fmt.Errorf("scan repo: %w", err)
	}
	return r, nil
}
