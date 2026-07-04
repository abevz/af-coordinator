package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAFC7LinkArtifactByShortID(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	// Create a project
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create an issue using the HTTP API
	body := `{"project":"test","scope_kind":"project","title":"My issue","actor":"test"}`
	resp, err := http.Post(server.URL+"/v1/issues", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var createRes struct {
		Issue struct {
			ID      string `json:"id"`
			ShortID string `json:"short_id"`
		} `json:"issue"`
	}
	createRes = decodeJSON[struct {
		Issue struct {
			ID      string `json:"id"`
			ShortID string `json:"short_id"`
		} `json:"issue"`
	}](t, resp)
	
	// Create a repo
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create artifact root
	rootBody := `{"repo":"main","root_path":"brain/session-1"}`
	resp, err = http.Post(server.URL+"/v1/artifact-roots", "application/json", strings.NewReader(rootBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created, got %d, body: %s", resp.StatusCode, string(b))
	}

	var rootRes struct {
		ArtifactRoot struct {
			ID string `json:"id"`
		} `json:"artifact_root"`
	}
	rootRes = decodeJSON[struct {
		ArtifactRoot struct {
			ID string `json:"id"`
		} `json:"artifact_root"`
	}](t, resp)

	// Create artifact
	artBody := `{"repo":"main","artifact_root_id":"` + rootRes.ArtifactRoot.ID + `","relative_path":"doc.md","kind":"spec"}`
	resp, err = http.Post(server.URL+"/v1/artifacts", "application/json", strings.NewReader(artBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created, got %d, body: %s", resp.StatusCode, string(b))
	}

	var artRes struct {
		Artifact struct {
			ID string `json:"id"`
		} `json:"artifact"`
	}
	artRes = decodeJSON[struct {
		Artifact struct {
			ID string `json:"id"`
		} `json:"artifact"`
	}](t, resp)

	// Link artifact using ShortID
	linkBody := `{"artifact":"` + artRes.Artifact.ID + `"}`
	resp, err = http.Post(server.URL+"/v1/issues/"+createRes.Issue.ShortID+"/links", "application/json", strings.NewReader(linkBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created for link, got %d, body: %s", resp.StatusCode, string(b))
	}
}

func TestAFC7ListArtifactsLogicalRepo(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	// Create project and repo
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create artifact root
	rootBody := `{"repo":"main","root_path":"brain/session-1"}`
	resp, err := http.Post(server.URL+"/v1/artifact-roots", "application/json", strings.NewReader(rootBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created for root, got %d, body: %s", resp.StatusCode, string(b))
	}

	var rootRes struct {
		ArtifactRoot struct {
			ID string `json:"id"`
		} `json:"artifact_root"`
	}
	rootRes = decodeJSON[struct {
		ArtifactRoot struct {
			ID string `json:"id"`
		} `json:"artifact_root"`
	}](t, resp)

	// Create artifact
	artBody := `{"repo":"main","artifact_root_id":"` + rootRes.ArtifactRoot.ID + `","relative_path":"doc.md","kind":"spec"}`
	resp, err = http.Post(server.URL+"/v1/artifacts", "application/json", strings.NewReader(artBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created for artifact, got %d, body: %s", resp.StatusCode, string(b))
	}

	// List artifacts by logical repo name
	resp, err = http.Get(server.URL+"/v1/artifacts?repo=main")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK for list artifacts, got %d, body: %s", resp.StatusCode, string(b))
	}

	var listRes struct {
		Artifacts []struct {
			RelativePath string `json:"relative_path"`
		} `json:"artifacts"`
	}
	listRes = decodeJSON[struct {
		Artifacts []struct {
			RelativePath string `json:"relative_path"`
		} `json:"artifacts"`
	}](t, resp)

	if len(listRes.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(listRes.Artifacts))
	}
	if listRes.Artifacts[0].RelativePath != "doc.md" {
		t.Fatalf("expected artifact relative_path 'doc.md', got %s", listRes.Artifacts[0].RelativePath)
	}
}
