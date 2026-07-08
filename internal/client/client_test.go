package client

import (
	"context"
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

// asError is a type-safe helper for errors.As in tests.
func asError[T error](t *testing.T, err error, target *T) bool {
	t.Helper()
	return errors.As(err, target)
}
