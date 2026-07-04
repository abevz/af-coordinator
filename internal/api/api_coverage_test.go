package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestApiCoverage_ListEndpoints(t *testing.T) {
	ts, db := newTestServer(t)
	// Suppress errcheck warning for the test
	_ = db

	// Create required entities so list endpoints don't just return empty arrays
	resp1, _ := http.Post(ts.URL+"/v1/projects", "application/json", strings.NewReader(`{"key":"cov-proj","name":"Cov Proj"}`))
	resp1.Body.Close()
	resp2, _ := http.Post(ts.URL+"/v1/repos", "application/json", strings.NewReader(`{"project_key":"cov-proj","logical_name":"cov-repo","canonical_git_dir":"/tmp/cov"}`))
	var repo struct {
		ID string `json:"id"`
	}
	bodyRepo, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(bodyRepo, &repo)
	resp2.Body.Close()
	resp3, _ := http.Post(ts.URL+"/v1/worktrees", "application/json", strings.NewReader(`{"repo_identifier":"cov-repo","absolute_path":"/tmp/cov/wkt"}`))
	resp3.Body.Close()
	resp4, _ := http.Post(ts.URL+"/v1/artifact-roots", "application/json", strings.NewReader(`{"repo_identifier":"cov-repo","root_path":"/tmp/cov/art","kind":"test"}`))
	resp4.Body.Close()
	resp5, _ := http.Post(ts.URL+"/v1/issues", "application/json", strings.NewReader(`{"scope_kind":"project","scope_identifier":"cov-proj","title":"Cov Issue"}`))

	body, _ := io.ReadAll(resp5.Body)
	resp5.Body.Close()

	var issue struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &issue)

	resp6, _ := http.Post(ts.URL+"/v1/issues/"+issue.ID+"/artifacts", "application/json", strings.NewReader(`{"repo_identifier":"cov-repo","relative_path":"foo.go"}`))
	resp6.Body.Close()

	// Now hit the untested list endpoints
	endpoints := []string{
		"/v1/worktrees?repo=" + repo.ID,
		"/v1/artifact-roots?repo=" + repo.ID,
		"/v1/artifacts?repo=" + repo.ID,
		"/v1/issues/ready?project=cov-proj",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			resp, err := http.Get(ts.URL + ep)
			if err != nil {
				t.Fatalf("GET %s failed: %v", ep, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("expected 200, got %d. Body: %s", resp.StatusCode, string(body))
			}
		})
	}
}

func TestApiCoverage_BadJSON(t *testing.T) {
	ts, _ := newTestServer(t)

	badEndpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/v1/projects"},
		{"POST", "/v1/repos"},
		{"POST", "/v1/worktrees"},
		{"POST", "/v1/artifact-roots"},
		{"POST", "/v1/artifacts"},
		{"POST", "/v1/issues"},
		{"POST", "/v1/issues/123/claim"},
		{"POST", "/v1/issues/123/heartbeat"},
		{"POST", "/v1/issues/123/release"},
		{"POST", "/v1/issues/123/close"},
		{"POST", "/v1/issues/123/notes"},
	}

	for _, ep := range badEndpoints {
		t.Run(ep.path+"_BadJSON", func(t *testing.T) {
			req, _ := http.NewRequest(ep.method, ts.URL+ep.path, strings.NewReader(`{bad json`))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("req failed: %v", err)
			}
			defer resp.Body.Close()
			expectedCode := http.StatusBadRequest
			if strings.Contains(ep.path, "/123/") {
				expectedCode = http.StatusNotFound
			}
			if resp.StatusCode != expectedCode {
				t.Errorf("expected %d for bad JSON on %s %s, got %d", expectedCode, ep.method, ep.path, resp.StatusCode)
			}
		})
	}
}
