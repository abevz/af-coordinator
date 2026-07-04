package sqlite

import (
	"database/sql"
	"encoding/json"
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

	// Append event.
	eventPayload := fmt.Sprintf(`{"title":"%s","scope_kind":"%s"}`, req.Title, req.ScopeKind)
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), id, req.Actor, "issue_created", eventPayload, now,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.Issue{}, fmt.Errorf("commit tx: %w", err)
	}

	return scanIssueRow(id, shortID, proj.ID, repoID, worktreeID, req.ScopeKind,
		req.Title, req.Description, status, priority, "", 1, "", "", "", "", now, now), nil
}

// ResolveIssueID resolves an issue by either its UUID id or short_id, returning the UUID id.
func ResolveIssueID(db *sql.DB, idOrShortID string) (string, error) {
	// Try by primary key first.
	var id string
	err := db.QueryRow(`SELECT id FROM issues WHERE id = ?`, idOrShortID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("resolve issue by id: %w", err)
	}

	// Fall back to short_id.
	err = db.QueryRow(`SELECT id FROM issues WHERE short_id = ?`, idOrShortID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", core.NewAPIError(core.ErrNotFound, "issue not found: "+idOrShortID)
	}
	if err != nil {
		return "", fmt.Errorf("resolve issue by short_id: %w", err)
	}
	return id, nil
}

// GetIssue retrieves an issue by ID (UUID), along with its optional active lease.
// Callers should use ResolveIssueID first to resolve short_id to UUID.
func GetIssue(db *sql.DB, id string) (core.Issue, *core.IssueLease, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	row := db.QueryRow(
		`SELECT i.id, i.short_id, i.project_id, i.repository_id, i.worktree_id, i.scope_kind,
		        i.title, i.description, i.status, i.priority, i.assignee, i.version,
		        i.claimed_at, i.closed_at, i.created_at, i.updated_at,
		        COALESCE(l.holder, ''), COALESCE(l.expires_at, '')
		 FROM issues i
		 LEFT JOIN leases l ON l.issue_id = i.id AND l.expires_at > ?
		 WHERE i.id = ?`, now, id,
	)
	issue, err := scanIssue(row)
	if err != nil {
		return core.Issue{}, nil, err
	}

	// Look up the lease (separate query for token/expiry detail).
	lease, err := getActiveLease(db, issue.ID)
	if err != nil {
		return core.Issue{}, nil, err
	}

	return issue, lease, nil
}

// ListIssues returns issues matching the given filters.
func ListIssues(db *sql.DB, params core.IssueListParams) ([]core.Issue, error) {
	var where []string
	args := []interface{}{time.Now().UTC().Format(time.RFC3339)}

	if params.Project != "" {
		proj, err := GetProjectByKey(db, params.Project)
		if err != nil {
			return nil, fmt.Errorf("resolve project: %w", err)
		}
		where = append(where, "i.project_id = ?")
		args = append(args, proj.ID)
	}
	if params.Repo != "" {
		repo, err := GetRepo(db, params.Repo)
		if err != nil {
			return nil, fmt.Errorf("resolve repo: %w", err)
		}
		where = append(where, "i.repository_id = ?")
		args = append(args, repo.ID)
	}
	if params.Worktree != "" {
		wt, err := GetWorktree(db, params.Worktree)
		if err != nil {
			return nil, fmt.Errorf("resolve worktree: %w", err)
		}
		where = append(where, "i.worktree_id = ?")
		args = append(args, wt.ID)
	}
	if params.Status != "" {
		where = append(where, "i.status = ?")
		args = append(args, params.Status)
	}
	if params.Assignee != "" {
		where = append(where, "i.assignee = ?")
		args = append(args, params.Assignee)
	}

	query := `SELECT i.id, i.short_id, i.project_id, i.repository_id, i.worktree_id, i.scope_kind,
	                 i.title, i.description, i.status, i.priority, i.assignee, i.version,
	                 i.claimed_at, i.closed_at, i.created_at, i.updated_at,
	                 COALESCE(l.holder, ''), COALESCE(l.expires_at, '')
	          FROM issues i
	          LEFT JOIN leases l ON l.issue_id = i.id AND l.expires_at > ?`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY i.updated_at DESC"

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

// ListReadyIssues returns issues that are actionable (not terminal), not currently leased,
// and not blocked by an unfinished blocks dependency.
func ListReadyIssues(db *sql.DB, projectID string) ([]core.Issue, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	args := []interface{}{now}
	query := `SELECT i.id, i.short_id, i.project_id, i.repository_id, i.worktree_id, i.scope_kind,
	                 i.title, i.description, i.status, i.priority, i.assignee, i.version,
	                 i.claimed_at, i.closed_at, i.created_at, i.updated_at,
	                 COALESCE(l.holder, ''), COALESCE(l.expires_at, '')
	          FROM issues i
	          LEFT JOIN leases l ON l.issue_id = i.id AND l.expires_at > ?
	          WHERE i.status NOT IN ('done', 'cancelled', 'deferred', 'blocked')
	            AND i.id NOT IN (SELECT issue_id FROM leases WHERE expires_at > ?)
	            AND NOT EXISTS (
	                SELECT 1 FROM dependencies d
	                JOIN issues blocker ON blocker.id = d.depends_on_issue_id
	                WHERE d.issue_id = i.id
	                  AND d.kind = 'blocks'
	                  AND blocker.status NOT IN ('done', 'cancelled')
	            )`

	args = append(args, now)

	if projectID != "" {
		query += " AND i.project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY i.priority, i.updated_at DESC"

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

// ClaimIssue acquires a lease on an issue, moving it from 'open' to 'in_progress'.
func ClaimIssue(db *sql.DB, issueID, holder string, ttlSeconds int) (core.ClaimResponse, error) {
	tx, err := db.Begin()
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check issue exists and is open.
	var status string
	err = tx.QueryRow(`SELECT status FROM issues WHERE id = ?`, issueID).Scan(&status)
	if err == sql.ErrNoRows {
		return core.ClaimResponse{}, core.NewAPIError(core.ErrNotFound, "issue not found: "+issueID)
	}
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("select issue: %w", err)
	}
	if status != "open" {
		return core.ClaimResponse{}, core.NewAPIError(core.ErrLeaseHeld,
			"issue cannot be claimed from status: "+status)
	}

	// Check no unexpired lease exists.
	var leaseCount int
	nowRFC := time.Now().UTC().Format(time.RFC3339)
	err = tx.QueryRow(
		`SELECT count(*) FROM leases WHERE issue_id = ? AND expires_at > ?`,
		issueID, nowRFC,
	).Scan(&leaseCount)
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("check lease: %w", err)
	}
	if leaseCount > 0 {
		return core.ClaimResponse{}, core.NewAPIError(core.ErrLeaseHeld,
			"issue is already claimed: "+issueID)
	}

	// Generate lease.
	now := time.Now().UTC()
	leaseToken := uuid.New().String()
	expiresAt := now.Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339)

	_, err = tx.Exec(
		`INSERT INTO leases (issue_id, holder, lease_token, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		issueID, holder, leaseToken, expiresAt, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		// Constraint violation (PK on issue_id) means another claim won while
		// both deferred transactions were reading the same "open" state.
		if isSQLiteConstraintError(err) {
			return core.ClaimResponse{}, core.NewAPIError(core.ErrLeaseHeld,
				"issue is already claimed: "+issueID)
		}
		return core.ClaimResponse{}, fmt.Errorf("insert lease: %w", err)
	}

	// Update issue status and version.
	nowStr := now.Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE issues SET status = 'in_progress', claimed_at = ?, version = version + 1, updated_at = ? WHERE id = ?`,
		nowStr, nowStr, issueID,
	)
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("update issue: %w", err)
	}

	// Append event.
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, holder, "issue_claimed", `{}`, nowStr,
	)
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.ClaimResponse{}, fmt.Errorf("commit tx: %w", err)
	}

	return core.ClaimResponse{LeaseToken: leaseToken, ExpiresAt: expiresAt}, nil
}

// HeartbeatLease extends the TTL on an existing lease.
func HeartbeatLease(db *sql.DB, issueID, leaseToken string, ttlSeconds int) (string, error) {
	// Look up the lease.
	var holder, expiresAt string
	err := db.QueryRow(
		`SELECT holder, expires_at FROM leases WHERE issue_id = ? AND lease_token = ?`,
		issueID, leaseToken,
	).Scan(&holder, &expiresAt)
	if err == sql.ErrNoRows {
		return "", core.NewAPIError(core.ErrLeaseExpired, "lease not found or expired")
	}
	if err != nil {
		return "", fmt.Errorf("select lease: %w", err)
	}

	// Check the lease hasn't expired.
	expiresTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", fmt.Errorf("parse expires_at: %w", err)
	}
	if time.Now().UTC().After(expiresTime) {
		return "", core.NewAPIError(core.ErrLeaseExpired, "lease has expired")
	}

	// Extend the lease.
	now := time.Now().UTC()
	newExpiresAt := now.Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339)

	_, err = db.Exec(
		`UPDATE leases SET expires_at = ?, updated_at = ? WHERE issue_id = ? AND lease_token = ?`,
		newExpiresAt, now.Format(time.RFC3339), issueID, leaseToken,
	)
	if err != nil {
		return "", fmt.Errorf("update lease: %w", err)
	}

	return newExpiresAt, nil
}

// ReleaseLease releases a lease and returns the issue to 'open' (unless blocked).
func ReleaseLease(db *sql.DB, issueID, leaseToken string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Find the lease.
	var holder string
	err = tx.QueryRow(
		`SELECT holder FROM leases WHERE issue_id = ? AND lease_token = ?`,
		issueID, leaseToken,
	).Scan(&holder)
	if err == sql.ErrNoRows {
		return core.NewAPIError(core.ErrLeaseExpired, "lease not found")
	}
	if err != nil {
		return fmt.Errorf("select lease: %w", err)
	}

	// Delete the lease.
	_, err = tx.Exec(
		`DELETE FROM leases WHERE issue_id = ? AND lease_token = ?`,
		issueID, leaseToken,
	)
	if err != nil {
		return fmt.Errorf("delete lease: %w", err)
	}

	// Update issue status: in_progress -> open (unless blocked).
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE issues SET
		     status = CASE WHEN status = 'blocked' THEN 'blocked' ELSE 'open' END,
		     claimed_at = NULL,
		     version = version + 1,
		     updated_at = ?
		 WHERE id = ?`,
		now, issueID,
	)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	// Append event.
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, holder, "issue_released", `{}`, now,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func getActiveLease(db *sql.DB, issueID string) (*core.IssueLease, error) {
	row := db.QueryRow(
		`SELECT holder, lease_token, expires_at FROM leases WHERE issue_id = ? AND expires_at > ?`,
		issueID, time.Now().UTC().Format(time.RFC3339),
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
	var holder, leaseExpiresAt sql.NullString
	err := s.Scan(&i.ID, &i.ShortID, &i.ProjectID, &repoID, &worktreeID,
		&i.ScopeKind, &i.Title, &i.Description, &i.Status, &i.Priority,
		&i.Assignee, &i.Version, &claimedAt, &closedAt,
		&i.CreatedAt, &i.UpdatedAt, &holder, &leaseExpiresAt)
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
	if holder.Valid {
		i.Holder = holder.String
	}
	if leaseExpiresAt.Valid {
		i.LeaseExpiresAt = leaseExpiresAt.String
	}
	return i, nil
}

// UpdateIssue updates an issue's mutable fields with optimistic concurrency.
func UpdateIssue(db *sql.DB, issueID string, req core.UpdateIssueRequest) (core.Issue, error) {
	issue, lease, err := GetIssue(db, issueID)
	if err != nil {
		return core.Issue{}, err
	}

	// Version check.
	if req.ExpectedVersion != issue.Version {
		return core.Issue{}, core.NewAPIError(core.ErrConflict,
			fmt.Sprintf("expected version %d, current version is %d", req.ExpectedVersion, issue.Version))
	}

	// Lease check: if issue has an active lease, require lease_token to match.
	if lease != nil && lease.LeaseToken != req.LeaseToken {
		return core.Issue{}, core.NewAPIError(core.ErrLeaseExpired,
			"issue is leased and lease_token does not match")
	}

	// Validate status transition if updating status.
	if req.Status != "" && req.Status != issue.Status {
		if err := core.ValidateStatusTransition(issue.Status, req.Status); err != nil {
			return core.Issue{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return core.Issue{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Build dynamic SET clause for non-zero / non-empty fields.
	var sets []string
	var args []interface{}

	if req.Title != "" {
		sets = append(sets, "title = ?")
		args = append(args, req.Title)
	}
	if req.Description != "" {
		sets = append(sets, "description = ?")
		args = append(args, req.Description)
	}
	if req.Priority > 0 {
		sets = append(sets, "priority = ?")
		args = append(args, req.Priority)
	}
	if req.Assignee != "" {
		sets = append(sets, "assignee = ?")
		args = append(args, req.Assignee)
	}
	if req.Status != "" {
		sets = append(sets, "status = ?")
		args = append(args, req.Status)
	}

	if len(sets) == 0 {
		return core.Issue{}, core.NewAPIError(core.ErrValidationFailed, "no fields to update")
	}

	sets = append(sets, "version = version + 1")
	sets = append(sets, "updated_at = ?")
	args = append(args, now)
	args = append(args, issueID)

	query := "UPDATE issues SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	result, err := tx.Exec(query, args...)
	if err != nil {
		return core.Issue{}, fmt.Errorf("update issue: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return core.Issue{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return core.Issue{}, core.NewAPIError(core.ErrNotFound, "issue not found: "+issueID)
	}

	// Append event.
	changed := buildChangedFields(req)
	eventPayload := map[string]interface{}{"changed": changed}
	payloadBytes, _ := json.Marshal(eventPayload)
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, req.Actor, "issue_updated", string(payloadBytes), now,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.Issue{}, fmt.Errorf("commit tx: %w", err)
	}

	// Re-read to get updated version, timestamps, etc.
	updated, _, err := GetIssue(db, issueID)
	if err != nil {
		return core.Issue{}, fmt.Errorf("re-read issue: %w", err)
	}
	return updated, nil
}

// buildChangedFields returns the list of field names that were changed in the update request.
func buildChangedFields(req core.UpdateIssueRequest) []string {
	var changed []string
	if req.Title != "" {
		changed = append(changed, "title")
	}
	if req.Description != "" {
		changed = append(changed, "description")
	}
	if req.Priority > 0 {
		changed = append(changed, "priority")
	}
	if req.Assignee != "" {
		changed = append(changed, "assignee")
	}
	if req.Status != "" {
		changed = append(changed, "status")
	}
	return changed
}

// CloseIssue closes an issue by setting its status to 'done' or 'cancelled'.
func CloseIssue(db *sql.DB, issueID string, req core.CloseIssueRequest) error {
	issue, lease, err := GetIssue(db, issueID)
	if err != nil {
		return err
	}

	// Version check.
	if req.ExpectedVersion != issue.Version {
		return core.NewAPIError(core.ErrConflict,
			fmt.Sprintf("expected version %d, current version is %d", req.ExpectedVersion, issue.Version))
	}

	// Lease check.
	if lease != nil && lease.LeaseToken != req.LeaseToken {
		return core.NewAPIError(core.ErrLeaseExpired,
			"issue is leased and lease_token does not match")
	}

	// Resolution validation.
	if req.Resolution != "done" && req.Resolution != "cancelled" {
		return core.NewAPIError(core.ErrValidationFailed,
			"resolution must be 'done' or 'cancelled'")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`UPDATE issues SET status = ?, closed_at = ?, version = version + 1, updated_at = ? WHERE id = ?`,
		req.Resolution, now, now, issueID,
	)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	// Remove any active lease on this issue.
	_, _ = tx.Exec(`DELETE FROM leases WHERE issue_id = ?`, issueID)

	// Append event.
	payload := fmt.Sprintf(`{"changed":["status","closed_at"],"resolution":"%s"}`, req.Resolution)
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, req.Actor, "issue_closed", payload, now,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return tx.Commit()
}

// AddDependency adds a dependency between two issues. For 'blocks' kind, it performs cycle detection.
func AddDependency(db *sql.DB, issueID string, req core.AddDependencyRequest) error {
	// Verify both issues exist.
	if _, _, err := GetIssue(db, issueID); err != nil {
		return err
	}
	if _, _, err := GetIssue(db, req.DependsOn); err != nil {
		return err
	}

	kind := req.Kind
	if kind == "" {
		kind = "blocks"
	}

	if kind == "blocks" {
		if wouldCreateCycle(db, issueID, req.DependsOn) {
			return core.NewAPIError(core.ErrDependencyCycle,
				"adding this dependency would create a cycle")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT INTO dependencies (issue_id, depends_on_issue_id, kind, created_at)
		 VALUES (?, ?, ?, ?)`,
		issueID, req.DependsOn, kind, now,
	)
	if err != nil {
		// Handle duplicate / unique constraint.
		if isSQLiteConstraintError(err) {
			return core.NewAPIError(core.ErrConflict,
				"dependency already exists")
		}
		return fmt.Errorf("insert dependency: %w", err)
	}

	// Append event.
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, req.Actor, "dependency_added",
		fmt.Sprintf(`{"depends_on":"%s","kind":"%s"}`, req.DependsOn, kind), now,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// RemoveDependency removes a dependency record.
func RemoveDependency(db *sql.DB, issueID, dependsOn, kind, actor string) error {
	if kind == "" {
		kind = "blocks"
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec(
		`DELETE FROM dependencies WHERE issue_id = ? AND depends_on_issue_id = ? AND kind = ?`,
		issueID, dependsOn, kind,
	)
	if err != nil {
		return fmt.Errorf("delete dependency: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return core.NewAPIError(core.ErrNotFound, "dependency not found")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Append event.
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, actor, "dependency_removed",
		fmt.Sprintf(`{"depends_on":"%s","kind":"%s"}`, dependsOn, kind), now,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// wouldCreateCycle checks if adding a 'blocks' dependency from fromIssueID to toIssueID would create a cycle.
// It does a BFS from toIssueID following 'blocks' edges to see if we reach fromIssueID.
func wouldCreateCycle(db *sql.DB, fromIssueID, toIssueID string) bool {
	visited := map[string]bool{}
	queue := []string{toIssueID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == fromIssueID {
			return true // cycle!
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		rows, err := db.Query(
			`SELECT depends_on_issue_id FROM dependencies WHERE issue_id = ? AND kind = 'blocks'`, current)
		if err != nil {
			continue
		}
		for rows.Next() {
			var dep string
			if err := rows.Scan(&dep); err != nil {
				continue
			}
			if !visited[dep] {
				queue = append(queue, dep)
			}
		}
		rows.Close()
	}
	return false
}

// LinkArtifact links an artifact to an issue by inserting into issue_artifacts.
func LinkArtifact(db *sql.DB, issueID string, req core.LinkArtifactRequest) (string, error) {
	// Verify the issue exists.
	if _, _, err := GetIssue(db, issueID); err != nil {
		return "", err
	}

	// Resolve artifact by ID.
	artifact, err := GetArtifact(db, req.Artifact)
	if err != nil {
		return "", err
	}

	relation := req.Relation
	if relation == "" {
		relation = "implements"
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		`INSERT INTO issue_artifacts (issue_id, artifact_id, relation, created_at)
		 VALUES (?, ?, ?, ?)`,
		issueID, artifact.ID, relation, now,
	)
	if err != nil {
		if isSQLiteConstraintError(err) {
			return "", core.NewAPIError(core.ErrAlreadyLinked,
				"issue is already linked to this artifact with relation '"+relation+"'")
		}
		return "", fmt.Errorf("insert issue_artifact: %w", err)
	}

	return now, nil
}

// ListIssueArtifacts returns all artifacts linked to an issue.
func ListIssueArtifacts(db *sql.DB, issueID string) ([]core.ArtifactRef, error) {
	rows, err := db.Query(
		`SELECT a.id, a.relative_path, a.kind, ia.relation
		 FROM issue_artifacts ia
		 JOIN artifacts a ON ia.artifact_id = a.id
		 WHERE ia.issue_id = ?
		 ORDER BY ia.created_at`,
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list issue artifacts: %w", err)
	}
	defer rows.Close()

	var refs []core.ArtifactRef
	for rows.Next() {
		var ref core.ArtifactRef
		if err := rows.Scan(&ref.ID, &ref.RelativePath, &ref.Kind, &ref.Relation); err != nil {
			return nil, fmt.Errorf("scan artifact ref: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact refs: %w", err)
	}
	if refs == nil {
		refs = []core.ArtifactRef{}
	}
	return refs, nil
}

// CreateNote inserts a new note on an issue.
func CreateNote(db *sql.DB, issueID string, req core.CreateNoteRequest) (core.Note, error) {
	// Verify issue exists.
	if _, _, err := GetIssue(db, issueID); err != nil {
		return core.Note{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()

	tx, err := db.Begin()
	if err != nil {
		return core.Note{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT INTO notes (id, issue_id, author, body, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, issueID, req.Author, req.Body, now,
	)
	if err != nil {
		return core.Note{}, fmt.Errorf("insert note: %w", err)
	}

	// Append event.
	_, err = tx.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, req.Author, "note_added", `{}`, now,
	)
	if err != nil {
		return core.Note{}, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.Note{}, fmt.Errorf("commit tx: %w", err)
	}

	return core.Note{
		ID:        id,
		IssueID:   issueID,
		Author:    req.Author,
		Body:      req.Body,
		CreatedAt: now,
	}, nil
}

// ListNotes returns all notes for an issue, ordered by creation time.
func ListNotes(db *sql.DB, issueID string) ([]core.Note, error) {
	// Verify issue exists.
	if _, _, err := GetIssue(db, issueID); err != nil {
		return nil, err
	}

	rows, err := db.Query(
		`SELECT id, issue_id, author, body, created_at FROM notes WHERE issue_id = ? ORDER BY created_at ASC`,
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []core.Note
	for rows.Next() {
		var n core.Note
		if err := rows.Scan(&n.ID, &n.IssueID, &n.Author, &n.Body, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}
	if notes == nil {
		notes = []core.Note{}
	}
	return notes, nil
}

// ListEvents returns all events for an issue, ordered by creation time.
func ListEvents(db *sql.DB, issueID string) ([]core.Event, error) {
	// Verify issue exists.
	if _, _, err := GetIssue(db, issueID); err != nil {
		return nil, err
	}

	rows, err := db.Query(
		`SELECT id, issue_id, actor, event_type, payload_json, created_at FROM events WHERE issue_id = ? ORDER BY created_at ASC`,
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []core.Event
	for rows.Next() {
		var e core.Event
		var issueID sql.NullString
		if err := rows.Scan(&e.ID, &issueID, &e.Actor, &e.EventType, &e.PayloadJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if issueID.Valid {
			e.IssueID = issueID.String
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	if events == nil {
		events = []core.Event{}
	}
	return events, nil
}

// isSQLiteConstraintError checks if an error is a SQLite constraint violation.
func isSQLiteConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// scanIssueRow builds an Issue struct from raw fields (used by CreateIssue to avoid a second query).
func scanIssueRow(id, shortID, projectID string, repoID, worktreeID interface{},
	scopeKind, title, description, status string, priority int,
	assignee string, version int, claimedAt, closedAt, holder, leaseExpiresAt, createdAt, updatedAt string) core.Issue {
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
	if holder != "" {
		i.Holder = holder
	}
	if leaseExpiresAt != "" {
		i.LeaseExpiresAt = leaseExpiresAt
	}
	return i
}
