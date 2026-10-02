package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/report"
	"github.com/abevz/dibs/internal/store/sqlite"
)

func TestStatsFreshDatabaseClaimWithoutProgress(t *testing.T) {
	server, db := newTestServer(t) // Real migrations, including 0005, on an empty DB.
	ctx := context.Background()
	if _, err := sqlite.CreateProject(ctx, db, "fresh", "Fresh", ""); err != nil {
		t.Fatal(err)
	}
	issue, err := sqlite.CreateIssue(ctx, db, "fresh", core.CreateIssueRequest{ScopeKind: "project", Title: "Repeated releases"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		claim, err := sqlite.ClaimIssue(ctx, db, issue.ID, "codex-12", 300)
		if err != nil {
			t.Fatal(err)
		}
		if err := sqlite.ReleaseLease(ctx, db, issue.ID, claim.LeaseToken, claim.LeaseGeneration, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	var markers, nonpositive int
	if err := db.QueryRow(`SELECT count(*) FROM events WHERE event_type = 'event_ordering_enabled'`).Scan(&markers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM events WHERE sequence <= 0`).Scan(&nonpositive); err != nil {
		t.Fatal(err)
	}
	if markers != 0 || nonpositive != 0 {
		t.Fatalf("fresh event ordering: markers=%d, nonpositive sequences=%d", markers, nonpositive)
	}
	r, err := http.Get(server.URL + "/v1/stats?by=actor&top=1&project=fresh")
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("status %d", r.StatusCode)
	}
	got := decodeJSON[struct {
		Report report.Report `json:"report"`
	}](t, r).Report
	if len(got.TopIssues) != 1 {
		t.Fatalf("top issues: %#v", got.TopIssues)
	}
	cost := got.TopIssues[0]
	if cost.NoProgressReleases != 3 || cost.UnclassifiedReleases != 0 || !cost.ClaimWithoutProgress {
		t.Fatalf("fresh releases misclassified: no-progress=%d, unclassified=%d, flag=%v", cost.NoProgressReleases, cost.UnclassifiedReleases, cost.ClaimWithoutProgress)
	}
	if got.ByActor["codex"].Claims != got.Attempts.Claims || got.ByActor["codex"].Completed != got.Attempts.Completed {
		t.Fatalf("actor totals do not reconcile: %#v", got.ByActor)
	}
}

func TestStatsBreakdownQueryAndOwner(t *testing.T) {
	server, db := newTestServer(t)
	at := "2026-09-29T00:00:00Z"
	if _, err := db.Exec(`INSERT INTO projects (id,key,name,created_at,updated_at) VALUES ('p','fixture','Fixture',?,?)`, at, at); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id,short_id,project_id,scope_kind,issue_type,title,status,created_at,updated_at) VALUES ('i','fixture-1','p','project','task','Fixture','open',?,?)`, at, at); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ id, kind, actor, payload, at string }{
		{"claim", "issue_claimed", "codex-12", `{"attempt_id":"a"}`, "2026-09-29T01:00:00Z"},
		{"release", "issue_operator_released", "admin", `{"attempt_id":"a"}`, "2026-09-29T01:02:00Z"},
	} {
		if _, err := db.Exec(`INSERT INTO events (id,issue_id,actor,event_type,payload_json,created_at) VALUES (?,'i',?,?,?,?)`, e.id, e.actor, e.kind, e.payload, e.at); err != nil {
			t.Fatal(err)
		}
	}
	r, err := http.Get(server.URL + "/v1/stats?by=actor&top=1&project=fixture&until=2026-09-30T00%3A00%3A00Z")
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	got := decodeJSON[struct {
		Report report.Report `json:"report"`
	}](t, r).Report
	if got.ByActor["codex"].Claims != 1 || got.ByActor["codex"].Outcomes["operator_released"] != 1 || len(got.TopIssues) != 1 || got.TopIssues[0].OperatorReleased != 1 {
		t.Fatalf("breakdowns %#v", got)
	}
	for _, query := range []string{"by=host", "top=0", "top=-1", "top=bad"} {
		r, err := http.Get(server.URL + "/v1/stats?" + query)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != 400 {
			t.Fatalf("%s: %d", query, r.StatusCode)
		}
		e := decodeJSON[core.APIErrorResponse](t, r)
		if e.Error.Code != core.ErrValidationFailed {
			t.Fatalf("error %#v", e)
		}
	}
}
