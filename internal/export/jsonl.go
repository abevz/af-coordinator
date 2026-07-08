package export

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/abevz/af-coordinator/internal/core"
	"github.com/abevz/af-coordinator/internal/store/sqlite"
)

// Record is a single JSONL envelope emitted by the export stream.
type Record struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Dependency captures an exported dependency edge as a first-class record.
type Dependency struct {
	IssueID          string `json:"issue_id"`
	IssueShortID     string `json:"issue_short_id"`
	DependsOnID      string `json:"depends_on_id"`
	DependsOnShortID string `json:"depends_on_short_id"`
	Kind             string `json:"kind"`
	CreatedAt        string `json:"created_at"`
}

// Reference captures an exported issue-to-artifact link.
type Reference struct {
	IssueID       string `json:"issue_id"`
	IssueShortID  string `json:"issue_short_id"`
	ArtifactID    string `json:"artifact_id"`
	ArtifactPath  string `json:"artifact_path"`
	ArtifactKind  string `json:"artifact_kind"`
	Relation      string `json:"relation"`
	LinkCreatedAt string `json:"link_created_at"`
}

// WriteJSONL streams a normalized JSONL export of coordinator state.
func WriteJSONL(ctx context.Context, db *sql.DB, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	emit := func(recordType string, payload any) error {
		if err := enc.Encode(Record{Type: recordType, Payload: payload}); err != nil {
			return fmt.Errorf("encode %s record: %w", recordType, err)
		}
		return nil
	}

	projects, err := sqlite.ListProjects(ctx, db)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	for _, project := range projects {
		if err := emit("project", project); err != nil {
			return err
		}
	}

	repos, err := sqlite.ListRepos(ctx, db, "")
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}
	for _, repo := range repos {
		if err := emit("repository", repo); err != nil {
			return err
		}
	}

	remotes, err := listRepoRemotes(ctx, db)
	if err != nil {
		return fmt.Errorf("list repo remotes: %w", err)
	}
	for _, remote := range remotes {
		if err := emit("repo_remote", remote); err != nil {
			return err
		}
	}

	worktrees, err := sqlite.ListWorktrees(ctx, db, "")
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	for _, worktree := range worktrees {
		if err := emit("worktree", worktree); err != nil {
			return err
		}
	}

	roots, err := sqlite.ListArtifactRoots(ctx, db, "")
	if err != nil {
		return fmt.Errorf("list artifact roots: %w", err)
	}
	for _, root := range roots {
		if err := emit("artifact_root", root); err != nil {
			return err
		}
	}

	artifacts, err := sqlite.ListArtifacts(ctx, db, "")
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}
	for _, artifact := range artifacts {
		if err := emit("artifact", artifact); err != nil {
			return err
		}
	}

	issues, err := sqlite.ListIssues(ctx, db, core.IssueListParams{})
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}
	for _, issue := range issues {
		issue.Dependencies = nil
		if err := emit("issue", issue); err != nil {
			return err
		}
	}

	dependencies, err := listDependencies(ctx, db)
	if err != nil {
		return fmt.Errorf("list dependencies: %w", err)
	}
	for _, dependency := range dependencies {
		if err := emit("dependency", dependency); err != nil {
			return err
		}
	}

	references, err := listReferences(ctx, db)
	if err != nil {
		return fmt.Errorf("list references: %w", err)
	}
	for _, reference := range references {
		if err := emit("reference", reference); err != nil {
			return err
		}
	}

	notes, err := listNotes(ctx, db)
	if err != nil {
		return fmt.Errorf("list notes: %w", err)
	}
	for _, note := range notes {
		if err := emit("note", note); err != nil {
			return err
		}
	}

	events, err := listEvents(ctx, db)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	for _, event := range events {
		if err := emit("event", event); err != nil {
			return err
		}
	}

	return nil
}

func listRepoRemotes(ctx context.Context, db *sql.DB) ([]core.RepoRemote, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, repository_id, remote_name, fetch_url, push_url, is_primary, created_at, updated_at
		 FROM repo_remotes
		 ORDER BY repository_id ASC, remote_name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query repo remotes: %w", err)
	}
	defer rows.Close()

	var remotes []core.RepoRemote
	for rows.Next() {
		var remote core.RepoRemote
		var isPrimary int
		if err := rows.Scan(
			&remote.ID,
			&remote.RepositoryID,
			&remote.RemoteName,
			&remote.FetchURL,
			&remote.PushURL,
			&isPrimary,
			&remote.CreatedAt,
			&remote.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repo remote: %w", err)
		}
		remote.IsPrimary = isPrimary != 0
		remotes = append(remotes, remote)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo remotes: %w", err)
	}
	if remotes == nil {
		remotes = []core.RepoRemote{}
	}
	return remotes, nil
}

func listDependencies(ctx context.Context, db *sql.DB) ([]Dependency, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT source.id, source.short_id, target.id, target.short_id, d.kind, d.created_at
		 FROM dependencies d
		 JOIN issues source ON source.id = d.issue_id
		 JOIN issues target ON target.id = d.depends_on_issue_id
		 ORDER BY d.created_at ASC, source.short_id ASC, target.short_id ASC, d.kind ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer rows.Close()

	var dependencies []Dependency
	for rows.Next() {
		var dependency Dependency
		if err := rows.Scan(
			&dependency.IssueID,
			&dependency.IssueShortID,
			&dependency.DependsOnID,
			&dependency.DependsOnShortID,
			&dependency.Kind,
			&dependency.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}
	if dependencies == nil {
		dependencies = []Dependency{}
	}
	return dependencies, nil
}

func listReferences(ctx context.Context, db *sql.DB) ([]Reference, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT i.id, i.short_id, a.id, a.relative_path, a.kind, ia.relation, ia.created_at
		 FROM issue_artifacts ia
		 JOIN issues i ON i.id = ia.issue_id
		 JOIN artifacts a ON a.id = ia.artifact_id
		 ORDER BY ia.created_at ASC, i.short_id ASC, a.relative_path ASC, ia.relation ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query references: %w", err)
	}
	defer rows.Close()

	var references []Reference
	for rows.Next() {
		var reference Reference
		if err := rows.Scan(
			&reference.IssueID,
			&reference.IssueShortID,
			&reference.ArtifactID,
			&reference.ArtifactPath,
			&reference.ArtifactKind,
			&reference.Relation,
			&reference.LinkCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reference: %w", err)
		}
		references = append(references, reference)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate references: %w", err)
	}
	if references == nil {
		references = []Reference{}
	}
	return references, nil
}

func listNotes(ctx context.Context, db *sql.DB) ([]core.Note, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, issue_id, author, body, created_at
		 FROM notes
		 ORDER BY created_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var notes []core.Note
	for rows.Next() {
		var note core.Note
		if err := rows.Scan(&note.ID, &note.IssueID, &note.Author, &note.Body, &note.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}
	if notes == nil {
		notes = []core.Note{}
	}
	return notes, nil
}

func listEvents(ctx context.Context, db *sql.DB) ([]core.Event, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, issue_id, actor, event_type, payload_json, created_at
		 FROM events
		 ORDER BY created_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []core.Event
	for rows.Next() {
		var event core.Event
		var issueID sql.NullString
		if err := rows.Scan(
			&event.ID,
			&issueID,
			&event.Actor,
			&event.EventType,
			&event.PayloadJSON,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if issueID.Valid {
			event.IssueID = issueID.String
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	if events == nil {
		events = []core.Event{}
	}
	return events, nil
}
