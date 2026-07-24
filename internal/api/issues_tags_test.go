package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAddRemoveTag(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-1', 'test-1', 'proj-1', 'project', 'First', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Add tag.
	body := `{"tag":"area/frontend","actor":"test"}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/issue-1/tags", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for add tag, got %d", resp.StatusCode)
	}

	// GET issue includes the tag.
	getResp, err := http.Get(server.URL + "/v1/issues/issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get issue, got %d", getResp.StatusCode)
	}

	// Remove tag.
	req, err = http.NewRequest("DELETE", server.URL+"/v1/issues/issue-1/tags?tag=area%2Ffrontend&actor=test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for tag removal, got %d", resp.StatusCode)
	}
}

func TestAddTagValidationFailure(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-1', 'test-1', 'proj-1', 'project', 'First', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"tag":"status/open","actor":"test"}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/issue-1/tags", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for reserved namespace tag, got %d", resp.StatusCode)
	}
}

func TestAddTagConflict(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-1', 'test-1', 'proj-1', 'project', 'First', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"tag":"area/frontend","actor":"test"}`
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("POST", server.URL+"/v1/issues/issue-1/tags", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created on first add, got %d", resp.StatusCode)
		}
		if i == 1 && resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409 Conflict on duplicate add, got %d", resp.StatusCode)
		}
	}
}

func TestRemoveTagNotFoundAPI(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-1', 'test-1', 'proj-1', 'project', 'First', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("DELETE", server.URL+"/v1/issues/issue-1/tags?tag=area%2Ffrontend", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for removing nonexistent tag, got %d", resp.StatusCode)
	}
}

func TestListIssuesRepeatedTagQueryANDSemantics(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-both', 'test-1', 'proj-1', 'project', 'Both', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-one', 'test-2', 'proj-1', 'project', 'One', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][2]string{{"issue-both", "area/frontend"}, {"issue-both", "theme/dark"}, {"issue-one", "area/frontend"}} {
		if _, err := db.Exec(
			`INSERT INTO issue_tags (issue_id, tag, created_at) VALUES (?, ?, ?)`,
			row[0], row[1], now,
		); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := http.Get(server.URL + "/v1/issues?project=test&tag=area%2Ffrontend&tag=theme%2Fdark")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var result struct {
		Issues []struct {
			ID string `json:"id"`
		} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].ID != "issue-both" {
		t.Fatalf("expected only issue-both to match both tags, got %+v", result.Issues)
	}
}

func TestListReadyIssuesRepeatedTagQueryANDSemantics(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-both', 'test-1', 'proj-1', 'project', 'Both', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-one', 'test-2', 'proj-1', 'project', 'One', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][2]string{{"issue-both", "exec/auto"}, {"issue-both", "area/frontend"}, {"issue-one", "exec/auto"}} {
		if _, err := db.Exec(
			`INSERT INTO issue_tags (issue_id, tag, created_at) VALUES (?, ?, ?)`,
			row[0], row[1], now,
		); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := http.Get(server.URL + "/v1/issues/ready?project=test&tag=exec%2Fauto&tag=area%2Ffrontend")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var result struct {
		Issues []struct {
			ID string `json:"id"`
		} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].ID != "issue-both" {
		t.Fatalf("expected only issue-both to match both tags, got %+v", result.Issues)
	}
}
