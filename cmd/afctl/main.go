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

	var err error
	switch filtered[0] {
	case "health":
		err = runHealth(ctx, c)
	case "protocol":
		runProtocol()
	case "init":
		err = runInit(filtered[1:])
	case "project":
		err = runProject(ctx, c, filtered[1:])
	case "repo":
		err = runRepo(ctx, c, filtered[1:])
	case "worktree":
		err = runWorktree(ctx, c, filtered[1:])
	case "artifact-root":
		err = runArtifactRoot(ctx, c, filtered[1:])
	case "artifact":
		err = runArtifact(ctx, c, filtered[1:])
	case "issue":
		err = runIssue(ctx, c, filtered[1:])
	case "ls":
		err = runLs(ctx, c, filtered[1:])
	case "show":
		err = runShow(ctx, c, filtered[1:])
	default:
		printUsage()
		os.Exit(1)
	}
	if err != nil {
		fail(err)
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

func runHealth(ctx context.Context, c *client.Client) error {
	health, err := c.Health(ctx)
	if err != nil {
		return err
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(health)
		return nil
	}
	fmt.Printf("Name:       %s\n", health.Name)
	fmt.Printf("Status:     %s\n", health.Status)
	fmt.Printf("DBPath:     %s\n", health.DBPath)
	fmt.Printf("SocketPath: %s\n", health.SocketPath)
	fmt.Printf("Time:       %s\n", health.Time.UTC().Format(time.RFC3339))
	return nil
}

// runLs is a top-level shortcut for `afctl issue list`.
func runLs(ctx context.Context, c *client.Client, args []string) error {
	return runIssueList(ctx, c, args)
}

// runShow is a top-level shortcut for `afctl issue get`.
func runShow(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: afctl show <issue-id-or-short-id>")
	}
	return runIssueGet(ctx, c, args)
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
func printIssueDetailed(i core.Issue, l *core.IssueLease, events []core.Event) {
	fmt.Printf("ID:            %s\n", i.ID)
	fmt.Printf("Short ID:      %s\n", i.ShortID)
	fmt.Printf("Status:        %s %s\n", statusSymbol(i.Status), i.Status)
	fmt.Printf("Title:         %s\n", i.Title)
	if i.Description != "" {
		fmt.Printf("Description:   %s\n", i.Description)
	}
	fmt.Printf("Priority:      %d\n", i.Priority)
	if i.Assignee != "" {
		fmt.Printf("Assignee:      %s\n", i.Assignee)
	} else {
		fmt.Printf("Assignee:      (unassigned)\n")
	}
	fmt.Printf("Project ID:    %s\n", i.ProjectID)
	if i.RepositoryID != "" {
		fmt.Printf("Repository ID: %s\n", i.RepositoryID)
	}
	if i.WorktreeID != "" {
		fmt.Printf("Worktree ID:   %s\n", i.WorktreeID)
	}
	fmt.Printf("Scope:         %s\n", i.ScopeKind)
	fmt.Printf("Version:       %d\n", i.Version)
	if l != nil {
		fmt.Printf("Claimed:       %s (expires %s)\n", l.Holder, l.ExpiresAt)
	} else {
		fmt.Printf("Claimed:       (not claimed)\n")
	}
	if i.ClaimedAt != "" {
		fmt.Printf("Claimed At:    %s\n", i.ClaimedAt)
	}
	if i.ClosedAt != "" {
		fmt.Printf("Closed At:     %s\n", i.ClosedAt)
	}
	fmt.Printf("Created:       %s\n", i.CreatedAt)
	fmt.Printf("Updated:       %s\n", i.UpdatedAt)

	if len(events) > 0 {
		fmt.Printf("\nHistory:\n")
		for _, e := range events {
			fmt.Printf("  [%s] %s by %s\n", e.CreatedAt, e.EventType, e.Actor)
			if e.PayloadJSON != "" && e.PayloadJSON != "{}" {
				fmt.Printf("    %s\n", e.PayloadJSON)
			}
		}
	}
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
