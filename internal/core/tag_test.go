package core

import "testing"

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{"valid simple", "area/frontend", false},
		{"valid with hyphens and digits", "exec-2/auto-v2", false},
		{"empty", "", true},
		{"missing slash", "areafrontend", true},
		{"double slash", "area/front/end", true},
		{"trailing slash", "area/", true},
		{"leading slash", "/frontend", true},
		{"uppercase", "Area/Frontend", true},
		{"underscore not allowed", "area/front_end", true},
		{"space not allowed", "area/front end", true},
		{"reserved namespace open", "open/x", true},
		{"reserved namespace blocked", "blocked/x", true},
		{"reserved namespace done", "done/x", true},
		{"reserved namespace in_progress", "in_progress/x", true},
		{"reserved namespace status", "status/x", true},
		{"reserved namespace state", "state/x", true},
		{"over length", "a-very-long-namespace-that-goes-on-and-on/a-very-long-value-that-goes-on-and-on-too", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTag(%q) error = %v, wantErr = %v", tt.tag, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCreateIssueRejectsBadTags(t *testing.T) {
	req := CreateIssueRequest{
		Project:   "proj",
		ScopeKind: "project",
		Title:     "title",
		Tags:      []string{"area/frontend", "bad tag"},
	}
	err := ValidateCreateIssue(req)
	if err == nil {
		t.Fatal("expected error for invalid tag in CreateIssueRequest.Tags")
	}
}

func TestValidateCreateIssueAcceptsGoodTags(t *testing.T) {
	req := CreateIssueRequest{
		Project:   "proj",
		ScopeKind: "project",
		Title:     "title",
		Tags:      []string{"area/frontend", "theme/dark"},
	}
	if err := ValidateCreateIssue(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
