package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/github"
)

type fakeGitHub struct {
	issue       github.Issue
	getErr      error
	comments    []github.Comment
	createCalls int
	commentsURL string
}

func (f *fakeGitHub) GetIssue(context.Context, github.IssueRef) (github.Issue, error) {
	return f.issue, f.getErr
}
func (f *fakeGitHub) ListComments(_ context.Context, commentsURL string) ([]github.Comment, error) {
	f.commentsURL = commentsURL
	return append([]github.Comment(nil), f.comments...), nil
}
func (f *fakeGitHub) CreateComment(_ context.Context, commentsURL, body string) (github.Comment, error) {
	f.commentsURL = commentsURL
	f.createCalls++
	comment := github.Comment{Body: body, HTMLURL: "https://github.com/new/repo/issues/7#issuecomment-1"}
	f.comments = append(f.comments, comment)
	return comment, nil
}

func githubContent(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	if result["isError"] == true {
		t.Fatalf("unexpected MCP error: %v", result)
	}
	content, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("no structured content: %v", result)
	}
	return content
}

func TestMCPGitHubRoundTripAndCloseReplay(t *testing.T) {
	t.Setenv("DIBS_LEASE_TOKEN", "")
	t.Setenv("DIBS_OPERATOR_TOKEN", "")
	t.Setenv("AF_OPERATOR_TOKEN", "")
	ctx := context.Background()
	c := startMCPTestDaemon(t)
	if _, err := c.CreateProject(ctx, "afc", "Test project", ""); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGitHub{issue: github.Issue{
		Title: "Fix login", Body: "source body", State: "open",
		HTMLURL:     "https://github.com/o/r/issues/7",
		CommentsURL: "https://api.github.com/repos/new/repo/issues/7/comments",
	}}
	s := NewServerWithGitHub(c, gh, "tester", "test")
	missingProject := mcpWireCall(t, s, "import_issue", map[string]any{"source": "o/r#7"})
	if missingProject["isError"] != true || missingProject["structuredContent"].(map[string]any)["code"] != "validation_failed" {
		t.Fatalf("missing project = %v", missingProject)
	}
	args := map[string]any{"source": "o/r#7", "project": "afc", "issue_type": "task", "acceptance_criteria": "checks pass"}
	first := githubContent(t, mcpWireCall(t, s, "import_issue", args))
	second := githubContent(t, mcpWireCall(t, s, "import_issue", args))
	if first["imported"] != true || second["imported"] != false {
		t.Fatalf("import results: %v / %v", first, second)
	}
	issue := first["issue"].(map[string]any)
	shortID := issue["short_id"].(string)
	if second["issue"].(map[string]any)["short_id"] != shortID {
		t.Fatalf("repeat imported new issue: %v", second)
	}
	claim := githubContent(t, mcpWireCall(t, s, "claim_issue", map[string]any{"issue_id": shortID, "holder": "tester"}))
	closeArgs := map[string]any{
		"issue_id": shortID, "resolution": "done", "expected_version": claim["version"],
		"lease_token": claim["lease_token"], "lease_generation": claim["lease_generation"],
		"note": "fixed", "pr_url": "https://github.com/new/repo/pull/3",
		"branch": "fix/login", "publish": true, "operation_id": "mcp-github-close-0001",
	}
	closed := githubContent(t, mcpWireCall(t, s, "close_issue", closeArgs))
	publication := closed["publish"].(map[string]any)
	if closed["status"] != "closed" || publication["ok"] != true || publication["already"] != false || gh.createCalls != 1 {
		t.Fatalf("close publish = %v, comments=%d", closed, gh.createCalls)
	}
	if gh.commentsURL != gh.issue.CommentsURL || !strings.Contains(gh.comments[0].Body, "https://github.com/new/repo/pull/3") || !strings.Contains(gh.comments[0].Body, "> fixed") {
		t.Fatalf("wrong comment target/body: url=%q comments=%v", gh.commentsURL, gh.comments)
	}
	replayed := githubContent(t, mcpWireCall(t, s, "close_issue", closeArgs))
	if replayed["publish"].(map[string]any)["already"] != true || gh.createCalls != 1 {
		t.Fatalf("replay publish = %v", replayed)
	}
	explicit := githubContent(t, mcpWireCall(t, s, "publish_issue", map[string]any{"issue_id": shortID}))
	if explicit["issue"] != shortID || explicit["already"] != true || gh.createCalls != 1 {
		t.Fatalf("explicit publish = %v", explicit)
	}
	gh.issue.Locked = true
	locked := mcpWireCall(t, s, "publish_issue", map[string]any{"issue_id": shortID})
	if locked["isError"] != true || locked["structuredContent"].(map[string]any)["error"].(map[string]any)["code"] != "locked" || gh.createCalls != 1 {
		t.Fatalf("locked publish = %v", locked)
	}
}

func TestMCPImportGitHubErrorCodes(t *testing.T) {
	ctx := context.Background()
	c := startMCPTestDaemon(t)
	if _, err := c.CreateProject(ctx, "afc", "Test project", ""); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGitHub{}
	s := NewServerWithGitHub(c, gh, "tester", "test")
	for _, code := range []string{"gh_missing", "gh_auth", "not_found", "timeout", "rate_limited", "github"} {
		t.Run(code, func(t *testing.T) {
			gh.getErr = &github.Error{Code: code, Remedy: "remedy", Stderr: "gh: concise failure"}
			result := mcpWireCall(t, s, "import_issue", map[string]any{"source": "o/r#7", "project": "afc"})
			payload, ok := result["structuredContent"].(map[string]any)
			if !ok || result["isError"] != true || payload["code"] != code || !strings.Contains(payload["message"].(string), "gh: concise failure") {
				t.Fatalf("error result = %v", result)
			}
		})
	}
}

func TestMCPClosePublishRequiresGitHubKeyBeforeClose(t *testing.T) {
	ctx := context.Background()
	c := startMCPTestDaemon(t)
	if _, err := c.CreateProject(ctx, "afc", "Test project", ""); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGitHub{}
	s := NewServerWithGitHub(c, gh, "tester", "test")
	created := githubContent(t, mcpWireCall(t, s, "create_issue", map[string]any{
		"project": "afc", "scope_kind": "project", "title": "Local only", "operation_id": "mcp-local-create-0001",
	}))
	id := created["issue"].(map[string]any)["short_id"].(string)
	claim := githubContent(t, mcpWireCall(t, s, "claim_issue", map[string]any{"issue_id": id, "holder": "tester"}))
	result := mcpWireCall(t, s, "close_issue", map[string]any{
		"issue_id": id, "resolution": "done", "expected_version": claim["version"],
		"lease_token": claim["lease_token"], "lease_generation": claim["lease_generation"],
		"publish": true, "operation_id": "mcp-local-close-0001",
	})
	if result["isError"] != true || !strings.Contains(result["structuredContent"].(map[string]any)["message"].(string), "no GitHub external key") {
		t.Fatalf("missing-key preflight = %v", result)
	}
	issue, _, err := c.GetIssue(ctx, id)
	if err != nil || issue.Status != "in_progress" || gh.createCalls != 0 {
		t.Fatalf("issue was closed: %+v, err=%v", issue, err)
	}
}

func TestMCPCloseReplayDoesNotPublishNewerClose(t *testing.T) {
	t.Setenv("DIBS_LEASE_TOKEN", "")
	t.Setenv("DIBS_OPERATOR_TOKEN", "scratch-operator-token")
	t.Setenv("AF_OPERATOR_TOKEN", "")
	ctx := context.Background()
	c := startMCPTestDaemon(t)
	c.SetOperatorToken("scratch-operator-token")
	if _, err := c.CreateProject(ctx, "afc", "Test project", ""); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGitHub{issue: github.Issue{Title: "Task", State: "open", HTMLURL: "https://github.com/o/r/issues/7", CommentsURL: "https://api.github.com/repos/o/r/issues/7/comments"}}
	s := NewServerWithGitHub(c, gh, "tester", "test")
	imported := githubContent(t, mcpWireCall(t, s, "import_issue", map[string]any{"source": "o/r#7", "project": "afc"}))
	id := imported["issue"].(map[string]any)["short_id"].(string)
	claim := githubContent(t, mcpWireCall(t, s, "claim_issue", map[string]any{"issue_id": id, "holder": "tester"}))
	firstArgs := map[string]any{
		"issue_id": id, "resolution": "done", "expected_version": claim["version"],
		"lease_token": claim["lease_token"], "lease_generation": claim["lease_generation"],
		"operation_id": "mcp-old-close-0001",
	}
	first := githubContent(t, mcpWireCall(t, s, "close_issue", firstArgs))
	closedAt, err := time.Parse(time.RFC3339, first["closed_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if wait := time.Until(closedAt.Add(time.Second)) + 20*time.Millisecond; wait > 0 {
		time.Sleep(wait)
	}
	current, _, err := c.GetIssue(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.OperatorReopenIssue(ctx, id, core.OperatorReopenIssueRequest{ExpectedVersion: current.Version, Actor: "operator", Reason: "more work"}); err != nil {
		t.Fatal(err)
	}
	secondClaim := githubContent(t, mcpWireCall(t, s, "claim_issue", map[string]any{"issue_id": id, "holder": "tester"}))
	second := githubContent(t, mcpWireCall(t, s, "close_issue", map[string]any{
		"issue_id": id, "resolution": "done", "expected_version": secondClaim["version"],
		"lease_token": secondClaim["lease_token"], "lease_generation": secondClaim["lease_generation"],
		"operation_id": "mcp-new-close-0001",
	}))
	if second["closed_at"] == first["closed_at"] {
		t.Fatalf("test requires distinct close timestamps: %v / %v", first, second)
	}
	firstArgs["publish"] = true
	replayed := githubContent(t, mcpWireCall(t, s, "close_issue", firstArgs))
	publication := replayed["publish"].(map[string]any)
	if replayed["closed_at"] != first["closed_at"] || publication["error"].(map[string]any)["code"] != "stale_close" || gh.createCalls != 0 {
		t.Fatalf("stale replay published a newer close: %v, calls=%d", replayed, gh.createCalls)
	}
	newPublish := githubContent(t, mcpWireCall(t, s, "publish_issue", map[string]any{"issue_id": id}))
	if newPublish["ok"] != true || gh.createCalls != 1 {
		t.Fatalf("new close publish = %v, calls=%d", newPublish, gh.createCalls)
	}
}

func TestMCPClosePublishFailureAndSecretCode(t *testing.T) {
	t.Setenv("DIBS_LEASE_TOKEN", "")
	t.Setenv("DIBS_OPERATOR_TOKEN", "")
	t.Setenv("AF_OPERATOR_TOKEN", "")
	ctx := context.Background()
	c := startMCPTestDaemon(t)
	if _, err := c.CreateProject(ctx, "afc", "Test project", ""); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGitHub{issue: github.Issue{Title: "Task", State: "open", HTMLURL: "https://github.com/o/r/issues/7", CommentsURL: "https://api.github.com/repos/o/r/issues/7/comments"}}
	s := NewServerWithGitHub(c, gh, "tester", "test")
	imported := githubContent(t, mcpWireCall(t, s, "import_issue", map[string]any{"source": "o/r#7", "project": "afc"}))
	id := imported["issue"].(map[string]any)["short_id"].(string)
	claim := githubContent(t, mcpWireCall(t, s, "claim_issue", map[string]any{"issue_id": id, "holder": "tester"}))
	gh.getErr = &github.Error{Code: "not_found", Remedy: "check access", Stderr: "gh: Not Found (HTTP 404)"}
	closeArgs := map[string]any{"issue_id": id, "resolution": "cancelled", "expected_version": claim["version"], "lease_token": claim["lease_token"], "lease_generation": claim["lease_generation"], "publish": true, "operation_id": "mcp-publish-failure-0001"}
	closed := githubContent(t, mcpWireCall(t, s, "close_issue", closeArgs))
	publication := closed["publish"].(map[string]any)
	if closed["status"] != "closed" || publication["ok"] != false || publication["error"].(map[string]any)["code"] != "not_found" {
		t.Fatalf("close failure = %v", closed)
	}
	if !strings.Contains(publication["error"].(map[string]any)["message"].(string), "Not Found") {
		t.Fatalf("gh stderr lost: %v", publication)
	}
	if gh.createCalls != 0 {
		t.Fatal("posted after GitHub failure")
	}
	gh.getErr = nil
	// Replaying the close still attempts publication; the original operation
	// result is returned by the daemon, and the new publish succeeds.
	replayed := githubContent(t, mcpWireCall(t, s, "close_issue", closeArgs))
	if replayed["publish"].(map[string]any)["ok"] != true || gh.createCalls != 1 {
		t.Fatalf("replay after failure = %v", replayed)
	}
	gh.issue.HTMLURL = "https://github.com/o/r/issues/8"
	secretIssue := githubContent(t, mcpWireCall(t, s, "import_issue", map[string]any{"source": "o/r#8", "project": "afc"}))
	secretID := secretIssue["issue"].(map[string]any)["short_id"].(string)
	secretClaim := githubContent(t, mcpWireCall(t, s, "claim_issue", map[string]any{"issue_id": secretID, "holder": "tester"}))
	secretClose := githubContent(t, mcpWireCall(t, s, "close_issue", map[string]any{
		"issue_id": secretID, "resolution": "done", "expected_version": secretClaim["version"],
		"lease_token": secretClaim["lease_token"], "lease_generation": secretClaim["lease_generation"],
		"note": "contains " + secretClaim["lease_token"].(string), "publish": true,
	}))
	if secretClose["status"] != "closed" || secretClose["publish"].(map[string]any)["error"].(map[string]any)["code"] != "secret_in_text" || gh.createCalls != 1 {
		t.Fatalf("secret close = %v, comments=%d", secretClose, gh.createCalls)
	}
	t.Setenv("DIBS_LEASE_TOKEN", secretClaim["lease_token"].(string))
	bad := mcpWireCall(t, s, "publish_issue", map[string]any{"issue_id": secretID})
	if bad["isError"] != true || bad["structuredContent"].(map[string]any)["error"].(map[string]any)["code"] != "secret_in_text" {
		t.Fatalf("secret publish error = %v", bad)
	}
}
