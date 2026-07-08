package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/abevz/af-coordinator/internal/core"
)

func TestPrintIssueFullShowsDependencyShortAndUUID(t *testing.T) {
	issue := core.Issue{
		ID:        "issue-1",
		ShortID:   "afc-1",
		Status:    "open",
		IssueType: "task",
		Title:     "Test issue",
		Priority:  3,
		ScopeKind: "project",
		Version:   1,
		CreatedAt: "2026-07-07T18:00:00Z",
		UpdatedAt: "2026-07-07T18:00:00Z",
		Dependencies: []core.Dependency{
			{
				IssueID:          "issue-1",
				IssueShortID:     "afc-1",
				DependsOnID:      "issue-2",
				DependsOnShortID: "afc-2",
				Kind:             "blocks",
			},
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	printIssueFull(issue, nil, nil, nil, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Dependencies:") {
		t.Fatalf("expected dependency section in output, got %q", out)
	}
	if !strings.Contains(out, "blocks afc-2 [issue-2]") {
		t.Fatalf("expected dependency short id and UUID in output, got %q", out)
	}
}

func TestPrintIssueFullShowsExternalKey(t *testing.T) {
	issue := core.Issue{
		ID:          "issue-1",
		ShortID:     "afc-1",
		Status:      "open",
		IssueType:   "task",
		Title:       "Test issue",
		ExternalKey: "gh://abevz/af-coordinator/issues/26",
		Priority:    3,
		ScopeKind:   "project",
		Version:     1,
		CreatedAt:   "2026-07-07T18:00:00Z",
		UpdatedAt:   "2026-07-07T18:00:00Z",
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	printIssueFull(issue, nil, nil, nil, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "External Key:  gh://abevz/af-coordinator/issues/26") {
		t.Fatalf("expected external key in output, got %q", out)
	}
}
