package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/abevz/af-coordinator/internal/core"
)

type fakeClient struct {
	healthResp     core.Health
	getIssueResp   core.Issue
	getLeaseResp   *core.IssueLease
	readyResp      []core.Issue
	claimResp      core.ClaimResponse
	heartbeatResp  string
	noteResp       core.Note
	notesResp      []core.Note
	eventsResp     []core.Event
	closeResp      core.CloseIssueResult
	lastIssueID    string
	lastHolder     string
	lastTTL        int
	lastLeaseToken string
	lastNoteAuthor string
	lastNoteBody   string
	lastCloseReq   core.CloseIssueRequest
}

func (f *fakeClient) Health(context.Context) (core.Health, error) { return f.healthResp, nil }
func (f *fakeClient) GetIssue(context.Context, string) (core.Issue, *core.IssueLease, error) {
	return f.getIssueResp, f.getLeaseResp, nil
}
func (f *fakeClient) ListReadyIssues(context.Context, string, string) ([]core.Issue, error) {
	return f.readyResp, nil
}
func (f *fakeClient) ClaimIssue(_ context.Context, issueID, holder string, ttlSeconds int) (core.ClaimResponse, error) {
	f.lastIssueID = issueID
	f.lastHolder = holder
	f.lastTTL = ttlSeconds
	return f.claimResp, nil
}
func (f *fakeClient) HeartbeatLease(_ context.Context, issueID, leaseToken string, ttlSeconds int) (string, error) {
	f.lastIssueID = issueID
	f.lastLeaseToken = leaseToken
	f.lastTTL = ttlSeconds
	return f.heartbeatResp, nil
}
func (f *fakeClient) CreateNote(_ context.Context, issueID, author, body string) (core.Note, error) {
	f.lastIssueID = issueID
	f.lastNoteAuthor = author
	f.lastNoteBody = body
	return f.noteResp, nil
}
func (f *fakeClient) ListNotes(context.Context, string) ([]core.Note, error) { return f.notesResp, nil }
func (f *fakeClient) ListEvents(context.Context, string) ([]core.Event, error) {
	return f.eventsResp, nil
}
func (f *fakeClient) CloseIssue(_ context.Context, issueID string, req core.CloseIssueRequest) (core.CloseIssueResult, error) {
	f.lastIssueID = issueID
	f.lastCloseReq = req
	return f.closeResp, nil
}

func TestHandleInitialize(t *testing.T) {
	s := NewServer(&fakeClient{}, "tester", "0055")
	resp := s.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26"}`),
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected initialize result, got %+v", resp)
	}
	result := resp.Result.(map[string]any)
	if result["protocolVersion"] != "2025-03-26" {
		t.Fatalf("protocolVersion = %v", result["protocolVersion"])
	}
}

func TestToolsListIncludesCoordinatorTools(t *testing.T) {
	s := NewServer(&fakeClient{}, "tester", "0055")
	resp := s.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("2"),
		Method:  "tools/list",
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected tools/list result, got %+v", resp)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]map[string]any)
	if len(tools) < 8 {
		t.Fatalf("expected several tools, got %d", len(tools))
	}
}

func TestToolCallClaimIssueUsesDefaultActor(t *testing.T) {
	fake := &fakeClient{claimResp: core.ClaimResponse{LeaseToken: "tok"}}
	s := NewServer(fake, "codex-actor", "0055")
	resp := s.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("3"),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"claim_issue","arguments":{"issue_id":"afc-28","ttl_seconds":900}}`),
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected tool result, got %+v", resp)
	}
	if fake.lastHolder != "codex-actor" {
		t.Fatalf("holder = %q, want %q", fake.lastHolder, "codex-actor")
	}
	callResult := resp.Result.(map[string]any)
	if callResult["isError"] != nil {
		t.Fatalf("unexpected tool error result: %+v", callResult)
	}
}

func TestToolCallCloseIssuePassesStructuredMetadata(t *testing.T) {
	fake := &fakeClient{closeResp: core.CloseIssueResult{Status: "closed", Branch: "codex/afc-28"}}
	s := NewServer(fake, "codex-actor", "0055")
	resp := s.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("4"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name":"close_issue",
			"arguments":{
				"issue_id":"afc-28",
				"resolution":"done",
				"expected_version":2,
				"lease_token":"lease",
				"branch":"codex/afc-28",
				"pr_url":"https://example/pr/28",
				"commit_sha":"ddb6d05"
			}
		}`),
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected close tool result, got %+v", resp)
	}
	if fake.lastCloseReq.Branch != "codex/afc-28" || fake.lastCloseReq.PRURL != "https://example/pr/28" || fake.lastCloseReq.CommitSHA != "ddb6d05" {
		t.Fatalf("unexpected close request: %+v", fake.lastCloseReq)
	}
}

func TestRunProcessesFramedMessages(t *testing.T) {
	fake := &fakeClient{healthResp: core.Health{Name: "af-coordinator", Status: "ok"}}
	s := NewServer(fake, "tester", "0055")

	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"health","arguments":{}}}`
	input := frame(request)
	var out bytes.Buffer
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "Content-Length:") {
		t.Fatalf("expected framed response, got %q", out.String())
	}
	if !strings.Contains(out.String(), `"status":"ok"`) {
		t.Fatalf("expected health payload in response, got %q", out.String())
	}
}

func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}
