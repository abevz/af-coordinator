package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abevz/af-coordinator/internal/core"
)

func TestClientError_Error(t *testing.T) {
	t.Parallel()
	err := &ClientError{Code: "not_found", Message: "issue not found"}
	want := "not_found: issue not found"
	if got := err.Error(); got != want {
		t.Errorf("ClientError.Error() = %q, want %q", got, want)
	}
}

func TestNew(t *testing.T) {
	c := New("/tmp/test.sock")
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.socketPath != "/tmp/test.sock" {
		t.Errorf("socketPath = %q, want %q", c.socketPath, "/tmp/test.sock")
	}
}

// testClient creates a Client whose transport dials the given test server.
func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	return &Client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial(server.Listener.Addr().Network(), server.Listener.Addr().String())
				},
			},
		},
	}
}

func TestDoJSON_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/healthz" {
			t.Errorf("path = %q, want /healthz", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := testClient(t, server)
	var result struct {
		Status string `json:"status"`
	}
	if err := c.doJSON(context.Background(), "GET", "/healthz", nil, &result); err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("Status = %q, want %q", result.Status, "ok")
	}
}

func TestDoJSON_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"lease_held","message":"already claimed"}}`))
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.doJSON(context.Background(), "POST", "/v1/issues/x/claim", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var clientErr *ClientError
	if !asError(t, err, &clientErr) {
		t.Fatalf("expected *ClientError, got %T", err)
	}
	if clientErr.Code != "lease_held" {
		t.Errorf("Code = %q, want %q", clientErr.Code, "lease_held")
	}
	if clientErr.Message != "already claimed" {
		t.Errorf("Message = %q, want %q", clientErr.Message, "already claimed")
	}
}

func TestDoJSON_NonJSONError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.doJSON(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should NOT be a ClientError for non-JSON responses
	var clientErr *ClientError
	if asError(t, err, &clientErr) {
		t.Errorf("unexpected *ClientError for non-JSON response")
	}
}

func TestDoJSON_NilTarget(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := testClient(t, server)
	if err := c.doJSON(context.Background(), "DELETE", "/v1/issues/x/release", nil, nil); err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"af-coordinator","status":"ok"}`))
	}))
	defer server.Close()

	c := testClient(t, server)
	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if h.Name != "af-coordinator" {
		t.Errorf("Name = %q, want %q", h.Name, "af-coordinator")
	}
	if h.Status != "ok" {
		t.Errorf("Status = %q, want %q", h.Status, "ok")
	}
}

func TestListIssuesEncodesExternalKeyQuery(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/issues" {
			t.Fatalf("path = %q, want /v1/issues", r.URL.Path)
		}
		if got := r.URL.Query().Get("external_key"); got != "gh://abevz/af-coordinator/issues/26" {
			t.Fatalf("external_key = %q", got)
		}
		if strings.Contains(r.URL.RawQuery, "gh://") {
			t.Fatalf("raw query should be encoded, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[]}`))
	}))
	defer server.Close()

	c := testClient(t, server)
	issues, err := c.ListIssues(context.Background(), "afc", "", "", "", "", "", "gh://abevz/af-coordinator/issues/26")
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected empty issue list, got %d", len(issues))
	}
}

func TestClaimIssueWithSessionSendsCorrelationAndReturnsAttempt(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/issues/i1/claim" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var req core.ClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Holder != "agent" || req.TTLSeconds != 900 || req.SessionID != "session-1" {
			t.Fatalf("unexpected claim request: %+v", req)
		}
		_, _ = w.Write([]byte(`{"lease_token":"secret","expires_at":"2026-07-13T21:00:00Z","attempt_id":"attempt-1"}`))
	}))
	defer server.Close()

	claim, err := testClient(t, server).ClaimIssueWithSession(context.Background(), "i1", "agent", 900, "session-1")
	if err != nil {
		t.Fatalf("ClaimIssueWithSession() error = %v", err)
	}
	if claim.AttemptID != "attempt-1" {
		t.Fatalf("attempt_id = %q", claim.AttemptID)
	}
}

func TestCloseIssueReturnsStructuredResult(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/issues/i1/close" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"closed","resolution":"done","branch":"codex/afc-27","pr_url":"https://github.com/abevz/af-coordinator/pull/27","commit_sha":"ba6d011","external_key":"temporal:workflow-456","closed_at":"2026-07-08T15:50:00Z"}`))
	}))
	defer server.Close()

	c := testClient(t, server)
	result, err := c.CloseIssue(context.Background(), "i1", core.CloseIssueRequest{Resolution: "done"})
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}
	if result.Status != "closed" || result.Branch != "codex/afc-27" || result.ExternalKey != "temporal:workflow-456" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOperatorIssueMethodsUseExplicitTokenlessPaths(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/v1/issues/i1/operator-close":
			if r.Method != http.MethodPost {
				t.Fatalf("operator close method = %q", r.Method)
			}
			var req core.OperatorCloseIssueRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Reason != "complete parent" || req.Actor != "operator" {
				t.Fatalf("unexpected operator close request: %+v", req)
			}
			_, _ = w.Write([]byte(`{"status":"closed","resolution":"done"}`))
		case "/v1/issues/i1/operator-reopen":
			if r.Method != http.MethodPost {
				t.Fatalf("operator reopen method = %q", r.Method)
			}
			var req core.OperatorReopenIssueRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Reason != "needs follow-up" || req.Actor != "operator" {
				t.Fatalf("unexpected operator reopen request: %+v", req)
			}
			_, _ = w.Write([]byte(`{"issue":{"id":"i1","status":"open"}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	if _, err := c.OperatorCloseIssue(context.Background(), "i1", core.OperatorCloseIssueRequest{
		Resolution: "done", ExpectedVersion: 1, Actor: "operator", Reason: "complete parent",
	}); err != nil {
		t.Fatalf("OperatorCloseIssue() error = %v", err)
	}
	issue, err := c.OperatorReopenIssue(context.Background(), "i1", core.OperatorReopenIssueRequest{
		ExpectedVersion: 2, Actor: "operator", Reason: "needs follow-up",
	})
	if err != nil {
		t.Fatalf("OperatorReopenIssue() error = %v", err)
	}
	if issue.Status != "open" || requests != 2 {
		t.Fatalf("unexpected operator results: issue=%+v requests=%d", issue, requests)
	}
}

func TestExportJSONL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/export/jsonl" {
			t.Fatalf("path = %q, want /v1/export/jsonl", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"project\",\"payload\":{\"id\":\"p1\"}}\n"))
	}))
	defer server.Close()

	c := testClient(t, server)
	var buf bytes.Buffer
	if err := c.ExportJSONL(context.Background(), &buf); err != nil {
		t.Fatalf("ExportJSONL() error = %v", err)
	}
	if got := buf.String(); got != "{\"type\":\"project\",\"payload\":{\"id\":\"p1\"}}\n" {
		t.Fatalf("ExportJSONL() output = %q", got)
	}
}

// asError is a type-safe helper for errors.As in tests.
func asError[T error](t *testing.T, err error, target *T) bool {
	t.Helper()
	return errors.As(err, target)
}
