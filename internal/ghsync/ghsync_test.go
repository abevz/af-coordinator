package ghsync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/github"
)

type testCoordinator struct {
	issues                             []core.Issue
	createReq                          core.CreateIssueRequest
	created                            core.Issue
	events                             []core.Event
	notes                              []core.Note
	get                                core.Issue
	listCalls, createCalls, eventCalls int
}

func (f *testCoordinator) ListIssuesWithFilters(context.Context, core.IssueListParams) ([]core.Issue, error) {
	f.listCalls++
	return f.issues, nil
}
func (f *testCoordinator) CreateIssue(_ context.Context, req core.CreateIssueRequest) (core.Issue, error) {
	f.createCalls++
	f.createReq = req
	return f.created, nil
}
func (f *testCoordinator) GetIssue(context.Context, string) (core.Issue, *core.IssueLease, error) {
	return f.get, nil, nil
}
func (f *testCoordinator) ListEvents(context.Context, string) ([]core.Event, error) {
	f.eventCalls++
	return f.events, nil
}
func (f *testCoordinator) ListNotes(context.Context, string) ([]core.Note, error) {
	return f.notes, nil
}

type testGitHub struct {
	source                           github.Issue
	comments                         []github.Comment
	getCalls, listCalls, createCalls int
	url                              string
	lastBody                         string
}

func (f *testGitHub) GetIssue(context.Context, github.IssueRef) (github.Issue, error) {
	f.getCalls++
	return f.source, nil
}
func (f *testGitHub) ListComments(_ context.Context, url string) ([]github.Comment, error) {
	f.listCalls++
	f.url = url
	return f.comments, nil
}
func (f *testGitHub) CreateComment(_ context.Context, url, body string) (github.Comment, error) {
	f.createCalls++
	f.url = url
	f.lastBody = body
	return github.Comment{Body: body, HTMLURL: "https://github.com/o/r/issues/1#issuecomment-1"}, nil
}

func TestImportLookupAndMapping(t *testing.T) {
	ref, err := github.ParseIssueRef("o/r#1")
	if err != nil {
		t.Fatal(err)
	}
	old := core.Issue{ID: "older", CreatedAt: "2026-09-25T12:00:00Z"}
	newer := core.Issue{ID: "newer", CreatedAt: "2026-09-26T12:00:00Z"}
	c := &testCoordinator{issues: []core.Issue{newer, old}, created: core.Issue{ID: "created"}}
	gh := &testGitHub{source: github.Issue{Title: "Title", Body: "Body", State: "open", HTMLURL: "https://github.com/o/r/issues/1"}}
	actorCalls := 0
	req := ImportRequest{Source: ref, ProjectID: "project-id", ProjectKey: "app", ScopeKind: "project", IssueType: "task", AcceptanceCriteria: "pass", Tags: []string{"area/test"}, ResolveActor: func() (string, error) { actorCalls++; return "agent", nil }}
	result, err := Import(t.Context(), c, gh, req)
	if err != nil || result.Imported || result.Issue.ID != "older" || gh.getCalls != 0 || actorCalls != 0 {
		t.Fatalf("existing import = %+v, %v", result, err)
	}
	c.issues = nil
	result, err = Import(t.Context(), c, gh, req)
	if err != nil || !result.Imported || c.createCalls != 1 || actorCalls != 1 {
		t.Fatalf("new import = %+v, %v", result, err)
	}
	if c.createReq.Description != "Source: https://github.com/o/r/issues/1\n\nBody" || c.createReq.OperationID != ImportOperationID("project-id", "github:o/r#1") || c.createReq.Actor != "agent" || c.createReq.AcceptanceCriteria != "pass" {
		t.Fatalf("create mapping = %+v", c.createReq)
	}
}

func TestPublishMarkerAndSecretGuard(t *testing.T) {
	t.Setenv("DIBS_LEASE_TOKEN", "")
	t.Setenv("DIBS_OPERATOR_TOKEN", "")
	t.Setenv("AF_OPERATOR_TOKEN", "")
	issue := core.Issue{ID: "issue-uuid", ShortID: "app-1", Status: "cancelled", ExternalKey: "github:o/r#1"}
	const when = "2026-09-26T17:00:00Z"
	c := &testCoordinator{get: issue, events: []core.Event{
		{ID: "note-id", Sequence: 2, EventType: "note_added", Actor: "agent", CreatedAt: when},
		{ID: "close-id", Sequence: 3, EventType: "issue_closed", Actor: "agent", CreatedAt: when, PayloadJSON: `{"resolution":"cancelled","branch":"fix/one"}`},
	}, notes: []core.Note{{Author: "agent", CreatedAt: when, Body: "done"}}}
	gh := &testGitHub{source: github.Issue{CommentsURL: "https://api.github.com/repos/moved/repo/issues/1/comments"}}
	result, err := Publish(t.Context(), c, gh, issue.ShortID)
	if err != nil || !result.OK || gh.createCalls != 1 || gh.url != gh.source.CommentsURL {
		t.Fatalf("publish = %+v, %v, gh=%+v", result, err, gh)
	}
	if !strings.Contains(gh.lastBody, "closed as **cancelled**") || !strings.Contains(gh.lastBody, "> done") || !strings.Contains(gh.lastBody, "close_event=close-id") {
		t.Fatalf("comment body = %q", gh.lastBody)
	}
	gh.comments = []github.Comment{{Body: "<!-- dibs:publish issue=issue-uuid close_event=close-id -->", HTMLURL: "https://github.com/o/r/issues/1#issuecomment-1"}}
	result, err = Publish(t.Context(), c, gh, issue.ShortID)
	if err != nil || !result.Already || gh.createCalls != 1 {
		t.Fatalf("repeat = %+v, %v", result, err)
	}
	c.notes[0].Body = "contains exact-secret"
	t.Setenv("DIBS_OPERATOR_TOKEN", "exact-secret")
	_, err = Publish(t.Context(), c, gh, issue.ShortID)
	var secret *PublishError
	if !errors.As(err, &secret) || secret.Code != "secret_in_text" || strings.Contains(err.Error(), "exact-secret") || gh.createCalls != 1 {
		t.Fatalf("secret = %v, calls=%d", err, gh.createCalls)
	}
}

func TestPublishCloseResultStaleReplay(t *testing.T) {
	issue := core.Issue{ID: "issue-uuid", ShortID: "app-1", Status: "done", ExternalKey: "github:o/r#1"}
	oldWhen, newWhen := "2026-09-26T17:00:00Z", "2026-09-26T17:00:01Z"
	c := &testCoordinator{get: issue, events: []core.Event{
		{ID: "old-close", Sequence: 2, EventType: "issue_closed", CreatedAt: oldWhen, PayloadJSON: `{"resolution":"done","lease_generation":1}`},
		{ID: "new-close", Sequence: 5, EventType: "issue_closed", CreatedAt: newWhen, PayloadJSON: `{"resolution":"done","lease_generation":2}`},
	}}
	gh := &testGitHub{source: github.Issue{CommentsURL: "https://api.github.com/repos/o/r/issues/1/comments"}}
	oldResult := core.CloseIssueResult{Status: "closed", Resolution: "done", ClosedAt: oldWhen}
	result, err := PublishCloseResult(t.Context(), c, gh, issue.ShortID, oldResult, 1, "")
	var publishErr *PublishError
	if !errors.As(err, &publishErr) || publishErr.Code != "stale_close" || result.OK || gh.createCalls != 0 {
		t.Fatalf("unpublished stale close = %+v, %v, calls=%d", result, err, gh.createCalls)
	}
	gh.comments = []github.Comment{{Body: "<!-- dibs:publish issue=issue-uuid close_event=old-close -->", HTMLURL: "https://github.com/o/r/issues/1#issuecomment-1"}}
	result, err = PublishCloseResult(t.Context(), c, gh, issue.ShortID, oldResult, 1, "")
	if err != nil || !result.OK || !result.Already || result.CommentURL != gh.comments[0].HTMLURL || gh.createCalls != 0 {
		t.Fatalf("published stale close = %+v, %v, calls=%d", result, err, gh.createCalls)
	}
	// Lease generation also disambiguates two closes within one timestamp second.
	c.events[1].CreatedAt = oldWhen
	gh.comments = nil
	_, err = PublishCloseResult(t.Context(), c, gh, issue.ShortID, oldResult, 1, "")
	if !errors.As(err, &publishErr) || publishErr.Code != "stale_close" || gh.createCalls != 0 {
		t.Fatalf("same-second newer close = %v, calls=%d", err, gh.createCalls)
	}
	c.events[1] = core.Event{ID: "operator-close", Sequence: 5, EventType: "issue_operator_closed", CreatedAt: newWhen, PayloadJSON: `{"resolution":"cancelled"}`}
	gh.comments = []github.Comment{{Body: "<!-- dibs:publish issue=issue-uuid close_event=old-close -->", HTMLURL: "https://github.com/o/r/issues/1#issuecomment-1"}}
	result, err = PublishCloseResult(t.Context(), c, gh, issue.ShortID, oldResult, 1, "")
	if err != nil || !result.Already || gh.createCalls != 0 {
		t.Fatalf("operator close after published close = %+v, %v", result, err)
	}
	gh.comments = nil
	_, err = PublishCloseResult(t.Context(), c, gh, issue.ShortID, oldResult, 1, "")
	if !errors.As(err, &publishErr) || publishErr.Code != "stale_close" || gh.createCalls != 0 {
		t.Fatalf("operator close after unpublished close = %v, calls=%d", err, gh.createCalls)
	}
}

func TestPublishCloseResultPinsSelectedEvent(t *testing.T) {
	issue := core.Issue{ID: "issue-uuid", ShortID: "app-1", Status: "done", ExternalKey: "github:o/r#1"}
	when := "2026-09-26T17:00:00Z"
	c := &testCoordinator{get: issue, events: []core.Event{{ID: "close-one", Sequence: 2, EventType: "issue_closed", CreatedAt: when, PayloadJSON: `{"resolution":"done","lease_generation":1}`}}}
	gh := &testGitHub{source: github.Issue{CommentsURL: "https://api.github.com/repos/o/r/issues/1/comments"}}
	result, err := PublishCloseResult(t.Context(), c, gh, issue.ShortID, core.CloseIssueResult{Status: "closed", Resolution: "done", ClosedAt: when}, 1, "")
	if err != nil || !result.OK || c.eventCalls != 1 || gh.createCalls != 1 || !strings.Contains(gh.lastBody, "close_event=close-one") {
		t.Fatalf("selected close = %+v, %v, event calls=%d, gh=%+v", result, err, c.eventCalls, gh)
	}
}
