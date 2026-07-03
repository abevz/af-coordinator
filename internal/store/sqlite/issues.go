package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/google/uuid"
)

// CreateIssue inserts a new issue, allocating a short_id from the project's sequence.
func CreateIssue(db *sql.DB, projectKey string, req core.CreateIssueRequest) (core.Issue, error) {
	// Resolve project by key.
	proj, err := GetProjectByKey(db, projectKey)
	if err != nil {
		return core.Issue{}, fmt.Errorf("resolve project: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()

	tx, err := db.Begin()
	if err != nil {
		return core.Issue{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Read current sequence value.
	var seq int64
	err = tx.QueryRow(`SELECT next_issue_seq FROM projects WHERE id = ?`, proj.ID).Scan(&seq)
	if err != nil {
		return core.Issue{}, fmt.Errorf("select next_issue_seq: %w", err)
	}

	shortID := fmt.Sprintf("%s-%d", proj.Key, seq)

	// Resolve repository and worktree references.
	var repoID, worktreeID interface{} = nil, nil
	if req.Repo != "" {
		repo, err := GetRepo(db, req.Repo)
		if err != nil {
			return core.Issue{}, fmt.Errorf("resolve repo: %w", err)
		}
		repoID = repo.ID
	}
	if req.Worktree != "" {
		wt, err := GetWorktree(db, req.Worktree)
		if err != nil {
			return core.Issue{}, fmt.Errorf("resolve worktree: %w", err)
		}
		worktreeID = wt.ID
	}

	status := "open"
	priority := req.Priority
	if priority <= 0 {
		priority = 3
	}

	_, err = tx.Exec(
		`INSERT INTO issues (id, short_id, project_id, repository_id, worktree_id, scope_kind,
		                    title, description, status, priority, assignee, version,
		                    claimed_at, closed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', 1, NULL, NULL, ?, ?)`,
		id, shortID, proj.ID, repoID, worktreeID, req.ScopeKind,
		req.Title, req.Description, status, priority, now, now,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("insert issue: %w", err)
	}

	// Increment the sequence.
	_, err = tx.Exec(`UPDATE projects SET next_issue_seq = ?, updated_at = ? WHERE id = ?`, seq+1, now, proj.ID)
	if err != nil {
		return core.Issue{}, fmt.Errorf("update next_issue_seq: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.Issue{}, fmt.Errorf("commit tx: %w", err)
	}

	return scanIssueRow(id, shortID, proj.ID, repoID, worktreeID, req.ScopeKind,
		req.Title, req.Description, status, priority, "", 1, "", "", now, now), nil
}

// GetIssue retrieves an issue by ID or short_id, along with its optional active lease.
func GetIssue(db *sql.DB, id string) (core.Issue, *core.IssueLease, error) {
	// Try by primary key first.
	row := db.QueryRow(
		`SELECT id, short_id, project_id, repository_id, worktree_id, scope_kind,
		        title, description, status, priority, assignee, version,
		        claimed_at, closed_at, created_at, updated_at
		 FROM issues WHERE id = ?`, id,
	)
	issue, err := scanIssue(row)
	if err != nil {
		// If not found by id, try by short_id.
		var apiErr core.APIError
		if errors.As(err, &apiErr) && apiErr.Code == core.ErrNotFound {
			row2 := db.QueryRow(
				`SELECT id, short_id, project_id, repository_id, worktree_id, scope_kind,
				        title, description, status, priority, assignee, version,
				        claimed_at, closed_at, created_at, updated_at
				 FROM issues WHERE short_id = ?`, id,
			)
			issue, err = scanIssue(row2)
			if err != nil {
				return core.Issue{}, nil, err
			}
		} else {
			return core.Issue{}, nil, err
		}
	}

	// Look up the lease.
	lease, err := getActiveLease(db, issue.ID)
	if err != nil {
		return core.Issue{}, nil, err
	}

	return issue, lease, nil
}

// ListIssues returns issues matching the given filters.
func ListIssues(db *sql.DB, params core.IssueListParams) ([]core.Issue, error) {
	var where []string
	var args []interface{}

	if params.Project != "" {
		proj, err := GetProjectByKey(db, params.Project)
		if err != nil {
			return nil, fmt.Errorf("resolve project: %w", err)
		}
		where = append(where, "project_id = ?")
		args = append(args, proj.ID)
	}
	if params.Repo != "" {
		repo, err := GetRepo(db, params.Repo)
		if err != nil {
			return nil, fmt.Errorf("resolve repo: %w", err)
		}
		where = append(where, "repository_id = ?")
		args = append(args, repo.ID)
	}
	if params.Worktree != "" {
		wt, err := GetWorktree(db, params.Worktree)
		if err != nil {
			return nil, fmt.Errorf("resolve worktree: %w", err)
		}
		where = append(where, "worktree_id = ?")
		args = append(args, wt.ID)
	}
	if params.Status != "" {
		where = append(where, "status = ?")
		args = append(args, params.Status)
	}
	if params.Assignee != "" {
		where = append(where, "assignee = ?")
		args = append(args, params.Assignee)
	}

	query := `SELECT id, short_id, project_id, repository_id, worktree_id, scope_kind,
	                 title, description, status, priority, assignee, version,
	                 claimed_at, closed_at, created_at, updated_at
	          FROM issues`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY updated_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var issues []core.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issues: %w", err)
	}
	if issues == nil {
		issues = []core.Issue{}
	}
	return issues, nil
}

// ListReadyIssues returns issues that are actionable (not terminal) and not currently leased.
// Dependency filtering (from the dependencies table) is deferred to SDD-0008.
func ListReadyIssues(db *sql.DB, projectID string) ([]core.Issue, error) {
	var args []interface{}
	query := `SELECT id, short_id, project_id, repository_id, worktree_id, scope_kind,
	                 title, description, status, priority, assignee, version,
	                 claimed_at, closed_at, created_at, updated_at
	          FROM issues
	          WHERE status NOT IN ('done', 'cancelled', 'deferred', 'blocked')
	            AND id NOT IN (SELECT issue_id FROM leases WHERE expires_at > datetime('now'))`

	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY priority, updated_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ready issues: %w", err)
	}
	defer rows.Close()

	var issues []core.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ready issues: %w", err)
	}
	if issues == nil {
		issues = []core.Issue{}
	}
	return issues, nil
}

func getActiveLease(db *sql.DB, issueID string) (*core.IssueLease, error) {
	row := db.QueryRow(
		`SELECT holder, lease_token, expires_at FROM leases WHERE issue_id = ? AND expires_at > datetime('now')`,
		issueID,
	)
	var lease core.IssueLease
	err := row.Scan(&lease.Holder, &lease.LeaseToken, &lease.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan lease: %w", err)
	}
	return &lease, nil
}

func scanIssue(s scanner) (core.Issue, error) {
	var i core.Issue
	var repoID, worktreeID, claimedAt, closedAt sql.NullString
	err := s.Scan(&i.ID, &i.ShortID, &i.ProjectID, &repoID, &worktreeID,
		&i.ScopeKind, &i.Title, &i.Description, &i.Status, &i.Priority,
		&i.Assignee, &i.Version, &claimedAt, &closedAt,
		&i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.Issue{}, core.NewAPIError(core.ErrNotFound, "issue not found")
		}
		return core.Issue{}, fmt.Errorf("scan issue: %w", err)
	}
	if repoID.Valid {
		i.RepositoryID = repoID.String
	}
	if worktreeID.Valid {
		i.WorktreeID = worktreeID.String
	}
	if claimedAt.Valid {
		i.ClaimedAt = claimedAt.String
	}
	if closedAt.Valid {
		i.ClosedAt = closedAt.String
	}
	return i, nil
}

// scanIssueRow builds an Issue struct from raw fields (used by CreateIssue to avoid a second query).
func scanIssueRow(id, shortID, projectID string, repoID, worktreeID interface{},
	scopeKind, title, description, status string, priority int,
	assignee string, version int, claimedAt, closedAt, createdAt, updatedAt string) core.Issue {
	i := core.Issue{
		ID:          id,
		ShortID:     shortID,
		ProjectID:   projectID,
		ScopeKind:   scopeKind,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		Assignee:    assignee,
		Version:     version,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	if rid, ok := repoID.(string); ok {
		i.RepositoryID = rid
	}
	if wid, ok := worktreeID.(string); ok {
		i.WorktreeID = wid
	}
	if claimedAt != "" {
		i.ClaimedAt = claimedAt
	}
	if closedAt != "" {
		i.ClosedAt = closedAt
	}
	return i
}
