package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevisionSkew(t *testing.T) {
	// Build afctl with a known, fixed Revision so the test can control both
	// sides of the comparison deterministically (a plain `go build` here
	// would leave build.Revision at its "unknown" default, which the
	// pre-command warning intentionally never flags).
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "afctl")
	const localRevision = "test-revision-abc"
	cmd := exec.Command("go", "build", "-buildvcs=false",
		"-ldflags", "-X github.com/abevz/af-coordinator/internal/build.Revision="+localRevision,
		"-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build afctl: %v\noutput: %s", err, out)
	}

	tests := []struct {
		name           string
		daemonRevision string
		args           []string
		wantStderr     bool
	}{
		{
			name:           "mismatch prints warning",
			daemonRevision: "old-revision",
			args:           []string{"ls"},
			wantStderr:     true,
		},
		{
			name:           "match is silent",
			daemonRevision: localRevision,
			args:           []string{"ls"},
			wantStderr:     false,
		},
		{
			name:           "unknown daemon revision is silent",
			daemonRevision: "unknown",
			args:           []string{"ls"},
			wantStderr:     false,
		},
		{
			name:           "init ignores skew",
			daemonRevision: "old-revision",
			args:           []string{"init"},
			wantStderr:     false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sockPath := filepath.Join(tmpDir, fmt.Sprintf("s%d.sock", i))
			os.Remove(sockPath)

			l, err := net.Listen("unix", sockPath)
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer l.Close()

			mux := http.NewServeMux()
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"status": "ok", "revision": tt.daemonRevision})
			})
			// Mock /v1/projects for ls
			mux.HandleFunc("/v1/projects", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"projects":[]}`))
			})
			go http.Serve(l, mux)

			runCmd := exec.Command(binPath, tt.args...)
			// Run in a temp dir: `init` without --path writes AGENTS.md
			// into the current directory.
			runCmd.Dir = t.TempDir()
			runCmd.Env = append(os.Environ(), "AF_COORDINATOR_SOCKET="+sockPath)
			var stderr bytes.Buffer
			runCmd.Stderr = &stderr

			_ = runCmd.Run() // exit code might be non-zero for some commands if mock is incomplete, that's fine

			out := stderr.String()
			hasWarning := strings.Contains(out, "restart af-coordinatord")

			if tt.wantStderr && !hasWarning {
				t.Errorf("expected warning in stderr, got: %q", out)
			}
			if !tt.wantStderr && hasWarning {
				t.Errorf("expected no warning in stderr, got: %q", out)
			}
		})
	}
}
