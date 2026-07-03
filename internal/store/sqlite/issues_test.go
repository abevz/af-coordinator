package sqlite

import (
	"errors"
	"testing"

	"github.com/abevz/af-coordinator/internal/core"
)

func TestCreateIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	p, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	req := core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "My test issue",
	}
	issue, err := CreateIssue(db, "test", req)
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Second"})
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

	_, err := CreateIssue(db, "nonexistent", core.CreateIssueRequest{ScopeKind: "project", Title: "Fail"})
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestGetIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	created, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Get me"})
	if err != nil {
		t.Fatal(err)
	}

	got, lease, err := GetIssue(db, created.ID)
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	created, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "By short ID"})
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := GetIssue(db, created.ShortID)
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

	_, _, err := GetIssue(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestGetIssueByShortIDNotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, _, err := GetIssue(db, "no-proj-42")
	if err == nil {
		t.Fatal("expected error for nonexistent short_id")
	}
}

func TestListIssues(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// No issues yet.
	issues, err := ListIssues(db, core.IssueListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}

	CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A"})
	CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "B"})

	issues, err = ListIssues(db, core.IssueListParams{})
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Only open"})

	issues, err := ListIssues(db, core.IssueListParams{Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues with status 'done', got %d", len(issues))
	}

	// Filter by 'open' should return the issue.
	issues, err = ListIssues(db, core.IssueListParams{Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue with status 'open', got %d", len(issues))
	}
}

func TestClaimIssue(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Claim me"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if resp.LeaseToken == "" {
		t.Error("expected non-empty lease token")
	}
	if resp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}

	// Verify the issue was moved to in_progress.
	got, lease, err := GetIssue(db, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", got.Status)
	}
	if lease == nil {
		t.Fatal("expected lease to be present after claim")
	}
	if lease.Holder != "agent-1" {
		t.Errorf("expected holder 'agent-1', got %q", lease.Holder)
	}
}

func TestClaimIssueAlreadyClaimed(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Shared"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(db, issue.ID, "agent-2", 3600)
	if err == nil {
		t.Fatal("expected error when claiming already-claimed issue")
	}

	var apiErr core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != core.ErrLeaseHeld {
		t.Errorf("expected code %q, got %q", core.ErrLeaseHeld, apiErr.Code)
	}
}

func TestClaimIssueNonexistent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := ClaimIssue(db, "nonexistent", "agent-1", 3600)
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestHeartbeatLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Heart me"})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	newExpires, err := HeartbeatLease(db, issue.ID, claim.LeaseToken, 7200)
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Heart fail"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = HeartbeatLease(db, issue.ID, "wrong-token", 7200)
	if err == nil {
		t.Fatal("expected error for wrong lease token")
	}
}

func TestReleaseLease(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Release me"})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	err = ReleaseLease(db, issue.ID, claim.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}

	// Verify lease is gone and issue is back to open.
	got, lease, err := GetIssue(db, issue.ID)
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Release fail"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	err = ReleaseLease(db, issue.ID, "wrong-token")
	if err == nil {
		t.Fatal("expected error for wrong lease token")
	}
}

func TestListReadyIssues(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create two open issues.
	issue1, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Open issue"})
	if err != nil {
		t.Fatal(err)
	}
	issue2, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Done issue"})
	if err != nil {
		t.Fatal(err)
	}

	// Mark the second issue as 'done' via raw SQL.
	_, err = db.Exec("UPDATE issues SET status = 'done' WHERE id = ?", issue2.ID)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := ListReadyIssues(db, "")
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Leased issue"})
	if err != nil {
		t.Fatal(err)
	}

	// Claim the issue → it becomes in_progress with an active lease.
	_, err = ClaimIssue(db, issue.ID, "agent-1", 3600)
	if err != nil {
		t.Fatal(err)
	}

	// ListReadyIssues should exclude leased issues.
	ready, err := ListReadyIssues(db, "")
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

	p1, err := CreateProject(db, "p1", "Project 1", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreateProject(db, "p2", "Project 2", "")
	if err != nil {
		t.Fatal(err)
	}

	CreateIssue(db, "p1", core.CreateIssueRequest{ScopeKind: "project", Title: "P1 issue"})
	CreateIssue(db, "p2", core.CreateIssueRequest{ScopeKind: "project", Title: "P2 issue"})

	ready, err := ListReadyIssues(db, p1.ID)
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

func TestAddDependency(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Second"})
	if err != nil {
		t.Fatal(err)
	}

	err = AddDependency(db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddDependencyDuplicate(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "B"})
	if err != nil {
		t.Fatal(err)
	}

	err = AddDependency(db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}

	// Adding the same dependency again should fail.
	err = AddDependency(db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err == nil {
		t.Fatal("expected error for duplicate dependency")
	}
}

func TestAddDependencyCycleDetection(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "B"})
	if err != nil {
		t.Fatal(err)
	}

	// A depends on B.
	err = AddDependency(db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: "blocks"})
	if err != nil {
		t.Fatal(err)
	}

	// B depending on A should create a cycle.
	err = AddDependency(db, i2.ID, core.AddDependencyRequest{DependsOn: i1.ID, Kind: "blocks"})
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

func TestCreateNote(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Note this"})
	if err != nil {
		t.Fatal(err)
	}

	note, err := CreateNote(db, issue.ID, core.CreateNoteRequest{Author: "me", Body: "A comment"})
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

	notes, err := ListNotes(db, issue.ID)
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

	_, err := CreateNote(db, "nonexistent", core.CreateNoteRequest{Author: "me", Body: "fail"})
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestListNotesEmpty(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Quiet"})
	if err != nil {
		t.Fatal(err)
	}

	notes, err := ListNotes(db, issue.ID)
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

	_, err := ListNotes(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
}

func TestMultipleNotes(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Chatty"})
	if err != nil {
		t.Fatal(err)
	}

	CreateNote(db, issue.ID, core.CreateNoteRequest{Author: "alice", Body: "First"})
	CreateNote(db, issue.ID, core.CreateNoteRequest{Author: "bob", Body: "Second"})

	notes, err := ListNotes(db, issue.ID)
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	i1, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Source"})
	if err != nil {
		t.Fatal(err)
	}
	i2, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Target"})
	if err != nil {
		t.Fatal(err)
	}

	// Empty kind should default to "blocks".
	err = AddDependency(db, i1.ID, core.AddDependencyRequest{DependsOn: i2.ID, Kind: ""})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssueDescriptionDefault(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{ScopeKind: "project", Title: "Desc check"})
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

	_, err := CreateProject(db, "test", "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	issue, err := CreateIssue(db, "test", core.CreateIssueRequest{
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
