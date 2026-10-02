package report

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/abevz/dibs/internal/core"
)

func TestNormalizeActor(t *testing.T) {
	cases := map[string]string{"codex-11041": "codex", "codex-afc-175": "codex", "agy-42": "agy", "claude.exe-44": "claude", "claude-code": "claude", "aion-forge-worker@arch:12986/1": "aion-forge-worker", "aion-worker-live-1": "aion-forge-worker", "aion-551-evidence-writer": "task/evidence-writer", "afc-96-reviewer": "task/reviewer", "2.1.220": "unknown", "": "unknown", "custom-42": "custom-42", "custom@host": "custom"}
	for raw, want := range cases {
		if got := NormalizeActor(raw); got != want {
			t.Errorf("%q -> %q, want %q", raw, got, want)
		}
	}
}
func agentFixture() fixtureSource {
	f := fixtureSource{projects: []core.Project{{ID: "p", Key: "fixture"}, {ID: "other", Key: "other"}}, repos: []core.Repository{{ID: "r", ProjectID: "p", LogicalName: "repo"}}}
	for i := 1; i <= 6; i++ {
		short := fmt.Sprintf("fixture-%d", i)
		if i == 1 {
			short = "afc-96"
		}
		if i == 2 {
			short = "aion-551"
		}
		f.issues = append(f.issues, core.Issue{ID: fmt.Sprint(i), ShortID: short, ProjectID: "p", RepositoryID: "r", Status: "open", CreatedAt: "2026-09-29T00:00:00Z"})
	}
	f.events = []core.Event{{ID: "cutoff", Sequence: 1, EventType: "event_ordering_enabled", PayloadJSON: "{}", CreatedAt: "2026-09-29T08:00:00Z"}}
	add := func(issue, actor, kind string, p map[string]any) {
		seq := int64(len(f.events) + 1)
		at := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC).Add(time.Duration(seq) * time.Minute)
		b, _ := json.Marshal(p)
		f.events = append(f.events, core.Event{ID: fmt.Sprint(seq), Sequence: seq, IssueID: issue, Actor: actor, EventType: kind, PayloadJSON: string(b), CreatedAt: at.Format(time.RFC3339)})
	}
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("churn-%d", i)
		add("1", "aion-forge-worker@arch:12986/1", "issue_claimed", map[string]any{"attempt_id": id})
		if i == 3 {
			add("1", "reviewer", "note_added", map[string]any{})
		}
		p := map[string]any{"attempt_id": id}
		if i == 4 {
			p["commit_sha"] = "fixture-sha"
		}
		add("1", "different-end-actor", "issue_released", p)
	}
	add("2", "codex-11", "issue_claimed", map[string]any{"attempt_id": "done"})
	add("2", "codex", "note_added", map[string]any{})
	add("2", "operator", "issue_operator_closed", map[string]any{"attempt_id": "done", "resolution": "done"})
	add("3", "claude.exe-12", "issue_claimed", map[string]any{"attempt_id": "expired"})
	add("3", "system", "lease_expired", map[string]any{"attempt_id": "expired"})
	add("4", "codex-22", "issue_claimed", map[string]any{"attempt_id": "handoff"})
	add("4", "codex", "note_added", map[string]any{})
	add("4", "codex", "issue_released", map[string]any{"attempt_id": "handoff", "end_reason": "handoff"})
	add("5", "2.1.220", "issue_claimed", map[string]any{"attempt_id": "operator"})
	add("5", "admin", "issue_operator_released", map[string]any{"attempt_id": "operator"})
	add("6", "claude-code", "issue_claimed", map[string]any{"attempt_id": "cancel"})
	add("6", "claude", "issue_closed", map[string]any{"attempt_id": "cancel", "resolution": "cancelled"})
	return f
}
func assertReconciles(t *testing.T, r Report) {
	t.Helper()
	claims, completed := 0, 0
	outcomes := map[string]int{}
	for _, a := range r.ByActor {
		claims += a.Claims
		completed += a.Completed
		for k, n := range a.Outcomes {
			outcomes[k] += n
		}
	}
	if claims != r.Attempts.Claims || completed != r.Attempts.Completed {
		t.Fatalf("counts %d/%d != global %#v", claims, completed, r.Attempts)
	}
	for k, n := range r.Attempts.Outcomes {
		if outcomes[k] != n {
			t.Fatalf("outcome %s: %d != %d", k, outcomes[k], n)
		}
	}
}
func TestAgentBreakdownReconcilesAndRanks(t *testing.T) {
	f := agentFixture()
	original := append([]core.Event(nil), f.events...)
	r, err := Build(context.Background(), f, Query{By: "actor", Top: 3}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	assertReconciles(t, r)
	if !reflect.DeepEqual(f.events, original) {
		t.Fatal("raw actors/events mutated")
	}
	if r.Attempts.Claims != 10 || r.Attempts.Completed != 10 || r.Attempts.Outcomes["operator_released"] != 1 {
		t.Fatalf("global: %#v", r.Attempts)
	}
	if r.ByActor["codex"].Claims != 2 || r.ByActor["codex"].Outcomes["done"] != 1 || r.ByActor["codex"].Outcomes["handoff"] != 1 || r.ByActor["codex"].CloseRate.Ratio != 0.5 {
		t.Fatalf("codex: %#v", r.ByActor["codex"])
	}
	if r.ByActor["claude"].Outcomes["expired"] != 1 || r.ByActor["claude"].Outcomes["cancelled"] != 1 || r.ByActor["unknown"].Outcomes["operator_released"] != 1 {
		t.Fatalf("owner attribution: %#v", r.ByActor)
	}
	if len(r.TopIssues) != 3 || r.TopIssues[0].ShortID != "afc-96" || !r.TopIssues[0].ClaimWithoutProgress || r.TopIssues[0].NoProgressReleases != 3 || r.TopIssues[0].Notes != 1 {
		t.Fatalf("top: %#v", r.TopIssues)
	}
	if r.TopIssues[1].ShortID != "aion-551" || r.TopIssues[1].Notes != 1 || r.TopIssues[1].ClaimWithoutProgress {
		t.Fatalf("second: %#v", r.TopIssues[1])
	}
	if r.ByActor["codex"].Duration.P50Seconds != 120 {
		t.Fatalf("median: %#v", r.ByActor["codex"].Duration)
	}
}
func TestAgentWindowRetainsClaimOwnerAndLegacyUncertainty(t *testing.T) {
	f := agentFixture()
	// Restrict to an end event: claim lies before since, yet the outcome owner
	// and full matched attempt duration must remain available.
	var end core.Event
	for _, e := range f.events {
		if e.EventType == "issue_operator_closed" {
			end = e
		}
	}
	r, err := Build(context.Background(), f, Query{By: "actor", Top: 10, Since: end.CreatedAt, Until: end.CreatedAt}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	assertReconciles(t, r)
	if r.ByActor["codex"].Claims != 0 || r.ByActor["codex"].Completed != 1 || r.ByActor["codex"].Duration.P50Seconds != 120 {
		t.Fatalf("window owner: %#v", r.ByActor)
	}
	// Move the exact-order marker past every claim; legacy release pairs must
	// remain unclassified rather than claim an absence of progress.
	marker := f.events[0]
	marker.Sequence = 1000
	f.events = append(f.events[1:], marker)
	r, err = Build(context.Background(), f, Query{By: "actor", Top: 10}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if r.TopIssues[0].ClaimWithoutProgress || r.TopIssues[0].NoProgressReleases != 0 || r.TopIssues[0].UnclassifiedReleases != 5 {
		t.Fatalf("legacy: %#v", r.TopIssues[0])
	}
}
func TestAgentMissingSequencesRemainUnclassified(t *testing.T) {
	f := agentFixture()
	f.events = f.events[1:] // No marker, but absent sequences are not fresh-DB evidence.
	for i := range f.events {
		f.events[i].Sequence = 0
	}
	r, err := Build(context.Background(), f, Query{By: "actor", Top: 10}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if r.TopIssues[0].ClaimWithoutProgress || r.TopIssues[0].NoProgressReleases != 0 || r.TopIssues[0].UnclassifiedReleases != 5 {
		t.Fatalf("missing sequences: %#v", r.TopIssues[0])
	}
}

func TestAgentProgressUsesSequenceDespiteClockSkew(t *testing.T) {
	for _, since := range []string{"", "2026-09-29T10:01:00Z"} {
		t.Run("since="+since, func(t *testing.T) {
			f := fixtureSource{projects: []core.Project{{ID: "p", Key: "fixture"}}, issues: []core.Issue{{ID: "i", ShortID: "fixture-1", ProjectID: "p", Status: "open", CreatedAt: "2026-09-29T09:00:00Z"}}}
			for i := 1; i <= 3; i++ {
				start := time.Date(2026, 9, 29, 10, i, 0, 0, time.UTC)
				id := fmt.Sprintf("attempt-%d", i)
				for _, e := range []core.Event{
					{EventType: "issue_claimed", Actor: "codex-12", PayloadJSON: fmt.Sprintf(`{"attempt_id":%q}`, id), CreatedAt: start.Format(time.RFC3339)},
					// AddNote emits no attempt_id. Association comes from the
					// active issue claim in the sequence-ordered event stream.
					{EventType: "note_added", Actor: "reviewer", PayloadJSON: `{"invocation_mode":"interactive"}`, CreatedAt: start.Add(-30 * time.Second).Format(time.RFC3339)},
					{EventType: "issue_released", Actor: "codex-12", PayloadJSON: fmt.Sprintf(`{"attempt_id":%q}`, id), CreatedAt: start.Add(30 * time.Second).Format(time.RFC3339)},
				} {
					e.Sequence = int64(len(f.events) + 1)
					e.ID, e.IssueID = fmt.Sprint(e.Sequence), "i"
					f.events = append(f.events, e)
				}
			}
			r, err := Build(context.Background(), f, Query{By: "actor", Top: 1, Since: since}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			cost := r.TopIssues[0]
			if cost.NoProgressReleases != 0 || cost.UnclassifiedReleases != 0 || cost.ClaimWithoutProgress {
				t.Fatalf("later-sequence notes ignored: no-progress=%d unclassified=%d flag=%v", cost.NoProgressReleases, cost.UnclassifiedReleases, cost.ClaimWithoutProgress)
			}
			wantNotes := 3
			if since != "" {
				wantNotes = 2 // Evidence before since counts as progress, not as a window note.
			}
			if cost.Notes != wantNotes || r.Attempts.Completed != 3 || r.Attempts.Duration.P50Seconds != 30 {
				t.Fatalf("window/duration contract changed: %#v; %#v", cost, r.Attempts)
			}
			assertReconciles(t, r)
		})
	}
}

func TestAgentProgressEvidenceBoundaries(t *testing.T) {
	start := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, kind, issue, attemptID, scm string
		sequence                          int64
		offset                            time.Duration
		legacy, wantProgress, wantExact   bool
	}{
		{name: "note clock skew", kind: "note_added", sequence: 11, offset: -time.Minute, wantProgress: true, wantExact: true},
		{name: "commit clock skew", kind: "issue_closed", attemptID: "current", scm: "commit_sha", sequence: 11, offset: -time.Minute, wantProgress: true, wantExact: true},
		{name: "PR clock skew", kind: "issue_closed", attemptID: "current", scm: "pr_url", sequence: 11, offset: -time.Minute, wantProgress: true, wantExact: true},
		{name: "other attempt SCM", kind: "issue_closed", attemptID: "previous", scm: "commit_sha", sequence: 11, wantExact: true},
		{name: "other issue note", kind: "note_added", issue: "other", sequence: 11, wantExact: true},
		{name: "after until", kind: "note_added", sequence: 11, offset: 2 * time.Hour, wantExact: true},
		{name: "missing note sequence", kind: "note_added", offset: -time.Minute},
		{name: "missing release sequence", kind: "issue_released", attemptID: "current"},
		{name: "non-forward sequence", kind: "note_added", sequence: 10, offset: -time.Minute},
		{name: "legacy skewed note", kind: "note_added", sequence: 11, offset: -time.Minute, legacy: true},
		{name: "legacy note", kind: "note_added", sequence: 11, offset: time.Minute, legacy: true, wantProgress: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Report{}
			b := newBreakdown(&r, []core.Issue{{ID: "i"}, {ID: "other"}}, parsedQuery{until: start.Add(time.Hour)}, 1)
			b.active["i"] = "current"
			attempts := map[string]attempt{"current": {started: start, sequence: 10, issueID: "i", exact: !tc.legacy}}
			issue := tc.issue
			if issue == "" {
				issue = "i"
			}
			p := map[string]any{}
			if tc.attemptID != "" {
				p["attempt_id"] = tc.attemptID
			}
			if tc.scm != "" {
				p[tc.scm] = "synthetic-evidence"
			}
			at := start.Add(tc.offset)
			b.observe(core.Event{IssueID: issue, EventType: tc.kind, Sequence: tc.sequence}, p, at, true, attempts)
			got := attempts["current"]
			if got.progress != tc.wantProgress || got.exact != tc.wantExact {
				t.Fatalf("progress=%v exact=%v, want %v/%v", got.progress, got.exact, tc.wantProgress, tc.wantExact)
			}
			if tc.scm != "" && at.Before(start) {
				var durations []float64
				completeAttempt(&r, attempts, p, at, "done", &durations, b)
				if r.Attempts.Completed != 0 || len(durations) != 0 {
					t.Fatal("negative-duration end became measurable")
				}
			}
		})
	}
}

func TestAgentQueryValidationAndEmptyScope(t *testing.T) {
	for _, q := range []Query{{By: "host"}, {Top: -1}} {
		if _, err := Build(context.Background(), agentFixture(), q, time.Now()); err == nil {
			t.Fatalf("accepted %#v", q)
		}
	}
	r, err := Build(context.Background(), agentFixture(), Query{By: "actor", Top: 3, Project: "other"}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if r.ByActor == nil || len(r.ByActor) != 0 || r.TopIssues == nil || len(r.TopIssues) != 0 {
		t.Fatalf("empty selection: %#v", r)
	}
	r, err = Build(context.Background(), agentFixture(), Query{}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil || r.ByActor != nil || r.TopIssues != nil {
		t.Fatal("optional sections changed default report")
	}
}
