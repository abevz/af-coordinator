package client

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
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
	if err := c.doJSON("GET", "/healthz", nil, &result); err != nil {
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
	err := c.doJSON("POST", "/v1/issues/x/claim", nil, nil)
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
	err := c.doJSON("GET", "/test", nil, nil)
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
	if err := c.doJSON("DELETE", "/v1/issues/x/release", nil, nil); err != nil {
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
	h, err := c.Health()
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

// asError is a type-safe helper for errors.As in tests.
func asError[T error](t *testing.T, err error, target *T) bool {
	t.Helper()
	return errors.As(err, target)
}
