package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/core"
)

var jsonOutput bool
var defaultActor string

func init() {
	defaultActor = os.Getenv("AF_COORDINATOR_ACTOR")
}

func main() {
	cfg := config.Default()

	// Parse global flags (--json, --actor) from os.Args before command dispatch.
	args := os.Args[1:]
	var filtered []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--actor":
			if i+1 < len(args) {
				defaultActor = args[i+1]
				i++
			}
		default:
			filtered = append(filtered, args[i])
		}
	}

	if len(filtered) < 1 {
		printUsage()
		os.Exit(1)
	}

	c := client.New(cfg.SocketPath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch filtered[0] {
	case "health":
		runHealth(ctx, c)
	case "protocol":
		runProtocol()
	case "init":
		runInit(filtered[1:])
	case "project":
		runProject(ctx, c, filtered[1:])
	case "repo":
		runRepo(ctx, c, filtered[1:])
	case "worktree":
		runWorktree(ctx, c, filtered[1:])
	case "artifact-root":
		runArtifactRoot(ctx, c, filtered[1:])
	case "artifact":
		runArtifact(ctx, c, filtered[1:])
	case "issue":
		runIssue(ctx, c, filtered[1:])
	case "ls":
		runLs(ctx, c, filtered[1:])
	case "show":
		runShow(ctx, c, filtered[1:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: afctl [--json] [--actor <name>] <command>

Global flags:
  --json                Output in JSON format (default: human-readable)
  --actor <name>        Set the acting identity (default: AF_COORDINATOR_ACTOR env)

Commands:
  health                Check daemon health
  protocol              Print the agent protocol contract
  init                  Initialize or update AGENTS.md with coordinator block
  project               Manage projects
    add                 Create a new project
    list                List all projects
  repo                  Manage repositories
    add                 Register a new repository
    list                List repositories
  worktree              Manage worktrees
    register            Register or update a worktree
    list                List worktrees
  artifact-root         Manage artifact roots
    add                 Register an artifact root in a repository
    list                List artifact roots
  artifact              Manage artifacts
    register            Register an artifact file
    list                List artifacts
  issue                 Manage issues
    create              Create a new issue
    get                 Get an issue by ID or short_id
    list                List issues with optional filters
    ready               List ready (actionable, unleased) issues
    claim               Claim an issue (acquire a lease)
    heartbeat           Extend an existing lease
    release             Release a claimed lease
    update              Update issue fields (title, description, priority, assignee, status)
    close               Close an issue (resolution: done or cancelled)
    link                Link an artifact to an issue
    note                Manage notes on an issue
      add              Add a note to an issue
      list             List notes on an issue
    events              Show activity timeline for an issue
      list             List events for an issue
    dependency          Manage issue dependencies
      add               Add a dependency between two issues
      remove            Remove a dependency between two issues
  ls [flags]             List issues (shortcut for issue list)
  show <issue-id>        Show issue details (shortcut for issue get)
`)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func fail(err error) {
	var clientErr *client.ClientError
	if errors.As(err, &clientErr) {
		if jsonOutput {
			resp := core.APIErrorResponse{Error: core.NewAPIError(clientErr.Code, clientErr.Message)}
			json.NewEncoder(os.Stderr).Encode(resp)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", clientErr)
		}
		os.Exit(mapExitCode(clientErr.Code))
	}
	// Транспортная/не-API ошибка
	if jsonOutput {
		json.NewEncoder(os.Stderr).Encode(core.APIErrorResponse{
			Error: core.NewAPIError("internal_error", err.Error()),
		})
	} else {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(1)
}

func mapExitCode(code string) int {
	switch code {
	case core.ErrNotFound:
		return 5
	case core.ErrLeaseHeld:
		return 3
	case core.ErrLeaseExpired:
		return 4
	case core.ErrConflict:
		return 2
	case core.ErrDependencyCycle:
		return 6
	default:
		return 1
	}
}

func mapExitCodeErr(err error) int {
	if err == nil {
		return 0
	}
	var clientErr *client.ClientError
	if errors.As(err, &clientErr) {
		return mapExitCode(clientErr.Code)
	}
	return 1
}

func resolveActor(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if defaultActor != "" {
		return defaultActor, nil
	}
	return "", fmt.Errorf("actor is required: set --actor flag or AF_COORDINATOR_ACTOR environment variable")
}

// ─── Health ──────────────────────────────────────────────────────────────────

func runHealth(ctx context.Context, c *client.Client) {
	health, err := c.Health(ctx)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(health)
		return
	}
	fmt.Printf("Name:       %s\n", health.Name)
	fmt.Printf("Status:     %s\n", health.Status)
	fmt.Printf("DBPath:     %s\n", health.DBPath)
	fmt.Printf("SocketPath: %s\n", health.SocketPath)
	fmt.Printf("Time:       %s\n", health.Time.UTC().Format(time.RFC3339))
}

// ─── Project ────────────────────────────────────────────────────────────────

func runProject(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl project <add|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runProjectAdd(ctx, c, args[1:])
	case "list":
		runProjectList(ctx, c)
	default:
		fmt.Fprintf(os.Stderr, "unknown project subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runProjectAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: afctl project add --key <key> --name <name> [--description <desc>]")
		os.Exit(1)
	}

	var key, name, description string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--key":
			if i+1 < len(args) {
				key = args[i+1]
				i++
			}
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		}
	}

	project, err := c.CreateProject(ctx, key, name, description)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(project)
		return
	}
	printProject(project)
}

func runProjectList(ctx context.Context, c *client.Client) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(projects)
		return
	}
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return
	}
	for _, p := range projects {
		printProject(p)
	}
}

func printProject(p core.Project) {
	fmt.Printf("ID:          %s\n", p.ID)
	fmt.Printf("Key:         %s\n", p.Key)
	fmt.Printf("Name:        %s\n", p.Name)
	fmt.Printf("Description: %s\n", p.Description)
	fmt.Printf("Created:     %s\n", p.CreatedAt)
	fmt.Printf("Updated:     %s\n\n", p.UpdatedAt)
}

// ─── Repo ───────────────────────────────────────────────────────────────────

func runRepo(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl repo <add|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runRepoAdd(ctx, c, args[1:])
	case "list":
		runRepoList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown repo subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runRepoAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl repo add --project <key> --logical-name <name> --canonical-git-dir <path> [--default-branch <branch>] [--remotes '<json>']")
		os.Exit(1)
	}

	var req core.CreateRepoRequest
	req.DefaultBranch = "main"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				req.Project = args[i+1]
				i++
			}
		case "--logical-name":
			if i+1 < len(args) {
				req.LogicalName = args[i+1]
				i++
			}
		case "--canonical-git-dir":
			if i+1 < len(args) {
				req.CanonicalGitDir = args[i+1]
				i++
			}
		case "--default-branch":
			if i+1 < len(args) {
				req.DefaultBranch = args[i+1]
				i++
			}
		case "--remotes":
			if i+1 < len(args) {
				if err := json.Unmarshal([]byte(args[i+1]), &req.Remotes); err != nil {
					fmt.Fprintf(os.Stderr, "error: invalid --remotes JSON: %v\n", err)
					os.Exit(1)
				}
				i++
			}
		}
	}

	repo, remotes, err := c.CreateRepo(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"repository": repo,
			"remotes":    remotes,
		})
		return
	}
	fmt.Printf("Repository ID: %s\n", repo.ID)
	fmt.Printf("Project ID:    %s\n", repo.ProjectID)
	fmt.Printf("Logical Name:  %s\n", repo.LogicalName)
	fmt.Printf("Git Dir:       %s\n", repo.CanonicalGitDir)
	fmt.Printf("Default Branch:%s\n", repo.DefaultBranch)
	if len(remotes) > 0 {
		fmt.Println("Remotes:")
		for _, r := range remotes {
			primary := ""
			if r.IsPrimary {
				primary = " (primary)"
			}
			fmt.Printf("  - %s -> %s%s\n", r.RemoteName, r.FetchURL, primary)
		}
	}
	fmt.Println()
}

func runRepoList(ctx context.Context, c *client.Client, args []string) {
	project := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			project = args[i+1]
		}
	}

	repos, err := c.ListRepos(ctx, project)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(repos)
		return
	}
	if len(repos) == 0 {
		fmt.Println("No repositories found.")
		return
	}
	for _, r := range repos {
		fmt.Printf("ID:           %s\n", r.ID)
		fmt.Printf("Project ID:   %s\n", r.ProjectID)
		fmt.Printf("Logical Name: %s\n", r.LogicalName)
		fmt.Printf("Git Dir:      %s\n", r.CanonicalGitDir)
		fmt.Printf("Branch:       %s\n", r.DefaultBranch)
		fmt.Printf("Created:      %s\n\n", r.CreatedAt)
	}
}

// ─── Worktree ───────────────────────────────────────────────────────────────

func runWorktree(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl worktree <register|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "register":
		runWorktreeRegister(ctx, c, args[1:])
	case "list":
		runWorktreeList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown worktree subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runWorktreeRegister(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl worktree register --repo <repo-id> --absolute-path <path> [--branch <branch>] [--head-commit <sha>] [--remote-name <name>] [--remote-branch <branch>] [--main] [--ephemeral]")
		os.Exit(1)
	}

	var req core.CreateWorktreeRequest
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 < len(args) {
				req.Repo = args[i+1]
				i++
			}
		case "--absolute-path":
			if i+1 < len(args) {
				req.AbsolutePath = args[i+1]
				i++
			}
		case "--branch":
			if i+1 < len(args) {
				req.Branch = args[i+1]
				i++
			}
		case "--head-commit":
			if i+1 < len(args) {
				req.HeadCommit = args[i+1]
				i++
			}
		case "--remote-name":
			if i+1 < len(args) {
				req.RemoteName = args[i+1]
				i++
			}
		case "--remote-branch":
			if i+1 < len(args) {
				req.RemoteBranch = args[i+1]
				i++
			}
		case "--main":
			req.IsMain = true
		case "--ephemeral":
			req.IsEphemeral = true
		}
	}

	wt, err := c.RegisterWorktree(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(wt)
		return
	}
	printWorktree(wt)
}

func runWorktreeList(ctx context.Context, c *client.Client, args []string) {
	repo := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
		}
	}

	worktrees, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(worktrees)
		return
	}
	if len(worktrees) == 0 {
		fmt.Println("No worktrees found.")
		return
	}
	for _, wt := range worktrees {
		printWorktree(wt)
	}
}

// ─── Artifact Root ──────────────────────────────────────────────────────────

func runArtifactRoot(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl artifact-root <add|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runArtifactRootAdd(ctx, c, args[1:])
	case "list":
		runArtifactRootList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown artifact-root subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runArtifactRootAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl artifact-root add --repo <repo-id> --root-path <path> [--kind <kind>] [--primary]")
		os.Exit(1)
	}

	var req core.CreateArtifactRootRequest
	req.Kind = "sdd"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 < len(args) {
				req.Repo = args[i+1]
				i++
			}
		case "--root-path":
			if i+1 < len(args) {
				req.RootPath = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				req.Kind = args[i+1]
				i++
			}
		case "--primary":
			req.Primary = true
		}
	}

	root, err := c.CreateArtifactRoot(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(root)
		return
	}
	printArtifactRoot(root)
}

func runArtifactRootList(ctx context.Context, c *client.Client, args []string) {
	repo := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
		}
	}

	roots, err := c.ListArtifactRoots(ctx, repo)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(roots)
		return
	}
	if len(roots) == 0 {
		fmt.Println("No artifact roots found.")
		return
	}
	for _, r := range roots {
		printArtifactRoot(r)
	}
}

func printArtifactRoot(r core.ArtifactRoot) {
	primary := ""
	if r.IsPrimary {
		primary = " (primary)"
	}
	fmt.Printf("ID:           %s%s\n", r.ID, primary)
	fmt.Printf("Repository ID:%s\n", r.RepositoryID)
	fmt.Printf("Root Path:    %s\n", r.RootPath)
	fmt.Printf("Kind:         %s\n", r.Kind)
	fmt.Printf("Created:      %s\n", r.CreatedAt)
	fmt.Printf("Updated:      %s\n\n", r.UpdatedAt)
}

// ─── Artifact ───────────────────────────────────────────────────────────────

func runArtifact(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl artifact <register|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "register":
		runArtifactRegister(ctx, c, args[1:])
	case "list":
		runArtifactList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown artifact subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runArtifactRegister(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: afctl artifact register --repo <repo-id> --relative-path <path> --kind <kind> [--worktree <worktree-id>] [--artifact-root <root-id>] [--title <title>] [--external-key <key>] [--status <status>]")
		os.Exit(1)
	}

	var req core.CreateArtifactRequest
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 < len(args) {
				req.Repo = args[i+1]
				i++
			}
		case "--relative-path":
			if i+1 < len(args) {
				req.RelativePath = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				req.Kind = args[i+1]
				i++
			}
		case "--worktree":
			if i+1 < len(args) {
				req.Worktree = args[i+1]
				i++
			}
		case "--artifact-root":
			if i+1 < len(args) {
				req.ArtifactRootID = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				req.Title = args[i+1]
				i++
			}
		case "--external-key":
			if i+1 < len(args) {
				req.ExternalKey = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				req.Status = args[i+1]
				i++
			}
		}
	}

	artifact, err := c.CreateArtifact(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(artifact)
		return
	}
	printArtifact(artifact)
}

func runArtifactList(ctx context.Context, c *client.Client, args []string) {
	repo := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
		}
	}

	artifacts, err := c.ListArtifacts(ctx, repo)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(artifacts)
		return
	}
	if len(artifacts) == 0 {
		fmt.Println("No artifacts found.")
		return
	}
	for _, a := range artifacts {
		printArtifact(a)
	}
}

func printArtifact(a core.Artifact) {
	fmt.Printf("ID:             %s\n", a.ID)
	fmt.Printf("Repository ID:  %s\n", a.RepositoryID)
	if a.WorktreeID != "" {
		fmt.Printf("Worktree ID:    %s\n", a.WorktreeID)
	}
	if a.ArtifactRootID != "" {
		fmt.Printf("Artifact Root:  %s\n", a.ArtifactRootID)
	}
	fmt.Printf("Kind:           %s\n", a.Kind)
	fmt.Printf("Relative Path:  %s\n", a.RelativePath)
	if a.Title != "" {
		fmt.Printf("Title:          %s\n", a.Title)
	}
	if a.ExternalKey != "" {
		fmt.Printf("External Key:   %s\n", a.ExternalKey)
	}
	if a.Status != "" {
		fmt.Printf("Status:         %s\n", a.Status)
	}
	fmt.Printf("Created:        %s\n", a.CreatedAt)
	fmt.Printf("Updated:        %s\n\n", a.UpdatedAt)
}

// ─── Issue ───────────────────────────────────────────────────────────────────

func runIssue(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue <create|get|list|ready|claim|heartbeat|release>")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		runIssueCreate(ctx, c, args[1:])
	case "get":
		runIssueGet(ctx, c, args[1:])
	case "list":
		runIssueList(ctx, c, args[1:])
	case "ready":
		runIssueReady(ctx, c, args[1:])
	case "claim":
		runIssueClaim(ctx, c, args[1:])
	case "heartbeat":
		runIssueHeartbeat(ctx, c, args[1:])
	case "release":
		runIssueRelease(ctx, c, args[1:])
	case "update":
		runIssueUpdate(ctx, c, args[1:])
	case "close":
		runIssueClose(ctx, c, args[1:])
	case "link":
		runIssueLink(ctx, c, args[1:])
	case "dependency":
		runIssueDependency(ctx, c, args[1:])
	case "note":
		runIssueNote(ctx, c, args[1:])
	case "events":
		runIssueEvents(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown issue subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

// runLs is a top-level shortcut for `afctl issue list`.
func runLs(ctx context.Context, c *client.Client, args []string) {
	runIssueList(ctx, c, args)
}

// runShow is a top-level shortcut for `afctl issue get`.
func runShow(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl show <issue-id-or-short-id>")
		os.Exit(1)
	}
	runIssueGet(ctx, c, args)
}

func runIssueCreate(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue create --project <key> --scope-kind <project|repository|worktree> --title <title> [--repo <repo>] [--worktree <worktree>] [--description <desc>] [--priority <n>]")
		os.Exit(1)
	}

	var req core.CreateIssueRequest
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				req.Project = args[i+1]
				i++
			}
		case "--scope-kind":
			if i+1 < len(args) {
				req.ScopeKind = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				req.Title = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				req.Description = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.Priority)
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				req.Repo = args[i+1]
				i++
			}
		case "--worktree":
			if i+1 < len(args) {
				req.Worktree = args[i+1]
				i++
			}
		}
	}

	actor, err := resolveActor("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Actor = actor

	issue, err := c.CreateIssue(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return
	}
	printIssue(issue)
}

func runIssueGet(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue get <issue-id-or-short-id>")
		os.Exit(1)
	}

	issueID := args[0]
	issue, lease, err := c.GetIssue(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		resp := map[string]any{
			"issue": issue,
		}
		if lease != nil {
			resp["lease"] = lease
		}
		json.NewEncoder(os.Stdout).Encode(resp)
		return
	}
	printIssueDetailed(issue, lease)
}

func runIssueList(ctx context.Context, c *client.Client, args []string) {
	project := ""
	repo := ""
	worktree := ""
	status := ""
	assignee := ""
	limit := 0
	offset := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				repo = args[i+1]
				i++
			}
		case "--worktree":
			if i+1 < len(args) {
				worktree = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				status = args[i+1]
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				assignee = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		case "--offset":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &offset)
				i++
			}
		}
	}

	// limit/offset are defined for future use; pass them through when the API supports pagination
	_ = limit
	_ = offset

	issues, err := c.ListIssues(ctx, project, repo, worktree, status, assignee)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issues)
		return
	}
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}
	printIssuesTable(issues)
}

func runIssueReady(ctx context.Context, c *client.Client, args []string) {
	project := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			project = args[i+1]
		}
	}

	issues, err := c.ListReadyIssues(ctx, project)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issues)
		return
	}
	if len(issues) == 0 {
		fmt.Println("No ready issues found.")
		return
	}
	printIssuesTable(issues)
}

func runIssueClaim(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue claim <issue-id> [--holder <name>|--actor <name>] [--ttl <seconds>]")
		os.Exit(1)
	}

	issueID := args[0]
	holder := ""
	ttl := 3600

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--holder", "--actor":
			if i+1 < len(args) {
				holder = args[i+1]
				i++
			}
		case "--ttl":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &ttl)
				i++
			}
		}
	}

	var err error
	holder, err = resolveActor(holder)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	resp, err := c.ClaimIssue(ctx, issueID, holder, ttl)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(resp)
		return
	}
	fmt.Printf("Lease Token: %s\n", resp.LeaseToken)
	fmt.Printf("Expires At:  %s\n", resp.ExpiresAt)
}

func runIssueHeartbeat(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue heartbeat <issue-id> --lease-token <token> [--ttl <seconds>]")
		os.Exit(1)
	}

	issueID := args[0]
	leaseToken := ""
	ttl := 3600

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--lease-token":
			if i+1 < len(args) {
				leaseToken = args[i+1]
				i++
			}
		case "--ttl":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &ttl)
				i++
			}
		}
	}

	if leaseToken == "" {
		fmt.Fprintln(os.Stderr, "error: --lease-token is required")
		os.Exit(1)
	}

	expiresAt, err := c.HeartbeatLease(ctx, issueID, leaseToken, ttl)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"expires_at": expiresAt})
		return
	}
	fmt.Printf("Expires At: %s\n", expiresAt)
}

func runIssueRelease(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue release <issue-id> --lease-token <token>")
		os.Exit(1)
	}

	issueID := args[0]
	leaseToken := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--lease-token":
			if i+1 < len(args) {
				leaseToken = args[i+1]
				i++
			}
		}
	}

	if leaseToken == "" {
		fmt.Fprintln(os.Stderr, "error: --lease-token is required")
		os.Exit(1)
	}

	if err := c.ReleaseLease(ctx, issueID, leaseToken); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Lease released.")
}

func runIssueUpdate(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue update <issue-id> [--title ...] [--description ...] [--priority N] [--assignee ...] [--status ...] --expected-version N [--lease-token ...]")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.UpdateIssueRequest
	req.ExpectedVersion = -1

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 < len(args) {
				req.Title = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				req.Description = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.Priority)
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				req.Assignee = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				req.Status = args[i+1]
				i++
			}
		case "--expected-version":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.ExpectedVersion)
				i++
			}
		case "--lease-token":
			if i+1 < len(args) {
				req.LeaseToken = args[i+1]
				i++
			}
		}
	}

	if req.ExpectedVersion < 0 {
		fmt.Fprintln(os.Stderr, "error: --expected-version is required")
		os.Exit(1)
	}

	actor, err := resolveActor("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Actor = actor

	issue, err := c.UpdateIssue(ctx, issueID, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return
	}
	printIssue(issue)
}

func runIssueClose(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue close <issue-id> --resolution done|cancelled --expected-version N --lease-token ...")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.CloseIssueRequest
	req.ExpectedVersion = -1

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--resolution":
			if i+1 < len(args) {
				req.Resolution = args[i+1]
				i++
			}
		case "--expected-version":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.ExpectedVersion)
				i++
			}
		case "--lease-token":
			if i+1 < len(args) {
				req.LeaseToken = args[i+1]
				i++
			}
		}
	}

	if req.Resolution == "" {
		fmt.Fprintln(os.Stderr, "error: --resolution is required (done or cancelled)")
		os.Exit(1)
	}
	if req.ExpectedVersion < 0 {
		fmt.Fprintln(os.Stderr, "error: --expected-version is required")
		os.Exit(1)
	}
	if req.LeaseToken == "" {
		fmt.Fprintln(os.Stderr, "error: --lease-token is required")
		os.Exit(1)
	}

	actor, err := resolveActor("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Actor = actor

	if err := c.CloseIssue(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Issue closed.")
}

func runIssueLink(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue link <issue-id> --artifact <artifact-id> [--relation implements|...]")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.LinkArtifactRequest

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 < len(args) {
				req.Artifact = args[i+1]
				i++
			}
		case "--relation":
			if i+1 < len(args) {
				req.Relation = args[i+1]
				i++
			}
		}
	}

	if req.Artifact == "" {
		fmt.Fprintln(os.Stderr, "error: --artifact is required")
		os.Exit(1)
	}

	if err := c.LinkArtifact(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Artifact linked.")
}

func runIssueDependency(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue dependency <add|remove> <issue-id> --depends-on <other-issue> [--kind blocks|parent|related|discovered-from]")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runIssueDependencyAdd(ctx, c, args[1:])
	case "remove":
		runIssueDependencyRemove(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown dependency subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueDependencyAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue dependency add <issue-id> --depends-on <other-issue> [--kind blocks|parent|related|discovered-from]")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.AddDependencyRequest

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--depends-on":
			if i+1 < len(args) {
				req.DependsOn = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				req.Kind = args[i+1]
				i++
			}
		}
	}

	if req.DependsOn == "" {
		fmt.Fprintln(os.Stderr, "error: --depends-on is required")
		os.Exit(1)
	}

	if err := c.AddDependency(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Dependency added.")
}

func runIssueDependencyRemove(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue dependency remove <issue-id> --depends-on <other-issue> [--kind blocks]")
		os.Exit(1)
	}

	issueID := args[0]
	dependsOn := ""
	kind := "blocks"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--depends-on":
			if i+1 < len(args) {
				dependsOn = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				kind = args[i+1]
				i++
			}
		}
	}

	if dependsOn == "" {
		fmt.Fprintln(os.Stderr, "error: --depends-on is required")
		os.Exit(1)
	}

	if err := c.RemoveDependency(ctx, issueID, dependsOn, kind); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Dependency removed.")
}

// ─── Issue Notes ────────────────────────────────────────────────────────────

func runIssueNote(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue note <add|list> <issue-id> [--author <name> --body <text>]")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runIssueNoteAdd(ctx, c, args[1:])
	case "list":
		runIssueNoteList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown note subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueNoteAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue note add <issue-id> [--author <name>|--actor <name>] --body <text>")
		os.Exit(1)
	}

	issueID := args[0]
	author := ""
	body := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--author", "--actor":
			if i+1 < len(args) {
				author = args[i+1]
				i++
			}
		case "--body":
			if i+1 < len(args) {
				body = args[i+1]
				i++
			}
		}
	}

	var err error
	author, err = resolveActor(author)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if body == "" {
		fmt.Fprintln(os.Stderr, "error: --body is required")
		os.Exit(1)
	}

	note, err := c.CreateNote(ctx, issueID, author, body)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(note)
		return
	}
	fmt.Printf("Note ID:    %s\n", note.ID)
	fmt.Printf("Issue ID:   %s\n", note.IssueID)
	fmt.Printf("Author:     %s\n", note.Author)
	fmt.Printf("Body:       %s\n", note.Body)
	fmt.Printf("Created At: %s\n", note.CreatedAt)
}

func runIssueNoteList(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue note list <issue-id>")
		os.Exit(1)
	}

	issueID := args[0]

	notes, err := c.ListNotes(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(notes)
		return
	}
	if len(notes) == 0 {
		fmt.Println("No notes found.")
		return
	}
	for _, n := range notes {
		fmt.Printf("Note ID:    %s\n", n.ID)
		fmt.Printf("Author:     %s\n", n.Author)
		fmt.Printf("Body:       %s\n", n.Body)
		fmt.Printf("Created At: %s\n\n", n.CreatedAt)
	}
}

// ─── Issue Events ──────────────────────────────────────────────────────────

func runIssueEvents(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue events list <issue-id>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		runIssueEventsList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown events subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueEventsList(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue events list <issue-id>")
		os.Exit(1)
	}

	issueID := args[0]

	events, err := c.ListEvents(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(events)
		return
	}
	if len(events) == 0 {
		fmt.Println("No events found.")
		return
	}
	for _, e := range events {
		fmt.Printf("Event ID:    %s\n", e.ID)
		fmt.Printf("Actor:       %s\n", e.Actor)
		fmt.Printf("Type:        %s\n", e.EventType)
		fmt.Printf("Payload:     %s\n", e.PayloadJSON)
		fmt.Printf("Created At:  %s\n\n", e.CreatedAt)
	}
}

func printIssue(i core.Issue) {
	fmt.Printf("ID:           %s\n", i.ID)
	fmt.Printf("Short ID:     %s\n", i.ShortID)
	fmt.Printf("Project ID:   %s\n", i.ProjectID)
	if i.RepositoryID != "" {
		fmt.Printf("Repository ID:%s\n", i.RepositoryID)
	}
	if i.WorktreeID != "" {
		fmt.Printf("Worktree ID:  %s\n", i.WorktreeID)
	}
	fmt.Printf("Scope:        %s\n", i.ScopeKind)
	fmt.Printf("Title:        %s\n", i.Title)
	if i.Description != "" {
		fmt.Printf("Description:  %s\n", i.Description)
	}
	fmt.Printf("Status:       %s\n", i.Status)
	fmt.Printf("Priority:     %d\n", i.Priority)
	if i.Assignee != "" {
		fmt.Printf("Assignee:     %s\n", i.Assignee)
	}
	if i.Holder != "" {
		fmt.Printf("Holder:       %s\n", i.Holder)
	}
	fmt.Printf("Version:      %d\n", i.Version)
	if i.ClaimedAt != "" {
		fmt.Printf("Claimed At:   %s\n", i.ClaimedAt)
	}
	if i.ClosedAt != "" {
		fmt.Printf("Closed At:    %s\n", i.ClosedAt)
	}
	fmt.Printf("Created:      %s\n", i.CreatedAt)
	fmt.Printf("Updated:      %s\n\n", i.UpdatedAt)
}

// ─── Issue Display Helpers ───────────────────────────────────────────────────

// statusSymbol returns a Unicode symbol representing the issue status.
func statusSymbol(status string) string {
	switch status {
	case "open":
		return "○"
	case "in_progress":
		return "●"
	case "done":
		return "✓"
	case "cancelled":
		return "✗"
	case "blocked":
		return "⊘"
	case "deferred":
		return "◌"
	default:
		return "?"
	}
}

// truncate shortens a string to n characters, adding "..." if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-3]) + "..."
}

// printIssueDetailed displays a single issue with full details.
func printIssueDetailed(i core.Issue, l *core.IssueLease) {
	fmt.Printf("ID:         %s\n", i.ID)
	fmt.Printf("Short ID:   %s\n", i.ShortID)
	fmt.Printf("Status:     %s %s\n", statusSymbol(i.Status), i.Status)
	fmt.Printf("Title:      %s\n", i.Title)
	fmt.Printf("Priority:   %d\n", i.Priority)
	if i.Assignee != "" {
		fmt.Printf("Assignee:   %s\n", i.Assignee)
	} else {
		fmt.Printf("Assignee:   (unassigned)\n")
	}
	fmt.Printf("Scope:      %s\n", i.ScopeKind)
	fmt.Printf("Version:    %d\n", i.Version)
	if l != nil {
		fmt.Printf("Claimed:    %s (expires %s)\n", l.Holder, l.ExpiresAt)
	} else {
		fmt.Printf("Claimed:    (not claimed)\n")
	}
	fmt.Printf("Created:    %s\n", i.CreatedAt)
	fmt.Printf("Updated:    %s\n", i.UpdatedAt)
}

// printIssuesTable displays a list of issues in a fixed-width table format.
func printIssuesTable(issues []core.Issue) {
	fmt.Printf("%-10s %-12s %-12s %-50s %-12s %-12s\n", "ID", "SHORT", "STATUS", "TITLE", "ASSIGNEE", "CLAIMED")
	fmt.Printf("%-10s %-12s %-12s %-50s %-12s %-12s\n", "---", "-----", "------", "-----", "-------", "-------")
	for _, i := range issues {
		id := truncate(i.ID, 11) // 8 chars + "..."
		title := truncate(i.Title, 50)
		status := statusSymbol(i.Status) + " " + i.Status
		assignee := truncate(i.Assignee, 12)
		claimed := truncate(i.Holder, 12)
		fmt.Printf("%-10s %-12s %-12s %-50s %-12s %-12s\n", id, i.ShortID, status, title, assignee, claimed)
	}
}

func printWorktree(wt core.Worktree) {
	mainStr := ""
	if wt.IsMain {
		mainStr = " (main)"
	}
	ephemeralStr := ""
	if wt.IsEphemeral {
		ephemeralStr = " [ephemeral]"
	}
	fmt.Printf("ID:           %s%s%s\n", wt.ID, mainStr, ephemeralStr)
	fmt.Printf("Repository ID:%s\n", wt.RepositoryID)
	fmt.Printf("Path:         %s\n", wt.AbsolutePath)
	fmt.Printf("Branch:       %s\n", wt.Branch)
	if wt.HeadCommit != "" {
		fmt.Printf("Head:         %s\n", wt.HeadCommit)
	}
	if wt.RemoteName != "" {
		fmt.Printf("Remote:       %s/%s\n", wt.RemoteName, wt.RemoteBranch)
	}
	fmt.Printf("Last Seen:    %s\n", wt.LastSeenAt)
	fmt.Printf("Created:      %s\n", wt.CreatedAt)
	fmt.Printf("Updated:      %s\n\n", wt.UpdatedAt)
}
