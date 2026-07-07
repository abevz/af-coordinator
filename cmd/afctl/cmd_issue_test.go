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
