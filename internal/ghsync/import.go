// Package ghsync implements the CLI and MCP GitHub import and publish flow.
// It only calls the local daemon client and the caller's GitHub CLI client.
package ghsync

import (
	"context"
	"errors"
	"fmt"

	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/github"
	"github.com/google/uuid"
)

// Coordinator is the daemon API surface needed by GitHub synchronization.
type Coordinator interface {
	ListIssuesWithFilters(context.Context, core.IssueListParams) ([]core.Issue, error)
	CreateIssue(context.Context, core.CreateIssueRequest) (core.Issue, error)
	GetIssue(context.Context, string) (core.Issue, *core.IssueLease, error)
	ListEvents(context.Context, string) ([]core.Event, error)
	ListNotes(context.Context, string) ([]core.Note, error)
}

type ImportRequest struct {
	Source                                 github.IssueRef
	ProjectID, ProjectKey, Repo, ScopeKind string
	IssueType, AcceptanceCriteria          string
	Priority                               int
	Tags                                   []string
	AllowClosed                            bool
	Actor                                  string
	// ResolveActor is called only when a new issue must be created. The CLI
	// retains its process-tree fallback without changing repeated imports.
	ResolveActor func() (string, error)
}

type ImportResult struct {
	Issue     core.Issue `json:"issue"`
	Imported  bool       `json:"imported"`
	SourceURL string     `json:"source_url"`
}

func ImportOperationID(projectID, externalKey string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("dibs:issue-import:"+projectID+":"+externalKey)).String()
}

func Import(ctx context.Context, c Coordinator, gh github.Client, req ImportRequest) (ImportResult, error) {
	ref := req.Source
	key := ref.ExternalKey()
	sourceURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", ref.Owner, ref.Repo, ref.Number)
	lookup := func() (core.Issue, bool, error) {
		issues, err := c.ListIssuesWithFilters(ctx, core.IssueListParams{Project: req.ProjectKey, ExternalKey: key})
		if err != nil {
			return core.Issue{}, false, err
		}
		if len(issues) == 0 {
			return core.Issue{}, false, nil
		}
		oldest := issues[0]
		for _, issue := range issues[1:] {
			if issue.CreatedAt < oldest.CreatedAt || (issue.CreatedAt == oldest.CreatedAt && issue.ID < oldest.ID) {
				oldest = issue
			}
		}
		return oldest, true, nil
	}
	if issue, ok, err := lookup(); err != nil {
		return ImportResult{}, err
	} else if ok {
		return ImportResult{Issue: issue, SourceURL: sourceURL}, nil
	}
	source, err := gh.GetIssue(ctx, ref)
	if err != nil {
		return ImportResult{}, err
	}
	if source.IsPullRequest() {
		return ImportResult{}, fmt.Errorf("GitHub pull requests cannot be imported; use an issue URL")
	}
	if source.State == "closed" && !req.AllowClosed {
		return ImportResult{}, fmt.Errorf("GitHub issue is closed; pass --allow-closed to import it")
	}
	actor := req.Actor
	if actor == "" && req.ResolveActor != nil {
		actor, err = req.ResolveActor()
		if err != nil {
			return ImportResult{}, err
		}
	}
	if actor == "" {
		return ImportResult{}, fmt.Errorf("actor is required: set actor or DIBS_ACTOR")
	}
	create := core.CreateIssueRequest{
		Project: req.ProjectKey, ScopeKind: req.ScopeKind, Repo: req.Repo,
		IssueType: req.IssueType, Priority: req.Priority, Tags: req.Tags,
		Title: source.Title, ExternalKey: key,
		Description:        "Source: " + source.HTMLURL + "\n\n" + source.Body,
		AcceptanceCriteria: req.AcceptanceCriteria, Actor: actor,
		OperationID: ImportOperationID(req.ProjectID, key),
	}
	issue, err := c.CreateIssue(ctx, create)
	if err != nil {
		var clientErr *client.ClientError
		if errors.As(err, &clientErr) && (clientErr.Code == "idempotency_conflict" || clientErr.Code == "conflict") {
			if existing, ok, lookupErr := lookup(); lookupErr == nil && ok {
				return ImportResult{Issue: existing, SourceURL: sourceURL}, nil
			}
		}
		return ImportResult{}, err
	}
	return ImportResult{Issue: issue, Imported: true, SourceURL: source.HTMLURL}, nil
}
