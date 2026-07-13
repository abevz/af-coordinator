package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunIssueUnlinkRequiresFlagValues(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "path missing value",
			args:    []string{"afc-1", "--path"},
			wantErr: "error: --path requires a value",
		},
		{
			name:    "artifact missing value",
			args:    []string{"afc-1", "--artifact"},
			wantErr: "error: --artifact requires a value",
		},
		{
			name:    "relation missing value",
			args:    []string{"afc-1", "--artifact", "docs/spec.md", "--relation"},
			wantErr: "error: --relation requires a value",
		},
		{
			name:    "artifact value is another flag",
			args:    []string{"afc-1", "--artifact", "--relation", "implements"},
			wantErr: "error: --artifact requires a value",
		},
		{
			name:    "relation value is another flag",
			args:    []string{"afc-1", "--artifact", "docs/spec.md", "--relation", "--path"},
			wantErr: "error: --relation requires a value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runIssueUnlink(context.Background(), nil, tt.args)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestOperatorCommandsRejectLeaseTokenFlag(t *testing.T) {
	t.Parallel()

	err := runIssue(context.Background(), nil, []string{
		"operator-close", "afc-50", "--resolution", "done", "--expected-version", "1",
		"--reason", "completed parent", "--lease-token", "fake",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("operator-close error = %v, want unknown flag", err)
	}

	err = runIssue(context.Background(), nil, []string{
		"operator-reopen", "afc-50", "--expected-version", "2", "--reason", "needs work",
		"--lease-token", "fake",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("operator-reopen error = %v, want unknown flag", err)
	}
}

func TestIssueHandoffValidatesRequiredHandoffNote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing token",
			args:    []string{"handoff", "afc-52", "--note", "HANDOFF: next steps"},
			wantErr: "--lease-token is required",
		},
		{
			name:    "missing note",
			args:    []string{"handoff", "afc-52", "--lease-token", "token"},
			wantErr: "note is required",
		},
		{
			name:    "malformed note",
			args:    []string{"handoff", "afc-52", "--lease-token", "token", "--note", "continue later"},
			wantErr: "note must begin with HANDOFF:",
		},
		{
			name:    "unknown flag",
			args:    []string{"handoff", "afc-52", "--lease-token", "token", "--note", "HANDOFF: next steps", "--author", "agent"},
			wantErr: "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runIssue(context.Background(), nil, tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
