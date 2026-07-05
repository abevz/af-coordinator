package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abevz/af-coordinator/internal/core"
)

func TestCreateIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	p, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	req := core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "My test issue",
	}
	issue, err := CreateIssue(context.Background(), db, "test", req)
	if err != nil {
		t.Fatal(err)
	}
	if issue.ID == "" {
		t.Error("expected non-empty issue ID")
	}
	if issue.ShortID == "" {
		t.Error("expected non-empty short_id")
	}
	if issue.ShortID[:5] != "test-" {
		t.Errorf("expected short_id to start with 'test-', got %q", issue.ShortID)
	}
	if issue.Title != "My test issue" {
		t.Errorf("expected title %q, got %q", "My test issue", issue.Title)
	}
	if issue.Status != "open" {
		t.Errorf("expected status 'open', got %q", issue.Status)
	}
	if issue.ProjectID != p.ID {
		t.Errorf("expected project_id %q, got %q", p.ID, issue.ProjectID)
	}
}

func TestCreateIssueIncrementsSeq(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Second"})
	if err != nil {
		t.Fatal(err)
	}

	if i1.ShortID != "test-1" {
		t.Errorf("expected first short_id 'test-1', got %q", i1.ShortID)
	}
	if i2.ShortID != "test-2" {
		t.Errorf("expected second short_id 'test-2', got %q", i2.ShortID)
	}
}

func TestCreateIssueProjectNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateIssue(context.Background(), db, "nonexistent", core.CreateIssueRequest{ScopeKind: "project", Title: "Fail"})
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestGetIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	created, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Get me"})
	if err != nil {
		t.Fatal(err)
	}

	got, lease, err := GetIssue(context.Background(), db, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, got.ID)
	}
	if lease != nil {
		t.Error("expected no lease on newly created issue")
	}
}

func TestGetIssueByShortID(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	created, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "By short ID"})
	if err != nil {
		t.Fatal(err)
	}

	resolvedID, err := ResolveIssueID(context.Background(), db, created.ShortID)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := GetIssue(context.Background(), db, resolvedID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, got.ID)
	}
	if got.ShortID != created.ShortID {
		t.Errorf("expected short_id %q, got %q", created.ShortID, got.ShortID)
	}
}

func TestGetIssueNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, _, err := GetIssue(context.Background(), db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestGetIssueByShortIDNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, _, err := GetIssue(context.Background(), db, "no-proj-42")
	if err == nil {
		t.Fatal("expected error for nonexistent short_id")
	}
}

func TestListIssues(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// No issues yet.
	issues, err := ListIssues(context.Background(), db, core.IssueListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}

	CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A"})
	CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "B"})

	issues, err = ListIssues(context.Background(), db, core.IssueListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
}

func TestListIssuesFilterByStatus(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Only open"})
	CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Only open 2"})

	// Use raw SQL to set one to 'done'.
	_, err = db.Exec("UPDATE issues SET status = 'done' WHERE id = (SELECT id FROM issues ORDER BY created_at DESC LIMIT 1)")
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(context.Background(), db, core.IssueListParams{Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 open issue, got %d", len(issues))
	}
}

func TestClaimIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Claimable"})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if claim.LeaseToken == "" {
		t.Error("expected non-empty lease_token")
	}
	if claim.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}

	// Verify issue status changed.
	got, lease, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", got.Status)
	}
	if lease == nil {
		t.Fatal("expected active lease")
	}
	if lease.Holder != "agent-1" {
		t.Errorf("expected holder 'agent-1', got %q", lease.Holder)
	}
}

func TestClaimIssueAlreadyClaimed(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Double claim"})
	if err != nil {
		t.Fatal(err)
	}

	// First claim succeeds.
	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	// Second claim should fail.
	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-2", 3600)
	if err == nil {
		t.Fatal("expected error for already-claimed issue")
	}

	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrLeaseHeld {
		t.Errorf("expected code %q, got %q", core.ErrLeaseHeld, apiErr.Code)
	}
}

func TestClaimIssueWithExpiredLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Expired lease claim"})
	if err != nil {
		t.Fatal(err)
	}

	// Claim it first.
	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	// Manually expire the lease in the DB and keep the status as 'in_progress'.
	_, err = db.Exec(
		`UPDATE leases SET expires_at = ? WHERE issue_id = ?`,
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		issue.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Second claim (by agent-2) should now succeed because the lease has expired,
	// even though the status is still 'in_progress'.
	resp, err := ClaimIssue(context.Background(), db, issue.ID, "agent-2", 3600)
	if err != nil {
		t.Fatalf("expected claim on expired lease to succeed, got: %v", err)
	}
	if resp.LeaseToken == "" {
		t.Error("expected new lease token to be returned")
	}
}

func TestClaimIssueNonexistent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := ClaimIssue(context.Background(), db, "nonexistent", "agent-1", 3600)
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestHeartbeatLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Heart me"})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	newExpires, err := HeartbeatLease(context.Background(), db, issue.ID, claim.LeaseToken, 7200)
	if err != nil {
		t.Fatal(err)
	}
	if newExpires == "" {
		t.Error("expected non-empty new expires_at")
	}
	if newExpires == claim.ExpiresAt {
		t.Error("expected new expires_at to differ from original")
	}
}

func TestHeartbeatLeaseWrongToken(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Heart fail"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = HeartbeatLease(context.Background(), db, issue.ID, "wrong-token", 7200)
	if err == nil {
		t.Fatal("expected error for wrong lease token")
	}
}

func TestReleaseLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Release me"})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	err = ReleaseLease(context.Background(), db, issue.ID, claim.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}

	// Verify lease is gone and issue is back to open.
	got, lease, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "open" {
		t.Errorf("expected status 'open' after release, got %q", got.Status)
	}
	if lease != nil {
		t.Error("expected lease to be nil after release")
	}
}

func TestReleaseLeaseWrongToken(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Release fail"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	err = ReleaseLease(context.Background(), db, issue.ID, "wrong-token")
	if err == nil {
		t.Fatal("expected error for wrong lease token")
	}
}

func TestListReadyIssues(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create two open issues.
	issue1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Open issue"})
	if err != nil {
		t.Fatal(err)
	}
	issue2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Done issue"})
	if err != nil {
		t.Fatal(err)
	}

	// Mark the second issue as 'done' via raw SQL.
	_, err = db.Exec("UPDATE issues SET status = 'done' WHERE id = ?", issue2.ID)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := ListReadyIssues(context.Background(), db, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 {
		t.Errorf("expected 1 ready issue, got %d", len(ready))
	}
	if len(ready) > 0 && ready[0].ID != issue1.ID {
		t.Errorf("expected ready issue to be the open one (%q), got %q", issue1.ID, ready[0].ID)
	}
}

func TestListReadyIssuesExcludesLeased(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Leased issue"})
	if err != nil {
		t.Fatal(err)
	}

	// Claim the issue → it becomes in_progress with an active lease.
	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	// ListReadyIssues should exclude leased issues.
	ready, err := ListReadyIssues(context.Background(), db, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 0 {
		t.Errorf("expected 0 ready issues (issue is leased), got %d", len(ready))
	}
}

func TestListReadyIssuesByProject(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	p1, err := CreateProject(context.Background(), db, "p1", "Project 1", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreateProject(context.Background(), db, "p2", "Project 2", "")
	if err != nil {
		t.Fatal(err)
	}

	CreateIssue(context.Background(), db, "p1", core.CreateIssueRequest{ScopeKind: "project", Title: "P1 issue"})
	CreateIssue(context.Background(), db, "p2", core.CreateIssueRequest{ScopeKind: "project", Title: "P2 issue"})

	ready, err := ListReadyIssues(context.Background(), db, p1.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 {
		t.Errorf("expected 1 ready issue for project p1, got %d", len(ready))
	}
	if len(ready) > 0 && ready[0].Title != "P1 issue" {
		t.Errorf("expected title 'P1 issue', got %q", ready[0].Title)
	}
}

func TestListReadyIssuesWithExpiredLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Expired lease"})
	if err != nil {
		t.Fatal(err)
	}

	// Insert an already-expired lease directly.
	_, err = db.Exec(
		`INSERT INTO leases (issue_id, holder, lease_token, expires_at, created_at, updated_at)
		 VALUES (?, 'old-agent', 'old-token', ?, ?, ?)`,
		issue.ID,
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := ListReadyIssues(context.Background(), db, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 {
		t.Errorf("expected 1 ready issue (expired lease should be ignored), got %d", len(ready))
	}
}

func TestCreateIssueWithRepoScope(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(context.Background(), db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "my-repo",
		CanonicalGitDir: "/repos/my-repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "repository",
		Repo:      repo.ID,
		Title:     "Repo-scoped issue",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue.RepositoryID != repo.ID {
		t.Errorf("expected repository_id %q, got %q", repo.ID, issue.RepositoryID)
	}
	if issue.ScopeKind != "repository" {
		t.Errorf("expected scope_kind 'repository', got %q", issue.ScopeKind)
	}
	if issue.ProjectID == "" {
		t.Error("expected non-empty project_id")
	}
}

func TestListReadyIssuesWithLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create two issues.
	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Ready issue",
	})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Claimed issue",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Claim i2.
	_, err = ClaimIssue(context.Background(), db, i2.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	// List ready issues: only i1 (unclaimed) should appear.
	ready, err := ListReadyIssues(context.Background(), db, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(ready))
	}
	if ready[0].ID != i1.ID {
		t.Errorf("expected ready issue ID %q, got %q", i1.ID, ready[0].ID)
	}
	if ready[0].Title != "Ready issue" {
		t.Errorf("expected title 'Ready issue', got %q", ready[0].Title)
	}
}

func TestUpdateIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Original title",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		Title:           "Updated title",
		ExpectedVersion: issue.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated title" {
		t.Errorf("expected title 'Updated title', got %q", updated.Title)
	}
	if updated.Version != issue.Version+1 {
		t.Errorf("expected version %d, got %d", issue.Version+1, updated.Version)
	}
}

func TestUpdateIssueVersionConflict(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Version test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// First update succeeds.
	_, err = UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		Title:           "First update",
		ExpectedVersion: issue.Version,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Second update with stale version should fail.
	_, err = UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		Title:           "Stale update",
		ExpectedVersion: issue.Version, // old version
	})
	if err == nil {
		t.Fatal("expected error for version conflict")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
}

func TestCloseIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Close me",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = CloseIssue(context.Background(), db, issue.ID, core.CloseIssueRequest{
		Resolution:      "done",
		ExpectedVersion: issue.Version,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's closed.
	got, _, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "done" {
		t.Errorf("expected status 'done', got %q", got.Status)
	}
	if got.ClosedAt == "" {
		t.Error("expected non-empty closed_at")
	}
	if got.Version != issue.Version+1 {
		t.Errorf("expected version %d, got %d", issue.Version+1, got.Version)
	}
}

func TestCloseIssueCancelled(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Cancel me",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = CloseIssue(context.Background(), db, issue.ID, core.CloseIssueRequest{
		Resolution:      "cancelled",
		ExpectedVersion: issue.Version,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got %q", got.Status)
	}
}

func TestCloseIssueVersionConflict(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Conflict",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Use wrong expected version.
	err = CloseIssue(context.Background(), db, issue.ID, core.CloseIssueRequest{
		Resolution:      "done",
		ExpectedVersion: 99,
	})
	if err == nil {
		t.Fatal("expected error for version conflict")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrConflict {
		t.Errorf("expected code %q, got %q", core.ErrConflict, apiErr.Code)
	}
}

func TestCreateIssueAppendsEvent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Event test"})
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "issue_created" {
		t.Errorf("expected event_type 'issue_created', got %q", events[0].EventType)
	}
}

func TestClaimIssueAppendsEvent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Claim event"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should have issue_created + issue_claimed.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	found := false
	for _, e := range events {
		if e.EventType == "issue_claimed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an event with event_type 'issue_claimed'")
	}
}

func TestReleaseLeaseAppendsEvent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Release event"})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	err = ReleaseLease(context.Background(), db, issue.ID, claim.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should have issue_created + issue_claimed + issue_released.
	found := false
	for _, e := range events {
		if e.EventType == "issue_released" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an event with event_type 'issue_released'")
	}
}

func TestCreateNoteAppendsEvent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Note event"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateNote(context.Background(), db, issue.ID, core.CreateNoteRequest{Author: "tester", Body: "A note"})
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should have issue_created + note_added.
	found := false
	for _, e := range events {
		if e.EventType == "note_added" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an event with event_type 'note_added'")
	}
}

func TestAddDependencyAppendsEvent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Source"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Target"})
	if err != nil {
		t.Fatal(err)
	}

	err = AddDependency(context.Background(), db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, i1.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should have issue_created + dependency_added.
	found := false
	for _, e := range events {
		if e.EventType == "dependency_added" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an event with event_type 'dependency_added'")
	}
}

func TestRemoveDependencyAppendsEvent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Src"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Dst"})
	if err != nil {
		t.Fatal(err)
	}

	err = AddDependency(context.Background(), db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}

	err = RemoveDependency(context.Background(), db, i1.ID, i2.ID, "blocks", "test")
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, i1.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Should have issue_created + dependency_added + dependency_removed.
	found := false
	for _, e := range events {
		if e.EventType == "dependency_removed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an event with event_type 'dependency_removed'")
	}
}

func TestListIssuesFilterByAssignee(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Unassigned"})

	// Update assignee directly.
	_, err = db.Exec("UPDATE issues SET assignee = 'alice' WHERE title = 'Unassigned'")
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := ListIssues(context.Background(), db, core.IssueListParams{Assignee: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assigned) != 1 {
		t.Errorf("expected 1 issue assigned to alice, got %d", len(assigned))
	}
}

func TestListIssuesEmpty(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(context.Background(), db, core.IssueListParams{})
	if err != nil {
		t.Fatal(err)
	}
	// Should return empty slice, not nil.
	if issues == nil {
		t.Error("expected non-nil slice when no issues")
	}
}

func TestListIssuesShowsHolder(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Leased issue"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-42", 3600)
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(context.Background(), db, core.IssueListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Holder != "agent-42" {
		t.Errorf("expected holder 'agent-42', got %q", issues[0].Holder)
	}
}

func TestListIssuesWithAssigneeFilter(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Alpha"})

	_, err = db.Exec("UPDATE issues SET assignee = ? WHERE title = ?", "bob", "Alpha")
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(context.Background(), db, core.IssueListParams{Assignee: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue for bob, got %d", len(issues))
	}
}

func TestListIssuesReturnsEmptySlice(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(context.Background(), db, core.IssueListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if issues == nil {
		t.Error("expected []core.Issue{}, got nil")
	}
}

func TestGetIssueIncludesLeasedFields(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Lease view"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-7", 3600)
	if err != nil {
		t.Fatal(err)
	}

	got, lease, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("expected lease to be non-nil")
	}
	if got.Holder != "agent-7" {
		t.Errorf("expected issue.Holder 'agent-7', got %q", got.Holder)
	}
	if lease.Holder != "agent-7" {
		t.Errorf("expected lease.Holder 'agent-7', got %q", lease.Holder)
	}
}

func TestGetIssueWithLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Lease test"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	got, lease, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("expected lease for claimed issue")
	}
	if lease.Holder != "agent-1" {
		t.Errorf("expected holder 'agent-1', got %q", lease.Holder)
	}
	if got.Holder != "agent-1" {
		t.Errorf("expected issue.holder 'agent-1', got %q", got.Holder)
	}
}

func TestCreateIssueWithWorktreeScope(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(context.Background(), db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	wt, _, err := UpsertWorktree(context.Background(), db, repo.ID, core.CreateWorktreeRequest{
		Repo:         repo.ID,
		AbsolutePath: "/worktrees/feature",
		Branch:       "feature-x",
	})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "worktree",
		Repo:      repo.ID,
		Worktree:  wt.ID,
		Title:     "Worktree-scoped",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue.WorktreeID != wt.ID {
		t.Errorf("expected worktree_id %q, got %q", wt.ID, issue.WorktreeID)
	}
	if issue.RepositoryID != repo.ID {
		t.Errorf("expected repository_id %q, got %q", repo.ID, issue.RepositoryID)
	}
	if issue.ScopeKind != "worktree" {
		t.Errorf("expected scope_kind 'worktree', got %q", issue.ScopeKind)
	}
}

func TestUpdateIssueAssignment(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Assign me"})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		Assignee:        "alice",
		ExpectedVersion: issue.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Assignee != "alice" {
		t.Errorf("expected assignee 'alice', got %q", updated.Assignee)
	}
}

func TestUpdateIssueLeaseTokenRequired(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Lease locked"})
	if err != nil {
		t.Fatal(err)
	}

	// Claim the issue.
	_, err = ClaimIssue(context.Background(), db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to update without providing the lease_token should fail.
	_, err = UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		Title:           "Hack attempt",
		ExpectedVersion: 2,
	})
	if err == nil {
		t.Fatal("expected error for missing lease_token on leased issue")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrLeaseExpired {
		t.Errorf("expected code %q, got %q", core.ErrLeaseExpired, apiErr.Code)
	}
}

func TestCreateIssueDefaultPriority(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Default priority"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Priority != 3 {
		t.Errorf("expected default priority 3, got %d", issue.Priority)
	}
}

func TestCreateIssueCustomPriority(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "High priority",
		Priority:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Priority != 1 {
		t.Errorf("expected priority 1, got %d", issue.Priority)
	}
}

func TestLinkArtifact(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(context.Background(), db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Link test"})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := CreateArtifact(context.Background(), db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "sdd",
		RelativePath: "docs/spec.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	createdAt, err := LinkArtifact(context.Background(), db, issue.ID, core.LinkArtifactRequest{Artifact: artifact.ID})
	if err != nil {
		t.Fatal(err)
	}
	if createdAt == "" {
		t.Error("expected non-empty created_at")
	}

	refs, err := ListIssueArtifacts(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 artifact ref, got %d", len(refs))
	}
	if refs[0].ID != artifact.ID {
		t.Errorf("expected artifact ID %q, got %q", artifact.ID, refs[0].ID)
	}
	if refs[0].Relation != "implements" {
		t.Errorf("expected relation 'implements', got %q", refs[0].Relation)
	}
}

func TestLinkArtifactCustomRelation(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(context.Background(), db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Custom rel"})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := CreateArtifact(context.Background(), db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "adr",
		RelativePath: "docs/adr.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = LinkArtifact(context.Background(), db, issue.ID, core.LinkArtifactRequest{
		Artifact: artifact.ID,
		Relation: "documents",
	})
	if err != nil {
		t.Fatal(err)
	}

	refs, err := ListIssueArtifacts(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refs[0].Relation != "documents" {
		t.Errorf("expected relation 'documents', got %q", refs[0].Relation)
	}
}

func TestLinkArtifactNonexistentIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := LinkArtifact(context.Background(), db, "nonexistent", core.LinkArtifactRequest{Artifact: "art-1"})
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestLinkArtifactNonexistentArtifact(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Link fail"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = LinkArtifact(context.Background(), db, issue.ID, core.LinkArtifactRequest{Artifact: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent artifact")
	}
}

func TestLinkArtifactDuplicate(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := CreateRepo(context.Background(), db, "test", core.CreateRepoRequest{
		Project:         "test",
		LogicalName:     "repo",
		CanonicalGitDir: "/repos/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Dup link"})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := CreateArtifact(context.Background(), db, repo.ID, core.CreateArtifactRequest{
		Repo:         repo.ID,
		Kind:         "sdd",
		RelativePath: "docs/dup.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	// First link succeeds.
	_, err = LinkArtifact(context.Background(), db, issue.ID, core.LinkArtifactRequest{Artifact: artifact.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Second link with same relation should fail.
	_, err = LinkArtifact(context.Background(), db, issue.ID, core.LinkArtifactRequest{Artifact: artifact.ID})
	if err == nil {
		t.Fatal("expected error for duplicate link")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrAlreadyLinked {
		t.Errorf("expected code %q, got %q", core.ErrAlreadyLinked, apiErr.Code)
	}
}

func TestListIssueArtifactsEmpty(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "No artifacts"})
	if err != nil {
		t.Fatal(err)
	}

	refs, err := ListIssueArtifacts(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 artifact refs, got %d", len(refs))
	}
}

func TestCreateNote(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Note this"})
	if err != nil {
		t.Fatal(err)
	}

	note, err := CreateNote(context.Background(), db, issue.ID, core.CreateNoteRequest{Author: "me", Body: "A comment"})
	if err != nil {
		t.Fatal(err)
	}
	if note.Author != "me" {
		t.Errorf("expected author 'me', got %q", note.Author)
	}
	if note.Body != "A comment" {
		t.Errorf("expected body 'A comment', got %q", note.Body)
	}
	if note.IssueID != issue.ID {
		t.Errorf("expected issue_id %q, got %q", issue.ID, note.IssueID)
	}

	notes, err := ListNotes(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Body != "A comment" {
		t.Errorf("expected body 'A comment', got %q", notes[0].Body)
	}
}

func TestCreateNoteNonexistentIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateNote(context.Background(), db, "nonexistent", core.CreateNoteRequest{Author: "me", Body: "fail"})
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestListNotesEmpty(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Quiet"})
	if err != nil {
		t.Fatal(err)
	}

	notes, err := ListNotes(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

func TestListNotesNonexistentIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := ListNotes(context.Background(), db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestMultipleNotes(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Chatty"})
	if err != nil {
		t.Fatal(err)
	}

	CreateNote(context.Background(), db, issue.ID, core.CreateNoteRequest{Author: "alice", Body: "First"})
	CreateNote(context.Background(), db, issue.ID, core.CreateNoteRequest{Author: "bob", Body: "Second"})

	notes, err := ListNotes(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Body != "First" {
		t.Errorf("expected first note body 'First', got %q", notes[0].Body)
	}
	if notes[1].Body != "Second" {
		t.Errorf("expected second note body 'Second', got %q", notes[1].Body)
	}
}

func TestDependencyKindDefaults(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Source"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Target"})
	if err != nil {
		t.Fatal(err)
	}

	// Empty kind should default to "blocks".
	err = AddDependency(context.Background(), db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: ""})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssueDescriptionDefault(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Desc check"})
	if err != nil {
		t.Fatal(err)
	}
	// Default description should be empty string.
	if issue.Description != "" {
		t.Errorf("expected empty description, got %q", issue.Description)
	}
}

func TestIssueWithDescription(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind:   "project",
		Title:       "With desc",
		Description: "A detailed description",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Description != "A detailed description" {
		t.Errorf("expected description %q, got %q", "A detailed description", issue.Description)
	}
}

func TestListEventsEmpty(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Eventless"})
	if err != nil {
		t.Fatal(err)
	}

	events, err := ListEvents(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event (the create event), got %d", len(events))
	}
}

func TestAddRemoveDependency(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "B"})
	if err != nil {
		t.Fatal(err)
	}

	// A depends on B.
	err = AddDependency(context.Background(), db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}

	err = RemoveDependency(context.Background(), db, i1.ID, i2.ID, "blocks", "tester")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDependencyNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "X"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Y"})
	if err != nil {
		t.Fatal(err)
	}

	err = RemoveDependency(context.Background(), db, i1.ID, i2.ID, "blocks", "tester")
	if err == nil {
		t.Fatal("expected error for removing nonexistent dependency")
	}
}

func TestDependencyCycleDetection(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "B"})
	if err != nil {
		t.Fatal(err)
	}

	// A depends on B.
	err = AddDependency(context.Background(), db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}

	// B depending on A should create a cycle.
	err = AddDependency(context.Background(), db, i2.ID, core.AddDependencyRequest{DependsOn: i1.ID, Kind: "blocks"})
	if err == nil {
		t.Fatal("expected error for cycle detection")
	}

	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrDependencyCycle {
		t.Errorf("expected code %q, got %q", core.ErrDependencyCycle, apiErr.Code)
	}
}

func TestCreateIssueDefaultType(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Default type"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.IssueType != "task" {
		t.Errorf("expected issue_type 'task', got %q", issue.IssueType)
	}

	got, _, err := GetIssue(context.Background(), db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IssueType != "task" {
		t.Errorf("expected persisted issue_type 'task', got %q", got.IssueType)
	}
}

func TestCreateIssueWithType(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A bug", IssueType: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.IssueType != "bug" {
		t.Errorf("expected issue_type 'bug', got %q", issue.IssueType)
	}
}

func TestListIssuesFilterByType(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A task"})
	if err != nil {
		t.Fatal(err)
	}
	bug, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A bug", IssueType: "bug"})
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(context.Background(), db, core.IssueListParams{IssueType: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 bug, got %d", len(issues))
	}
	if issues[0].ID != bug.ID {
		t.Errorf("expected bug %q, got %q", bug.ID, issues[0].ID)
	}
}

func TestReadyExcludesEpics(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Big epic", IssueType: "epic"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Small task"})
	if err != nil {
		t.Fatal(err)
	}

	issues, err := ListReadyIssues(context.Background(), db, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(issues))
	}
	if issues[0].ID != task.ID {
		t.Errorf("expected task %q to be ready, got %q", task.ID, issues[0].ID)
	}
}

func TestClaimEpicRejected(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	epic, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Big epic", IssueType: "epic"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(context.Background(), db, epic.ID, "agent-1", 60)
	if err == nil {
		t.Fatal("expected error claiming an epic")
	}
	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrValidationFailed {
		t.Errorf("expected code %q, got %q", core.ErrValidationFailed, apiErr.Code)
	}
}

func TestUpdateIssueType(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(context.Background(), db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Reclassify me"})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		IssueType:       "bug",
		ExpectedVersion: issue.Version,
		Actor:           "tester",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.IssueType != "bug" {
		t.Errorf("expected issue_type 'bug', got %q", updated.IssueType)
	}

	// Invalid type is rejected.
	_, err = UpdateIssue(context.Background(), db, issue.ID, core.UpdateIssueRequest{
		IssueType:       "story",
		ExpectedVersion: updated.Version,
		Actor:           "tester",
	})
	if err == nil {
		t.Fatal("expected error for invalid issue_type")
	}
}
