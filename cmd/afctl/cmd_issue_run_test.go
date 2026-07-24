package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestIssueRunFlagParsingErrors covers the validation paths that don't need
// a daemon: they're handled entirely by runIssue's own arg parsing before
// any client call, mirroring the pattern used for the other lifecycle
// commands' usage tests.
func TestIssueRunFlagParsingErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing separator",
			args:    []string{"afc-1", "echo", "hi"},
			wantErr: "missing -- separator",
		},
		{
			name:    "no command after separator",
			args:    []string{"afc-1", "--"},
			wantErr: "no command given after --",
		},
		{
			name:    "missing issue id",
			args:    []string{"--"},
			wantErr: "Usage: afctl issue run",
		},
		{
			name:    "bad close-resolution",
			args:    []string{"afc-1", "--close-resolution", "maybe", "--", "true"},
			wantErr: "--close-resolution must be done or cancelled",
		},
		{
			name:    "non-positive ttl",
			args:    []string{"afc-1", "--ttl", "0", "--", "true"},
			wantErr: "--ttl must be positive",
		},
		{
			name:    "unknown flag",
			args:    []string{"afc-1", "--bogus", "x", "--", "true"},
			wantErr: "unknown flag",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"run"}, tt.args...)
			err := runIssue(t.Context(), nil, args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestIssueRunHelpFlagShortCircuits(t *testing.T) {
	if err := runIssue(t.Context(), nil, []string{"run", "-h"}); err != nil {
		t.Errorf("runIssue(run, -h) = %v, want nil", err)
	}
}

// mockCoordinator is a minimal /v1/issues/{id}/{claim,heartbeat,close,handoff}
// server used to drive `afctl issue run` end to end through a real subprocess,
// since claim/heartbeat/exec/close all happen inside a single compiled
// binary invocation -- there's no lighter-weight in-process seam for it.
type mockCoordinator struct {
	mu           sync.Mutex
	claimVersion int
	closeReqs    []map[string]any
	handoffReqs  []map[string]any
}

func (m *mockCoordinator) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/issues/{id}/claim", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"lease_token": "test-lease-token",
			"expires_at":  "2099-01-01T00:00:00Z",
			"attempt_id":  "test-attempt-id",
			"version":     m.claimVersion,
		})
	})
	mux.HandleFunc("POST /v1/issues/{id}/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"expires_at": "2099-01-01T00:00:00Z"})
	})
	mux.HandleFunc("POST /v1/issues/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		m.closeReqs = append(m.closeReqs, body)
		m.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"status": "closed", "resolution": body["resolution"]})
	})
	mux.HandleFunc("POST /v1/issues/{id}/handoff", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		m.handoffReqs = append(m.handoffReqs, body)
		m.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"note": map[string]any{"id": "n1", "body": body["note"]}})
	})
	return mux
}

func buildAfctlForRunTest(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "afctl")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build afctl: %v\noutput: %s", err, out)
	}
	return binPath
}

func startMockCoordinator(t *testing.T, mock *mockCoordinator) string {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "af-coordinator.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	go http.Serve(l, mock.handler())
	return sockPath
}

func TestIssueRunClosesOnSuccess(t *testing.T) {
	binPath := buildAfctlForRunTest(t)
	mock := &mockCoordinator{claimVersion: 7}
	sockPath := startMockCoordinator(t, mock)

	cmd := exec.Command(binPath, "issue", "run", "afc-1", "--actor", "tester", "--ttl", "60", "--", "sh", "-c", "exit 0")
	cmd.Env = append(os.Environ(), "AF_COORDINATOR_SOCKET="+sockPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("issue run failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.closeReqs) != 1 {
		t.Fatalf("expected exactly one close request, got %d", len(mock.closeReqs))
	}
	if len(mock.handoffReqs) != 0 {
		t.Fatalf("expected no handoff requests on success, got %d", len(mock.handoffReqs))
	}
	got := mock.closeReqs[0]
	if got["resolution"] != "done" {
		t.Errorf("resolution = %v, want done", got["resolution"])
	}
	if v, ok := got["expected_version"].(float64); !ok || int(v) != 7 {
		t.Errorf("expected_version = %v, want the claimed version 7 (not re-fetched)", got["expected_version"])
	}
	if got["lease_token"] != "test-lease-token" {
		t.Errorf("lease_token = %v, want test-lease-token", got["lease_token"])
	}
}

func TestIssueRunHandsOffOnFailureAndMirrorsExitCode(t *testing.T) {
	binPath := buildAfctlForRunTest(t)
	mock := &mockCoordinator{claimVersion: 3}
	sockPath := startMockCoordinator(t, mock)

	cmd := exec.Command(binPath, "issue", "run", "afc-2", "--actor", "tester", "--ttl", "60", "--", "sh", "-c", "exit 5")
	cmd.Env = append(os.Environ(), "AF_COORDINATOR_SOCKET="+sockPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected an *exec.ExitError, got %v (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}
	if exitErr.ExitCode() != 5 {
		t.Fatalf("exit code = %d, want 5 (mirroring the child's exit code)", exitErr.ExitCode())
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.closeReqs) != 0 {
		t.Fatalf("expected no close requests on failure, got %d", len(mock.closeReqs))
	}
	if len(mock.handoffReqs) != 1 {
		t.Fatalf("expected exactly one handoff request, got %d", len(mock.handoffReqs))
	}
	note, _ := mock.handoffReqs[0]["note"].(string)
	if !strings.HasPrefix(note, "HANDOFF:") {
		t.Errorf("handoff note = %q, want HANDOFF: prefix", note)
	}
	if !strings.Contains(note, "exit 5") {
		t.Errorf("handoff note = %q, want it to mention exit 5", note)
	}
}

func TestIssueRunExportsLeaseEnvToChild(t *testing.T) {
	binPath := buildAfctlForRunTest(t)
	mock := &mockCoordinator{claimVersion: 1}
	sockPath := startMockCoordinator(t, mock)

	printEnv := `test "$AF_LEASE_TOKEN" = "test-lease-token" || exit 10
test "$AF_ATTEMPT_ID" = "test-attempt-id" || exit 11
test "$AF_ISSUE_ID" = "afc-3" || exit 12
test "$AF_EXPECTED_VERSION" = "1" || exit 13
exit 0`
	cmd := exec.Command(binPath, "issue", "run", "afc-3", "--actor", "tester", "--ttl", "60", "--", "sh", "-c", printEnv)
	cmd.Env = append(os.Environ(), "AF_COORDINATOR_SOCKET="+sockPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("issue run failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
}
