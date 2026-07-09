// Package store defines the persistence boundary used by the API layer.
package store

import (
	"context"
	"io"

	"github.com/abevz/af-coordinator/internal/core"
)

// CoordinatorStore is the API-facing persistence contract.
//
// SQLite is the only implementation today; this interface keeps transport
// handlers independent from SQLite package details without promising multiple
// database backends.
type CoordinatorStore interface {
	Ping(context.Context) error
	ExportJSONL(context.Context, io.Writer) error

	CreateProject(ctx context.Context, key, name, description string) (core.Project, error)
	GetProjectByKey(ctx context.Context, key string) (core.Project, error)
	ListProjects(context.Context) ([]core.Project, error)

	CreateRepo(ctx context.Context, projectKey string, req core.CreateRepoRequest) (core.Repository, []core.RepoRemote, error)
	GetRepo(ctx context.Context, idOrName string) (core.Repository, error)
	GetRepoInProject(ctx context.Context, projectID, idOrName string) (core.Repository, error)
	ListRepos(ctx context.Context, projectID string) ([]core.Repository, error)
	ListReposByProjectKey(ctx context.Context, projectKey string) ([]core.Repository, error)

	UpsertWorktree(ctx context.Context, repoID string, req core.CreateWorktreeRequest) (core.Worktree, bool, error)
	ListWorktrees(ctx context.Context, repoID string) ([]core.Worktree, error)
	DeleteWorktree(ctx context.Context, id string) (core.Worktree, error)

	CreateArtifactRoot(ctx context.Context, repoID string, req core.CreateArtifactRootRequest) (core.ArtifactRoot, error)
	ListArtifactRoots(ctx context.Context, repoID string) ([]core.ArtifactRoot, error)
	CreateArtifact(ctx context.Context, repoID string, req core.CreateArtifactRequest) (core.Artifact, error)
	ListArtifacts(ctx context.Context, repoID string) ([]core.Artifact, error)

	CreateIssue(ctx context.Context, projectKey string, req core.CreateIssueRequest) (core.Issue, error)
	ResolveIssueID(ctx context.Context, idOrShortID string) (string, error)
	GetIssue(ctx context.Context, id string) (core.Issue, *core.IssueLease, error)
	ListIssues(ctx context.Context, params core.IssueListParams) ([]core.Issue, error)
	ListReadyIssues(ctx context.Context, projectID, repoID string) ([]core.Issue, error)
	ClaimIssue(ctx context.Context, issueID, holder string, ttlSeconds int) (core.ClaimResponse, error)
	HeartbeatLease(ctx context.Context, issueID, leaseToken string, ttlSeconds int) (string, error)
	ReleaseLease(ctx context.Context, issueID, leaseToken string) error
	UpdateIssue(ctx context.Context, issueID string, req core.UpdateIssueRequest) (core.Issue, error)
	CloseIssue(ctx context.Context, issueID string, req core.CloseIssueRequest) (core.CloseIssueResult, error)
	AddDependency(ctx context.Context, issueID string, req core.AddDependencyRequest) error
	RemoveDependency(ctx context.Context, issueID, dependsOn, kind, actor string) error
	LinkArtifact(ctx context.Context, issueID string, req core.LinkArtifactRequest) (string, error)
	UnlinkArtifact(ctx context.Context, issueID, artifact, relation, actor string) error
	ListIssueArtifacts(ctx context.Context, issueID string) ([]core.ArtifactRef, error)
	CreateNote(ctx context.Context, issueID string, req core.CreateNoteRequest) (core.Note, error)
	ListNotes(ctx context.Context, issueID string) ([]core.Note, error)
	ListEvents(ctx context.Context, issueID string) ([]core.Event, error)
	ListGlobalEvents(ctx context.Context, since string, limit int) (core.EventPage, error)
}
