package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/ghsync"
	"github.com/abevz/dibs/internal/github"
)

type githubToolError struct {
	Code, Message string
}

func (e *githubToolError) Error() string { return e.Code + ": " + e.Message }

func githubToolFailure(err error) error {
	code, message := "internal_error", err.Error()
	var ghErr *github.Error
	var syncErr *ghsync.PublishError
	var clientErr *client.ClientError
	switch {
	case errors.As(err, &ghErr):
		code, message = ghErr.Code, ghErr.Message()
	case errors.As(err, &syncErr):
		code, message = syncErr.Code, syncErr.Message
	case errors.As(err, &clientErr):
		code, message = clientErr.Code, clientErr.Message
	}
	return &githubToolError{Code: code, Message: message}
}

func githubValidation(message string) error {
	return &githubToolError{Code: "validation_failed", Message: message}
}

func (s *Server) importIssue(ctx context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		Source             string   `json:"source"`
		Project            string   `json:"project"`
		Repo               string   `json:"repo"`
		ScopeKind          string   `json:"scope_kind"`
		IssueType          string   `json:"issue_type"`
		Priority           int      `json:"priority"`
		AcceptanceCriteria string   `json:"acceptance_criteria"`
		Tags               []string `json:"tags"`
		AllowClosed        bool     `json:"allow_closed"`
		Actor              string   `json:"actor"`
	}
	if err := unmarshalArgs(raw, &args); err != nil {
		return nil, githubValidation(err.Error())
	}
	if args.Source == "" {
		return nil, githubValidation("source is required")
	}
	if args.Project == "" {
		return nil, githubValidation("project is required for MCP import_issue")
	}
	ref, err := github.ParseIssueRef(args.Source)
	if err != nil {
		return nil, githubValidation(err.Error())
	}
	if args.Priority < 0 {
		return nil, githubValidation("priority requires a non-negative integer")
	}
	if args.ScopeKind != "" && args.ScopeKind != "project" && args.ScopeKind != "repository" {
		return nil, githubValidation("scope_kind must be project or repository")
	}
	projects, err := s.client.ListProjects(ctx)
	if err != nil {
		return nil, githubToolFailure(err)
	}
	var project core.Project
	for _, candidate := range projects {
		if candidate.Key == args.Project || candidate.ID == args.Project {
			project = candidate
			break
		}
	}
	if project.ID == "" {
		return nil, githubToolFailure(fmt.Errorf("project %q is not registered", args.Project))
	}
	scope, repoName := "project", ""
	if args.Repo == "" {
		if args.ScopeKind == "repository" {
			return nil, githubValidation("repo is required for repository scope")
		}
	} else {
		if args.ScopeKind == "project" {
			return nil, githubValidation("repo cannot be combined with project scope")
		}
		scope = "repository"
		repos, err := s.client.ListRepos(ctx, "")
		if err != nil {
			return nil, githubToolFailure(err)
		}
		for _, candidate := range repos {
			if candidate.ProjectID == project.ID && (candidate.LogicalName == args.Repo || candidate.ID == args.Repo) {
				repoName = candidate.LogicalName
				break
			}
		}
		if repoName == "" {
			return nil, githubToolFailure(fmt.Errorf("repository %q is not registered in project %s", args.Repo, project.Key))
		}
	}
	result, err := ghsync.Import(ctx, s.client, s.github, ghsync.ImportRequest{
		Source: ref, ProjectID: project.ID, ProjectKey: project.Key,
		Repo: repoName, ScopeKind: scope, IssueType: args.IssueType,
		Priority: args.Priority, AcceptanceCriteria: args.AcceptanceCriteria,
		Tags: args.Tags, AllowClosed: args.AllowClosed,
		ResolveActor: func() (string, error) { return s.resolveActor(args.Actor, "") },
	})
	if err != nil {
		return nil, githubToolFailure(err)
	}
	return result, nil
}

type publishOutcomeError struct {
	Issue  string
	Result ghsync.PublishResult
}

func (e *publishOutcomeError) Error() string { return e.Result.Error.Message }

func (s *Server) publishIssue(ctx context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		IssueID string `json:"issue_id"`
	}
	if err := unmarshalArgs(raw, &args); err != nil {
		return nil, githubValidation(err.Error())
	}
	if args.IssueID == "" {
		return nil, githubValidation("issue_id is required")
	}
	issue, _, err := s.client.GetIssue(ctx, args.IssueID)
	shortID := args.IssueID
	if issue.ShortID != "" {
		shortID = issue.ShortID
	}
	if err != nil {
		return nil, &publishOutcomeError{Issue: shortID, Result: ghsync.Failure(err)}
	}
	result, err := ghsync.Publish(ctx, s.client, s.github, args.IssueID)
	if err != nil {
		return nil, &publishOutcomeError{Issue: shortID, Result: ghsync.Failure(err)}
	}
	return struct {
		Issue string `json:"issue"`
		ghsync.PublishResult
	}{shortID, result}, nil
}
