package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/abevz/af-coordinator/internal/core"
)

func TestParseIssueListArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     core.IssueListParams
		wantHelp bool
		wantErr  string
	}{
		{
			name: "csv and repeated filters",
			args: []string{"--project", "afc,aion", "--type", "epic,chore", "--status", "open", "--status", "in_progress"},
			want: core.IssueListParams{
				Projects:   []string{"afc", "aion"},
				IssueTypes: []string{"epic", "chore"},
				Statuses:   []string{"open", "in_progress"},
			},
		},
		{name: "help", args: []string{"--help"}, wantHelp: true},
		{name: "unknown flag", args: []string{"--wat"}, wantErr: "unknown flag"},
		{name: "missing value", args: []string{"--project"}, wantErr: "requires a value"},
		{name: "empty csv element", args: []string{"--type", "epic,"}, wantErr: "empty elements"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, help, err := parseIssueListArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if help != tt.wantHelp {
				t.Fatalf("help = %v, want %v", help, tt.wantHelp)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("params = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestIssueListHelpDoesNotRequireClient(t *testing.T) {
	if err := runIssueList(context.Background(), nil, []string{"--help"}); err != nil {
		t.Fatalf("issue list help: %v", err)
	}
	if err := runLs(context.Background(), nil, []string{"--help"}); err != nil {
		t.Fatalf("ls help: %v", err)
	}
}

func TestShouldCheckDaemonVersion(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"ls", "--help"}, want: false},
		{args: []string{"issue", "list", "--help"}, want: false},
		{args: []string{"init"}, want: false},
		{args: []string{"protocol"}, want: false},
		{args: []string{"ls", "--project", "afc"}, want: true},
	}
	for _, tt := range tests {
		if got := shouldCheckDaemonVersion(tt.args); got != tt.want {
			t.Errorf("shouldCheckDaemonVersion(%q) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

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
