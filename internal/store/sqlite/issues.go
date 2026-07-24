package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/google/uuid"
)

// CreateIssue inserts a new issue, allocating a short_id from the project's sequence.
func CreateIssue(ctx context.Context, db *sql.DB, projectKey string, req core.CreateIssueRequest) (core.Issue, error) {
	// Resolve project by key.
	proj, err := GetProjectByKey(ctx, db, projectKey)
	if err != nil {
		return core.Issue{}, fmt.Errorf("resolve project: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()

	// Resolve repository and worktree references.
	var repoID, worktreeID interface{} = nil, nil
	if req.Repo != "" {
		repo, err := GetRepoInProject(ctx, db, proj.ID, req.Repo)
		if err != nil {
			return core.Issue{}, fmt.Errorf("resolve repo: %w", err)
		}
		repoID = repo.ID
	}
	if req.Worktree != "" {
		wt, err := GetWorktree(ctx, db, req.Worktree)
		if err != nil {
			return core.Issue{}, fmt.Errorf("resolve worktree: %w", err)
		}
		worktreeID = wt.ID
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.Issue{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Read current sequence value.
	var seq int64
	err = tx.QueryRowContext(ctx, `SELECT next_issue_seq FROM projects WHERE id = ?`, proj.ID).Scan(&seq)
	if err != nil {
		return core.Issue{}, fmt.Errorf("select next_issue_seq: %w", err)
	}

	shortID := fmt.Sprintf("%s-%d", proj.Key, seq)

	status := "open"
	priority := req.Priority
	if priority <= 0 {
		priority = 3
	}
	issueType := req.IssueType
	if issueType == "" {
		issueType = "task"
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO issues (id, short_id, project_id, repository_id, worktree_id, scope_kind,
		                    issue_type, title, external_key, description, acceptance_criteria, status, priority, assignee, version,
		                    claimed_at, closed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', 1, NULL, NULL, ?, ?)`,
		id, shortID, proj.ID, repoID, worktreeID, req.ScopeKind,
		issueType, req.Title, req.ExternalKey, req.Description, req.AcceptanceCriteria, status, priority, now, now,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("insert issue: %w", err)
	}

	// Increment the sequence.
	_, err = tx.ExecContext(ctx, `UPDATE projects SET next_issue_seq = ?, updated_at = ? WHERE id = ?`, seq+1, now, proj.ID)
	if err != nil {
		return core.Issue{}, fmt.Errorf("update next_issue_seq: %w", err)
	}

	// Append event.
	eventPayload := map[string]string{
		"title":      req.Title,
		"scope_kind": req.ScopeKind,
	}
	if req.ExternalKey != "" {
		eventPayload["external_key"] = req.ExternalKey
	}
	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return core.Issue{}, fmt.Errorf("marshal event payload: %w", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), id, req.Actor, "issue_created", string(payloadBytes), now,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.Issue{}, fmt.Errorf("commit tx: %w", err)
	}

	return scanIssueRow(id, shortID, proj.ID, repoID, worktreeID, req.ScopeKind,
		issueType, req.Title, req.ExternalKey, req.Description, req.AcceptanceCriteria, status, priority, "", 1, "", "", "", "", now, now), nil
}

// ResolveIssueID resolves an issue by either its UUID id or short_id, returning the UUID id.
func ResolveIssueID(ctx context.Context, db *sql.DB, idOrShortID string) (string, error) {
	// Try by primary key first.
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM issues WHERE id = ?`, idOrShortID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("resolve issue by id: %w", err)
	}

	// Fall back to short_id.
	err = db.QueryRowContext(ctx, `SELECT id FROM issues WHERE short_id = ?`, idOrShortID).Scan(&id)
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
func GetIssue(ctx context.Context, db *sql.DB, id string) (core.Issue, *core.IssueLease, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	row := db.QueryRowContext(ctx,
		`SELECT i.id, i.short_id, i.project_id, i.repository_id, i.worktree_id, i.scope_kind,
		        i.issue_type, i.title, i.external_key, i.description, i.acceptance_criteria, i.status, i.priority, i.assignee, i.version,
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
	lease, err := getActiveLease(ctx, db, issue.ID)
	if err != nil {
		return core.Issue{}, nil, err
	}

	populated, err := populateDependencies(ctx, db, []core.Issue{issue})
	if err != nil {
		return core.Issue{}, nil, err
	}
	issue = populated[0]

	return issue, lease, nil
}

// ListIssues returns issues matching the given filters.
func ListIssues(ctx context.Context, db *sql.DB, params core.IssueListParams) ([]core.Issue, error) {
	var where []string
	args := []interface{}{time.Now().UTC().Format(time.RFC3339)}
	projectKeys := issueListFilterValues(params.Projects, params.Project)
	projectIDs := make([]string, 0, len(projectKeys))

	for _, projectKey := range projectKeys {
		proj, err := GetProjectByKey(ctx, db, projectKey)
		if err != nil {
			return nil, fmt.Errorf("resolve project: %w", err)
		}
		projectIDs = append(projectIDs, proj.ID)
	}
	if len(projectIDs) > 0 {
		appendIssueListInFilter(&where, &args, "i.project_id", projectIDs)
	}
	if params.Repo != "" {
		if len(projectIDs) > 1 {
			return nil, core.NewAPIError(core.ErrValidationFailed,
				"repo filter requires exactly one project")
		}
		projectID := ""
		if len(projectIDs) == 1 {
			projectID = projectIDs[0]
		}
		repo, err := GetRepoInProject(ctx, db, projectID, params.Repo)
		if err != nil {
			return nil, fmt.Errorf("resolve repo: %w", err)
		}
		where = append(where, "i.repository_id = ?")
		args = append(args, repo.ID)
	}
	if params.Worktree != "" {
		wt, err := GetWorktree(ctx, db, params.Worktree)
		if err != nil {
			return nil, fmt.Errorf("resolve worktree: %w", err)
		}
		where = append(where, "i.worktree_id = ?")
		args = append(args, wt.ID)
	}
	if statuses := issueListFilterValues(params.Statuses, params.Status); len(statuses) > 0 {
		appendIssueListInFilter(&where, &args, "i.status", statuses)
	}
	if params.Assignee != "" {
		where = append(where, "i.assignee = ?")
		args = append(args, params.Assignee)
	}
	if issueTypes := issueListFilterValues(params.IssueTypes, params.IssueType); len(issueTypes) > 0 {
		appendIssueListInFilter(&where, &args, "i.issue_type", issueTypes)
	}
	if params.ExternalKey != "" {
		where = append(where, "i.external_key = ?")
		args = append(args, params.ExternalKey)
	}

	query := `SELECT i.id, i.short_id, i.project_id, i.repository_id, i.worktree_id, i.scope_kind,
	                 i.issue_type, i.title, i.external_key, i.description, i.acceptance_criteria, i.status, i.priority, i.assignee, i.version,
	                 i.claimed_at, i.closed_at, i.created_at, i.updated_at,
	                 COALESCE(l.holder, ''), COALESCE(l.expires_at, '')
	          FROM issues i
	          LEFT JOIN leases l ON l.issue_id = i.id AND l.expires_at > ?`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY i.updated_at DESC, i.id ASC"

	rows, err := db.QueryContext(ctx, query, args...)
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
	} else {
		issues, err = populateDependencies(ctx, db, issues)
		if err != nil {
			return nil, err
		}
	}
	return issues, nil
}

func issueListFilterValues(values []string, fallback string) []string {
	if len(values) > 0 {
		return values
	}
	if fallback == "" {
		return nil
	}
	return []string{fallback}
}

func appendIssueListInFilter(where *[]string, args *[]interface{}, column string, values []string) {
	placeholders := make([]string, len(values))
	for i, value := range values {
		placeholders[i] = "?"
		*args = append(*args, value)
	}
	*where = append(*where, column+" IN ("+strings.Join(placeholders, ", ")+")")
}

// ListReadyIssues returns issues that are actionable (not terminal), not currently leased,
// and not blocked by an unfinished blocks dependency.
func ListReadyIssues(ctx context.Context, db *sql.DB, projectID, repoID string) ([]core.Issue, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	args := []interface{}{now}
	query := `SELECT i.id, i.short_id, i.project_id, i.repository_id, i.worktree_id, i.scope_kind,
	                 i.issue_type, i.title, i.external_key, i.description, i.acceptance_criteria, i.status, i.priority, i.assignee, i.version,
	                 i.claimed_at, i.closed_at, i.created_at, i.updated_at,
	                 COALESCE(l.holder, ''), COALESCE(l.expires_at, '')
	          FROM issues i
	          LEFT JOIN leases l ON l.issue_id = i.id AND l.expires_at > ?
	          WHERE i.status NOT IN ('done', 'cancelled', 'deferred', 'blocked')
	            AND i.issue_type != 'epic'
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
	if repoID != "" {
		query += " AND i.repository_id = ?"
		args = append(args, repoID)
	}
	query += " ORDER BY i.priority, i.updated_at DESC"

	rows, err := db.QueryContext(ctx, query, args...)
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
	} else {
		issues, err = populateDependencies(ctx, db, issues)
		if err != nil {
			return nil, err
		}
	}
	return issues, nil
}

// ClaimIssue acquires a lease without session correlation for compatibility.
func ClaimIssue(ctx context.Context, db *sql.DB, issueID, holder string, ttlSeconds int) (core.ClaimResponse, error) {
	return ClaimIssueWithSession(ctx, db, issueID, holder, ttlSeconds, "")
}

// ClaimIssueWithSession acquires a lease and creates the durable attempt
// identity used by lifecycle events. Session IDs are optional caller supplied
// correlation data; they do not affect holder or lease authorization.
func ClaimIssueWithSession(ctx context.Context, db *sql.DB, issueID, holder string, ttlSeconds int, sessionID string) (core.ClaimResponse, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check issue exists and is open.
	var status, issueType string
	var version int
	err = tx.QueryRowContext(ctx, `SELECT status, issue_type, version FROM issues WHERE id = ?`, issueID).Scan(&status, &issueType, &version)
	if err == sql.ErrNoRows {
		return core.ClaimResponse{}, core.NewAPIError(core.ErrNotFound, "issue not found: "+issueID)
	}
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("select issue: %w", err)
	}
	if issueType == "epic" {
		return core.ClaimResponse{}, core.NewAPIError(core.ErrValidationFailed,
			"epics cannot be claimed; claim their child issues instead")
	}
	if status != "open" && status != "in_progress" {
		return core.ClaimResponse{}, core.NewAPIError(core.ErrLeaseHeld,
			"issue cannot be claimed from status: "+status)
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Check whether an unexpired lease exists and, if so, whether the
	// requesting holder already owns it (same-holder reattach).
	var existingHolder, existingToken, existingAttemptID string
	err = tx.QueryRowContext(ctx,
		`SELECT holder, lease_token, attempt_id FROM leases WHERE issue_id = ? AND expires_at > ?`,
		issueID, nowStr,
	).Scan(&existingHolder, &existingToken, &existingAttemptID)
	if err != nil && err != sql.ErrNoRows {
		return core.ClaimResponse{}, fmt.Errorf("check lease: %w", err)
	}
	if err == nil {
		// An active lease exists.
		if existingHolder != holder {
			// Different holder — reject as before.
			return core.ClaimResponse{}, core.NewAPIError(core.ErrLeaseHeld,
				"issue is already claimed: "+issueID)
		}
		// Same holder — reattach: renew the expiry in-place.
		newExpiry := now.Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339)
		_, err = tx.ExecContext(ctx,
			`UPDATE leases SET expires_at = ?, updated_at = ? WHERE issue_id = ?`,
			newExpiry, nowStr, issueID,
		)
		if err != nil {
			return core.ClaimResponse{}, fmt.Errorf("renew lease: %w", err)
		}
		reattachPayload := map[string]any{
			"attempt_id":  existingAttemptID,
			"ttl_seconds": ttlSeconds,
			"expires_at":  newExpiry,
			"session_id":  sessionID,
		}
		if err := insertEvent(ctx, tx, issueID, holder, "lease_reattached", reattachPayload, nowStr); err != nil {
			return core.ClaimResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return core.ClaimResponse{}, fmt.Errorf("commit tx: %w", err)
		}
		return core.ClaimResponse{
			LeaseToken: existingToken,
			ExpiresAt:  newExpiry,
			AttemptID:  existingAttemptID,
			Version:    version,
			Reattached: true,
		}, nil
	}

	// A row that did not pass the active-lease query is an expired attempt.
	// Record its terminal outcome before replacing it, so the event stream
	// remains the complete attempt history.
	var expiredAttemptID, expiredSessionID, expiredAt string
	err = tx.QueryRowContext(ctx,
		`SELECT attempt_id, session_id, expires_at FROM leases WHERE issue_id = ? AND expires_at <= ?`,
		issueID, nowStr,
	).Scan(&expiredAttemptID, &expiredSessionID, &expiredAt)
	if err != nil && err != sql.ErrNoRows {
		return core.ClaimResponse{}, fmt.Errorf("select expired lease: %w", err)
	}
	if err == nil {
		payload := map[string]any{
			"attempt_id": expiredAttemptID,
			"end_reason": "expired",
			"expired_at": expiredAt,
			"session_id": expiredSessionID,
		}
		if err := insertEvent(ctx, tx, issueID, "system", "lease_expired", payload, nowStr); err != nil {
			return core.ClaimResponse{}, err
		}
	}

	// Delete the expired attempt before inserting its replacement.
	_, err = tx.ExecContext(ctx, `DELETE FROM leases WHERE issue_id = ?`, issueID)
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("delete expired lease: %w", err)
	}

	// Generate lease and a separate non-secret attempt ID. The lease token is
	// intentionally never written to events or public issue responses.
	leaseToken := uuid.New().String()
	attemptID := uuid.New().String()
	expiresAt := now.Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO leases (issue_id, holder, lease_token, expires_at, attempt_id, session_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		issueID, holder, leaseToken, expiresAt, attemptID, sessionID, nowStr, nowStr,
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
	_, err = tx.ExecContext(ctx,
		`UPDATE issues SET status = 'in_progress', claimed_at = ?, version = version + 1, updated_at = ? WHERE id = ?`,
		nowStr, nowStr, issueID,
	)
	if err != nil {
		return core.ClaimResponse{}, fmt.Errorf("update issue: %w", err)
	}

	payload := map[string]any{
		"attempt_id":  attemptID,
		"ttl_seconds": ttlSeconds,
		"expires_at":  expiresAt,
		"session_id":  sessionID,
	}
	if err := insertEvent(ctx, tx, issueID, holder, "issue_claimed", payload, nowStr); err != nil {
		return core.ClaimResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return core.ClaimResponse{}, fmt.Errorf("commit tx: %w", err)
	}

	return core.ClaimResponse{LeaseToken: leaseToken, ExpiresAt: expiresAt, AttemptID: attemptID, Version: version + 1}, nil
}

// HeartbeatLease extends the TTL on an existing lease.
func HeartbeatLease(ctx context.Context, db *sql.DB, issueID, leaseToken string, ttlSeconds int) (string, error) {
	// Look up the lease.
	var holder, expiresAt string
	err := db.QueryRowContext(ctx,
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

	_, err = db.ExecContext(ctx,
		`UPDATE leases SET expires_at = ?, updated_at = ? WHERE issue_id = ? AND lease_token = ?`,
		newExpiresAt, now.Format(time.RFC3339), issueID, leaseToken,
	)
	if err != nil {
		return "", fmt.Errorf("update lease: %w", err)
	}

	return newExpiresAt, nil
}

// ReleaseLease releases a lease and returns the issue to 'open' (unless blocked).
func ReleaseLease(ctx context.Context, db *sql.DB, issueID, leaseToken string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Find the lease.
	var holder, attemptID string
	err = tx.QueryRowContext(ctx,
		`SELECT holder, attempt_id FROM leases WHERE issue_id = ? AND lease_token = ?`,
		issueID, leaseToken,
	).Scan(&holder, &attemptID)
	if err == sql.ErrNoRows {
		return core.NewAPIError(core.ErrLeaseExpired, "lease not found")
	}
	if err != nil {
		return fmt.Errorf("select lease: %w", err)
	}

	// Delete the lease.
	_, err = tx.ExecContext(ctx,
		`DELETE FROM leases WHERE issue_id = ? AND lease_token = ?`,
		issueID, leaseToken,
	)
	if err != nil {
		return fmt.Errorf("delete lease: %w", err)
	}

	// Update issue status: in_progress -> open (unless blocked).
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx,
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

	if err := insertEvent(ctx, tx, issueID, holder, "issue_released", map[string]any{
		"attempt_id": attemptID,
		"end_reason": "released",
	}, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// HandoffLease records a required HANDOFF note and releases its active lease
// in one transaction. The note is authored by the current lease holder, so a
// caller cannot separate the handoff evidence from its authorized owner.
func HandoffLease(ctx context.Context, db *sql.DB, issueID string, req core.HandoffRequest) (core.HandoffResponse, error) {
	if err := core.ValidateHandoffRequest(req); err != nil {
		return core.HandoffResponse{}, core.NewAPIError(core.ErrValidationFailed, err.Error())
	}
	if req.LeaseToken == "" {
		return core.HandoffResponse{}, core.NewAPIError(core.ErrValidationFailed, "lease_token is required")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.HandoffResponse{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	var holder, attemptID string
	err = tx.QueryRowContext(ctx,
		`SELECT holder, attempt_id FROM leases
		 WHERE issue_id = ? AND lease_token = ? AND expires_at > ?`,
		issueID, req.LeaseToken, now,
	).Scan(&holder, &attemptID)
	if err == sql.ErrNoRows {
		return core.HandoffResponse{}, core.NewAPIError(core.ErrLeaseExpired, "active lease not found")
	}
	if err != nil {
		return core.HandoffResponse{}, fmt.Errorf("select active lease: %w", err)
	}

	note := core.Note{
		ID:        uuid.New().String(),
		IssueID:   issueID,
		Author:    holder,
		Body:      req.Note,
		CreatedAt: now,
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes (id, issue_id, author, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		note.ID, note.IssueID, note.Author, note.Body, note.CreatedAt,
	); err != nil {
		return core.HandoffResponse{}, fmt.Errorf("insert handoff note: %w", err)
	}
	if err := insertEvent(ctx, tx, issueID, holder, "note_added", map[string]any{}, now); err != nil {
		return core.HandoffResponse{}, err
	}

	result, err := tx.ExecContext(ctx,
		`DELETE FROM leases WHERE issue_id = ? AND lease_token = ?`, issueID, req.LeaseToken)
	if err != nil {
		return core.HandoffResponse{}, fmt.Errorf("delete lease: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return core.HandoffResponse{}, fmt.Errorf("handoff lease rows affected: %w", err)
	} else if rows != 1 {
		return core.HandoffResponse{}, core.NewAPIError(core.ErrLeaseExpired, "active lease not found")
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE issues SET
		     status = CASE WHEN status = 'blocked' THEN 'blocked' ELSE 'open' END,
		     claimed_at = NULL,
		     version = version + 1,
		     updated_at = ?
		 WHERE id = ?`,
		now, issueID,
	); err != nil {
		return core.HandoffResponse{}, fmt.Errorf("update issue: %w", err)
	}
	if err := insertEvent(ctx, tx, issueID, holder, "issue_released", map[string]any{
		"attempt_id": attemptID,
		"end_reason": "handoff",
	}, now); err != nil {
		return core.HandoffResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return core.HandoffResponse{}, fmt.Errorf("commit handoff: %w", err)
	}
	return core.HandoffResponse{Note: note}, nil
}

func getActiveLease(ctx context.Context, db *sql.DB, issueID string) (*core.IssueLease, error) {
	row := db.QueryRowContext(ctx,
		`SELECT holder, lease_token, expires_at, attempt_id, session_id FROM leases WHERE issue_id = ? AND expires_at > ?`,
		issueID, time.Now().UTC().Format(time.RFC3339),
	)
	var lease core.IssueLease
	err := row.Scan(&lease.Holder, &lease.LeaseToken, &lease.ExpiresAt, &lease.AttemptID, &lease.SessionID)
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
		&i.ScopeKind, &i.IssueType, &i.Title, &i.ExternalKey, &i.Description, &i.AcceptanceCriteria, &i.Status, &i.Priority,
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
func UpdateIssue(ctx context.Context, db *sql.DB, issueID string, req core.UpdateIssueRequest) (core.Issue, error) {
	issue, lease, err := GetIssue(ctx, db, issueID)
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

	// Generic update is for metadata and non-terminal routing only. Closing and
	// reopening have dedicated authorization and audit paths.
	if req.Status != "" && req.Status != issue.Status {
		if isTerminalStatus(req.Status) {
			return core.Issue{}, core.NewAPIError(core.ErrValidationFailed,
				"terminal status changes require issue close or issue operator-close")
		}
		if isTerminalStatus(issue.Status) {
			return core.Issue{}, core.NewAPIError(core.ErrValidationFailed,
				"terminal issues require issue operator-reopen")
		}
		if err := core.ValidateStatusTransition(issue.Status, req.Status); err != nil {
			return core.Issue{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.BeginTx(ctx, nil)
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
	if req.IssueType != "" {
		if !core.ValidIssueType(req.IssueType) {
			return core.Issue{}, core.NewAPIError(core.ErrValidationFailed,
				"issue_type must be one of: "+strings.Join(core.IssueTypes, ", "))
		}
		sets = append(sets, "issue_type = ?")
		args = append(args, req.IssueType)
	}
	if req.ExternalKey != "" {
		sets = append(sets, "external_key = ?")
		args = append(args, req.ExternalKey)
	}
	if req.Description != "" {
		sets = append(sets, "description = ?")
		args = append(args, req.Description)
	}
	if req.AcceptanceCriteria != "" {
		sets = append(sets, "acceptance_criteria = ?")
		args = append(args, req.AcceptanceCriteria)
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
	result, err := tx.ExecContext(ctx, query, args...)
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
	if req.Status != "" && req.Status != issue.Status {
		eventPayload["from_status"] = issue.Status
		eventPayload["to_status"] = req.Status
	}
	payloadBytes, _ := json.Marshal(eventPayload)
	_, err = tx.ExecContext(ctx,
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
	updated, _, err := GetIssue(ctx, db, issueID)
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
	if req.IssueType != "" {
		changed = append(changed, "issue_type")
	}
	if req.ExternalKey != "" {
		changed = append(changed, "external_key")
	}
	if req.Description != "" {
		changed = append(changed, "description")
	}
	if req.AcceptanceCriteria != "" {
		changed = append(changed, "acceptance_criteria")
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

// CloseIssue closes a non-terminal issue through the agent path. The caller
// must prove it still owns an active lease for the exact issue version.
func CloseIssue(ctx context.Context, db *sql.DB, issueID string, req core.CloseIssueRequest) (core.CloseIssueResult, error) {
	if err := validateResolution(req.Resolution); err != nil {
		return core.CloseIssueResult{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	issue, err := getIssueForTerminalTransition(ctx, tx, issueID, req.ExpectedVersion)
	if err != nil {
		return core.CloseIssueResult{}, err
	}
	if isTerminalStatus(issue.Status) {
		return core.CloseIssueResult{}, core.NewAPIError(core.ErrValidationFailed,
			"issue is already terminal")
	}
	if err := validateTerminalTransition(issue.Status, req.Resolution); err != nil {
		return core.CloseIssueResult{}, err
	}

	var leaseToken, attemptID string
	err = tx.QueryRowContext(ctx,
		`SELECT lease_token, attempt_id FROM leases WHERE issue_id = ? AND expires_at > ?`,
		issueID, now,
	).Scan(&leaseToken, &attemptID)
	if err == sql.ErrNoRows {
		return core.CloseIssueResult{}, core.NewAPIError(core.ErrLeaseExpired,
			"an active lease is required to close an issue")
	}
	if err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("select lease: %w", err)
	}
	if leaseToken != req.LeaseToken {
		return core.CloseIssueResult{}, core.NewAPIError(core.ErrLeaseExpired,
			"lease_token does not match the active lease")
	}

	result, err := updateTerminalIssue(ctx, tx, issue, req.Resolution, now)
	if err != nil {
		return core.CloseIssueResult{}, err
	}
	result.Branch = req.Branch
	result.PRURL = req.PRURL
	result.CommitSHA = req.CommitSHA

	if _, err := tx.ExecContext(ctx, `DELETE FROM leases WHERE issue_id = ? AND lease_token = ?`, issueID, req.LeaseToken); err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("delete lease: %w", err)
	}

	// A close note is appended before its closing event so the audit stream has
	// a deterministic note-then-close order.
	if req.Note != "" {
		if err := insertCloseNote(ctx, tx, issueID, req.Actor, req.Note, now); err != nil {
			return core.CloseIssueResult{}, err
		}
	}

	payload := terminalEventPayload(issue.Status, req.Resolution, issue.ExternalKey)
	payload["attempt_id"] = attemptID
	payload["end_reason"] = req.Resolution
	if req.Branch != "" {
		payload["branch"] = req.Branch
	}
	if req.PRURL != "" {
		payload["pr_url"] = req.PRURL
	}
	if req.CommitSHA != "" {
		payload["commit_sha"] = req.CommitSHA
	}
	if err := insertEvent(ctx, tx, issueID, req.Actor, "issue_closed", payload, now); err != nil {
		return core.CloseIssueResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("commit tx: %w", err)
	}
	return result, nil
}

// OperatorCloseIssue closes unclaimable or administratively managed work on a
// distinct, tokenless local-operator path. It is deliberately separate from
// CloseIssue so it cannot be mistaken for agent lease authorization.
func OperatorCloseIssue(ctx context.Context, db *sql.DB, issueID string, req core.OperatorCloseIssueRequest) (core.CloseIssueResult, error) {
	if err := validateOperatorRequest(req.ExpectedVersion, req.Actor, req.Reason); err != nil {
		return core.CloseIssueResult{}, err
	}
	if err := validateResolution(req.Resolution); err != nil {
		return core.CloseIssueResult{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	issue, err := getIssueForTerminalTransition(ctx, tx, issueID, req.ExpectedVersion)
	if err != nil {
		return core.CloseIssueResult{}, err
	}
	if isTerminalStatus(issue.Status) {
		return core.CloseIssueResult{}, core.NewAPIError(core.ErrValidationFailed,
			"issue is already terminal")
	}
	if err := validateTerminalTransition(issue.Status, req.Resolution); err != nil {
		return core.CloseIssueResult{}, err
	}

	result, err := updateTerminalIssue(ctx, tx, issue, req.Resolution, now)
	if err != nil {
		return core.CloseIssueResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM leases WHERE issue_id = ?`, issueID); err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("delete leases: %w", err)
	}

	payload := terminalEventPayload(issue.Status, req.Resolution, issue.ExternalKey)
	payload["reason"] = req.Reason
	if err := insertEvent(ctx, tx, issueID, req.Actor, "issue_operator_closed", payload, now); err != nil {
		return core.CloseIssueResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("commit tx: %w", err)
	}
	return result, nil
}

// OperatorReopenIssue reopens done or cancelled work on the explicit local
// operator path. Generic metadata updates may not reopen terminal issues.
func OperatorReopenIssue(ctx context.Context, db *sql.DB, issueID string, req core.OperatorReopenIssueRequest) (core.Issue, error) {
	if err := validateOperatorRequest(req.ExpectedVersion, req.Actor, req.Reason); err != nil {
		return core.Issue{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.Issue{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	issue, err := getIssueForTerminalTransition(ctx, tx, issueID, req.ExpectedVersion)
	if err != nil {
		return core.Issue{}, err
	}
	if !isTerminalStatus(issue.Status) {
		return core.Issue{}, core.NewAPIError(core.ErrValidationFailed,
			"only terminal issues can be reopened")
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE issues
		 SET status = 'open', closed_at = NULL, claimed_at = NULL, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ?`,
		now, issueID, req.ExpectedVersion,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("reopen issue: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return core.Issue{}, fmt.Errorf("reopen rows affected: %w", err)
	} else if rows != 1 {
		return core.Issue{}, core.NewAPIError(core.ErrConflict, "issue changed while reopening")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM leases WHERE issue_id = ?`, issueID); err != nil {
		return core.Issue{}, fmt.Errorf("delete leases: %w", err)
	}
	if err := insertEvent(ctx, tx, issueID, req.Actor, "issue_reopened", map[string]any{
		"from_status": issue.Status,
		"to_status":   "open",
		"reason":      req.Reason,
	}, now); err != nil {
		return core.Issue{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.Issue{}, fmt.Errorf("commit tx: %w", err)
	}

	updated, _, err := GetIssue(ctx, db, issueID)
	if err != nil {
		return core.Issue{}, fmt.Errorf("re-read reopened issue: %w", err)
	}
	return updated, nil
}

// OperatorReleaseIssue force-clears an issue's lease without a lease token
// and returns it to open, on the explicit local operator path. It is the
// recovery path for a claim whose lease token was lost before its TTL
// naturally expired it: without this, the only way to unstick the issue
// before expiry was operator-close followed by operator-reopen. Unlike
// operator-close, it never touches resolution — the work is not considered
// done, just unstuck for someone (possibly the same agent) to reclaim.
func OperatorReleaseIssue(ctx context.Context, db *sql.DB, issueID string, req core.OperatorReleaseIssueRequest) (core.Issue, error) {
	if err := validateOperatorRequest(req.ExpectedVersion, req.Actor, req.Reason); err != nil {
		return core.Issue{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.Issue{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	issue, err := getIssueForTerminalTransition(ctx, tx, issueID, req.ExpectedVersion)
	if err != nil {
		return core.Issue{}, err
	}
	if issue.Status != "in_progress" {
		return core.Issue{}, core.NewAPIError(core.ErrValidationFailed,
			"only in_progress issues can be force-released; use operator-close for terminal resolution or operator-reopen for terminal issues")
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE issues
		 SET status = 'open', claimed_at = NULL, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ?`,
		now, issueID, req.ExpectedVersion,
	)
	if err != nil {
		return core.Issue{}, fmt.Errorf("release issue: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return core.Issue{}, fmt.Errorf("release rows affected: %w", err)
	} else if rows != 1 {
		return core.Issue{}, core.NewAPIError(core.ErrConflict, "issue changed while releasing")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM leases WHERE issue_id = ?`, issueID); err != nil {
		return core.Issue{}, fmt.Errorf("delete leases: %w", err)
	}
	if err := insertEvent(ctx, tx, issueID, req.Actor, "issue_operator_released", map[string]any{
		"reason": req.Reason,
	}, now); err != nil {
		return core.Issue{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.Issue{}, fmt.Errorf("commit tx: %w", err)
	}

	updated, _, err := GetIssue(ctx, db, issueID)
	if err != nil {
		return core.Issue{}, fmt.Errorf("re-read released issue: %w", err)
	}
	return updated, nil
}

func isTerminalStatus(status string) bool {
	return status == "done" || status == "cancelled"
}

func validateResolution(resolution string) error {
	if resolution != "done" && resolution != "cancelled" {
		return core.NewAPIError(core.ErrValidationFailed, "resolution must be 'done' or 'cancelled'")
	}
	return nil
}

func validateTerminalTransition(from, to string) error {
	if err := core.ValidateStatusTransition(from, to); err != nil {
		return core.NewAPIError(core.ErrValidationFailed, err.Error())
	}
	return nil
}

func validateOperatorRequest(expectedVersion int, actor, reason string) error {
	if expectedVersion <= 0 {
		return core.NewAPIError(core.ErrValidationFailed, "expected_version is required")
	}
	if strings.TrimSpace(actor) == "" {
		return core.NewAPIError(core.ErrValidationFailed, "actor is required")
	}
	if strings.TrimSpace(reason) == "" {
		return core.NewAPIError(core.ErrValidationFailed, "reason is required")
	}
	return nil
}

func getIssueForTerminalTransition(ctx context.Context, tx *sql.Tx, issueID string, expectedVersion int) (core.Issue, error) {
	var issue core.Issue
	err := tx.QueryRowContext(ctx,
		`SELECT id, status, external_key, version FROM issues WHERE id = ?`, issueID,
	).Scan(&issue.ID, &issue.Status, &issue.ExternalKey, &issue.Version)
	if err == sql.ErrNoRows {
		return core.Issue{}, core.NewAPIError(core.ErrNotFound, "issue not found: "+issueID)
	}
	if err != nil {
		return core.Issue{}, fmt.Errorf("select issue: %w", err)
	}
	if expectedVersion != issue.Version {
		return core.Issue{}, core.NewAPIError(core.ErrConflict,
			fmt.Sprintf("expected version %d, current version is %d", expectedVersion, issue.Version))
	}
	return issue, nil
}

func updateTerminalIssue(ctx context.Context, tx *sql.Tx, issue core.Issue, resolution, now string) (core.CloseIssueResult, error) {
	result, err := tx.ExecContext(ctx,
		`UPDATE issues
		 SET status = ?, closed_at = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ?`,
		resolution, now, now, issue.ID, issue.Version,
	)
	if err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("update issue: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return core.CloseIssueResult{}, fmt.Errorf("close rows affected: %w", err)
	} else if rows != 1 {
		return core.CloseIssueResult{}, core.NewAPIError(core.ErrConflict, "issue changed while closing")
	}
	return core.CloseIssueResult{
		Status:      "closed",
		Resolution:  resolution,
		ExternalKey: issue.ExternalKey,
		ClosedAt:    now,
	}, nil
}

func insertCloseNote(ctx context.Context, tx *sql.Tx, issueID, actor, note, now string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes (id, issue_id, author, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, actor, note, now,
	); err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	return insertEvent(ctx, tx, issueID, actor, "note_added", map[string]any{}, now)
}

func terminalEventPayload(fromStatus, resolution, externalKey string) map[string]any {
	payload := map[string]any{
		"changed":     []string{"status", "closed_at"},
		"from_status": fromStatus,
		"to_status":   resolution,
		"resolution":  resolution,
	}
	if externalKey != "" {
		payload["external_key"] = externalKey
	}
	return payload
}

func insertEvent(ctx context.Context, tx *sql.Tx, issueID, actor, eventType string, payload map[string]any, now string) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, actor, eventType, string(payloadBytes), now,
	); err != nil {
		return fmt.Errorf("insert %s event: %w", eventType, err)
	}
	return nil
}

// AddDependency adds a dependency between two issues. For 'blocks' kind, it performs cycle detection.
func AddDependency(ctx context.Context, db *sql.DB, issueID string, req core.AddDependencyRequest) error {
	// Verify both issues exist and resolve DependsOn
	if _, _, err := GetIssue(ctx, db, issueID); err != nil {
		return err
	}
	dependsOnID, err := ResolveIssueID(ctx, db, req.DependsOn)
	if err != nil {
		return err
	}
	if _, _, err := GetIssue(ctx, db, dependsOnID); err != nil {
		return err
	}
	req.DependsOn = dependsOnID

	kind := req.Kind
	if kind == "" {
		kind = "blocks"
	}

	if kind == "blocks" {
		if wouldCreateCycle(ctx, db, issueID, req.DependsOn) {
			return core.NewAPIError(core.ErrDependencyCycle,
				"adding this dependency would create a cycle")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
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
	_, err = tx.ExecContext(ctx,
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
func RemoveDependency(ctx context.Context, db *sql.DB, issueID, dependsOn, kind, actor string) error {
	if kind == "" {
		kind = "blocks"
	}

	dependsOnID, err := ResolveIssueID(ctx, db, dependsOn)
	if err != nil {
		return err
	}
	dependsOn = dependsOnID

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx,
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
	_, err = tx.ExecContext(ctx,
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
func wouldCreateCycle(ctx context.Context, db *sql.DB, fromIssueID, toIssueID string) bool {
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
		rows, err := db.QueryContext(ctx,
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
func LinkArtifact(ctx context.Context, db *sql.DB, issueID string, req core.LinkArtifactRequest) (string, error) {
	// Verify the issue exists to get its repository_id for path resolution.
	issue, _, err := GetIssue(ctx, db, issueID)
	if err != nil {
		return "", err
	}

	// Resolve artifact by ID or relative path.
	artifactID, err := ResolveArtifactID(ctx, db, issue.RepositoryID, req.Artifact)
	if err != nil {
		return "", err
	}

	relation := req.Relation
	if relation == "" {
		relation = "implements"
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.ExecContext(ctx,
		`INSERT INTO issue_artifacts (issue_id, artifact_id, relation, created_at)
		 VALUES (?, ?, ?, ?)`,
		issueID, artifactID, relation, now,
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
func ListIssueArtifacts(ctx context.Context, db *sql.DB, issueID string) ([]core.ArtifactRef, error) {
	rows, err := db.QueryContext(ctx,
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

// UnlinkArtifact removes an artifact link from an issue. When relation is empty
// it removes every relation to that artifact; otherwise only the matching one.
func UnlinkArtifact(ctx context.Context, db *sql.DB, issueID, artifact, relation, actor string) error {
	issue, _, err := GetIssue(ctx, db, issueID)
	if err != nil {
		return err
	}

	artifactID, err := ResolveArtifactID(ctx, db, issue.RepositoryID, artifact)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var result sql.Result
	if relation != "" {
		result, err = tx.ExecContext(ctx,
			`DELETE FROM issue_artifacts WHERE issue_id = ? AND artifact_id = ? AND relation = ?`,
			issueID, artifactID, relation)
	} else {
		result, err = tx.ExecContext(ctx,
			`DELETE FROM issue_artifacts WHERE issue_id = ? AND artifact_id = ?`,
			issueID, artifactID)
	}
	if err != nil {
		return fmt.Errorf("delete issue_artifact: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return core.NewAPIError(core.ErrNotFound, "artifact link not found")
	}

	payload, err := json.Marshal(map[string]string{
		"artifact": artifact,
		"relation": relation,
	})
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), issueID, actor, "artifact_unlinked",
		string(payload), now)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return tx.Commit()
}

// CreateNote inserts a new note on an issue.
func CreateNote(ctx context.Context, db *sql.DB, issueID string, req core.CreateNoteRequest) (core.Note, error) {
	// Verify issue exists.
	if _, _, err := GetIssue(ctx, db, issueID); err != nil {
		return core.Note{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.Note{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO notes (id, issue_id, author, body, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, issueID, req.Author, req.Body, now,
	)
	if err != nil {
		return core.Note{}, fmt.Errorf("insert note: %w", err)
	}

	// Append event.
	_, err = tx.ExecContext(ctx,
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
func ListNotes(ctx context.Context, db *sql.DB, issueID string) ([]core.Note, error) {
	// Verify issue exists.
	if _, _, err := GetIssue(ctx, db, issueID); err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx,
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
func ListEvents(ctx context.Context, db *sql.DB, issueID string) ([]core.Event, error) {
	// Verify issue exists.
	if _, _, err := GetIssue(ctx, db, issueID); err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT sequence, id, issue_id, actor, event_type, payload_json, created_at FROM events WHERE issue_id = ? ORDER BY sequence ASC`,
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
		if err := rows.Scan(&e.Sequence, &e.ID, &issueID, &e.Actor, &e.EventType, &e.PayloadJSON, &e.CreatedAt); err != nil {
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

type eventCursor struct {
	Sequence int64 `json:"sequence"`
}

type legacyEventCursor struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// ListGlobalEvents returns a global cursor-paginated event stream ordered by
// daemon-assigned sequence.
func ListGlobalEvents(ctx context.Context, db *sql.DB, since string, limit int) (core.EventPage, error) {
	if limit <= 0 {
		limit = 100
	}

	cursor, err := decodeEventCursor(ctx, db, since)
	if err != nil {
		return core.EventPage{}, err
	}

	var (
		rows *sql.Rows
	)
	if cursor == nil {
		rows, err = db.QueryContext(ctx,
			`SELECT sequence, id, issue_id, actor, event_type, payload_json, created_at
			 FROM events
			 ORDER BY sequence ASC
			 LIMIT ?`,
			limit,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT sequence, id, issue_id, actor, event_type, payload_json, created_at
			 FROM events
			 WHERE sequence > ?
			 ORDER BY sequence ASC
			 LIMIT ?`,
			cursor.Sequence, limit,
		)
	}
	if err != nil {
		return core.EventPage{}, fmt.Errorf("list global events: %w", err)
	}
	defer rows.Close()

	events := make([]core.Event, 0)
	for rows.Next() {
		var e core.Event
		var issueID sql.NullString
		if err := rows.Scan(&e.Sequence, &e.ID, &issueID, &e.Actor, &e.EventType, &e.PayloadJSON, &e.CreatedAt); err != nil {
			return core.EventPage{}, fmt.Errorf("scan global event: %w", err)
		}
		if issueID.Valid {
			e.IssueID = issueID.String
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return core.EventPage{}, fmt.Errorf("iterate global events: %w", err)
	}

	page := core.EventPage{
		Events:    events,
		NextSince: since,
	}
	if page.Events == nil {
		page.Events = []core.Event{}
	}
	if len(page.Events) > 0 {
		last := page.Events[len(page.Events)-1]
		next, err := encodeEventCursor(eventCursor{Sequence: last.Sequence})
		if err != nil {
			return core.EventPage{}, err
		}
		page.NextSince = next
	}
	return page, nil
}

func encodeEventCursor(cursor eventCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal event cursor: %w", err)
	}
	return "v2." + base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeEventCursor(ctx context.Context, db *sql.DB, since string) (*eventCursor, error) {
	if since == "" {
		return nil, nil
	}

	decode := func(encoded string, destination any) error {
		raw, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return core.NewAPIError(core.ErrValidationFailed, "since cursor is invalid")
		}
		if err := json.Unmarshal(raw, destination); err != nil {
			return core.NewAPIError(core.ErrValidationFailed, "since cursor is invalid")
		}
		return nil
	}

	switch {
	case strings.HasPrefix(since, "v2."):
		var cursor eventCursor
		if err := decode(strings.TrimPrefix(since, "v2."), &cursor); err != nil {
			return nil, err
		}
		if cursor.Sequence <= 0 {
			return nil, core.NewAPIError(core.ErrValidationFailed, "since cursor is invalid")
		}
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM events WHERE sequence = ?`, cursor.Sequence).Scan(&exists)
		if err == sql.ErrNoRows {
			return nil, core.NewAPIError(core.ErrValidationFailed, "since cursor does not reference an event")
		}
		if err != nil {
			return nil, fmt.Errorf("resolve event cursor: %w", err)
		}
		return &cursor, nil
	case strings.HasPrefix(since, "v1."):
		var legacy legacyEventCursor
		if err := decode(strings.TrimPrefix(since, "v1."), &legacy); err != nil {
			return nil, err
		}
		if legacy.ID == "" || legacy.CreatedAt == "" {
			return nil, core.NewAPIError(core.ErrValidationFailed, "since cursor is invalid")
		}
		if _, err := time.Parse(time.RFC3339, legacy.CreatedAt); err != nil {
			return nil, core.NewAPIError(core.ErrValidationFailed, "since cursor is invalid")
		}
		var sequence int64
		err := db.QueryRowContext(ctx,
			`SELECT sequence FROM events WHERE id = ? AND created_at = ?`,
			legacy.ID, legacy.CreatedAt,
		).Scan(&sequence)
		if err == sql.ErrNoRows {
			return nil, core.NewAPIError(core.ErrValidationFailed, "since cursor does not reference an event")
		}
		if err != nil {
			return nil, fmt.Errorf("resolve legacy event cursor: %w", err)
		}
		return &eventCursor{Sequence: sequence}, nil
	default:
		return nil, core.NewAPIError(core.ErrValidationFailed, "since cursor must start with v2. or v1.")
	}
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
	scopeKind, issueType, title, externalKey, description, acceptanceCriteria, status string, priority int,
	assignee string, version int, claimedAt, closedAt, holder, leaseExpiresAt, createdAt, updatedAt string) core.Issue {
	i := core.Issue{
		ID:                 id,
		ShortID:            shortID,
		ProjectID:          projectID,
		ScopeKind:          scopeKind,
		IssueType:          issueType,
		Title:              title,
		ExternalKey:        externalKey,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
		Status:             status,
		Priority:           priority,
		Assignee:           assignee,
		Version:            version,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
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
